package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"errors"

	"github.com/google/uuid"
	"github.com/logicmonitor/lm-data-sdk-go/internal/client"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"go.uber.org/multierr"
)

const (
	logIngestURI             = "/log/ingest"
	lmLogsMessageKey         = "msg"
	commonMessageKey         = "message"
	resourceIDKey            = "_lm.resourceId"
	timestampKey             = "timestamp"
	defaultBatchingInterval  = 10 * time.Second
	maxHTTPResponseReadBytes = 64 * 1024
	headerRetryAfter         = "Retry-After"
)

type LMLogIngest struct {
	client             *http.Client
	url                string
	auth               utils.AuthParams
	gzip               bool
	rateLimiterSetting rateLimiter.LogRateLimiterSetting
	rateLimiter        rateLimiter.RateLimiter
	batch              *logsBatch
}

type LMLogIngestRequest struct {
	Payload []model.LogPayload
}

type LMLogIngestResponse struct {
	Success   bool                     `json:"success"`
	Message   string                   `json:"message"`
	Errors    []map[string]interface{} `json:"errors"`
	RequestID uuid.UUID                `json:"requestId"`
}

type logsBatch struct {
	enabled  bool
	data     []*LMLogIngestRequest
	interval time.Duration
	lock     *sync.Mutex
}

func NewLogBatch() *logsBatch {
	return &logsBatch{enabled: true, interval: defaultBatchingInterval, lock: &sync.Mutex{}}
}

// NewLMLogIngest initializes LMLogIngest
func NewLMLogIngest(ctx context.Context, opts ...Option) (*LMLogIngest, error) {
	logIngest := LMLogIngest{
		gzip:               true,
		client:             client.Client(),
		auth:               utils.AuthParams{},
		rateLimiterSetting: rateLimiter.LogRateLimiterSetting{},
		batch:              NewLogBatch(),
	}

	for _, opt := range opts {
		if err := opt(&logIngest); err != nil {
			return nil, err
		}
	}

	var err error
	if logIngest.url == "" {
		logsURL, err := utils.URL()
		if err != nil {
			return nil, fmt.Errorf("error in forming Logs URL: %v", err)
		}
		logIngest.url = logsURL
	}

	logIngest.rateLimiter, err = rateLimiter.NewLogRateLimiter(logIngest.rateLimiterSetting)
	if err != nil {
		return nil, err
	}

	go logIngest.rateLimiter.Run(ctx)

	if logIngest.batch.enabled {
		go logIngest.processBatch(ctx)
	}
	return &logIngest, nil
}

func (logIngest *LMLogIngest) processBatch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.NewTicker(logIngest.batch.batchInterval()).C:
			req := logIngest.batch.combineBatchedLogRequests()
			if req == nil {
				continue
			}
			_, err := logIngest.export(req, logIngest.uri(), http.MethodPost)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

// SendLogs is the entry point for receiving log data
func (logIngest *LMLogIngest) SendLogs(ctx context.Context, body []model.LogInput, o ...SendLogsOptionalParameters) (*model.IngestResponse, error) {
	req, err := buildLogRequest(ctx, body, o...)
	if err != nil {
		return nil, err
	}
	if logIngest.batch.enabled {
		logIngest.batch.pushToBatch(req)
		return nil, nil
	}
	return logIngest.export(req, logIngestURI, http.MethodPost)
}

// buildLogRequest creates LMLogIngestRequest
func buildLogRequest(ctx context.Context, body []model.LogInput, o ...SendLogsOptionalParameters) (*LMLogIngestRequest, error) {
	payload := buildLogPayload(body)
	return &LMLogIngestRequest{Payload: payload}, nil
}

// buildLogPayload creates log payload from the received LogInput
func buildLogPayload(logItems []model.LogInput) []model.LogPayload {
	var payload []model.LogPayload

	for _, logItem := range logItems {
		body := make(map[string]interface{}, 0)
		var messageFound bool

		switch value := logItem.Message.(type) {
		case string:
			parseStringMessage(value, body)
			messageFound = true
			for k, v := range logItem.Metadata {
				body[k] = v
			}

		case map[string]interface{}:
			parseMapMessage(value, body)
			messageFound = true
			for k, v := range logItem.Metadata {
				body[k] = v
			}
		}

		// if message not found in body, check for metadata attributes
		if !messageFound {
			messageFound = parseMessageFromMetadata(logItem.Metadata, body)
		}

		// set message field with empty value if not set yet
		if !messageFound {
			body[lmLogsMessageKey] = nil
		}

		body[resourceIDKey] = logItem.ResourceID
		body[timestampKey] = logItem.Timestamp

		payload = append(payload, body)
	}
	return payload
}

// parseStringMessage extracts the message value from the string body
func parseStringMessage(value string, body map[string]interface{}) {
	body[lmLogsMessageKey] = value
}

// parseMapMessage extracts the message value from the map type log body
func parseMapMessage(properties map[string]interface{}, body map[string]interface{}) {
	// for eg.
	// windows event logs Raw format: Body contains map of attributes
	// containing log message
	// Body: Map({"ActivityID":"","Channel":"Setup","Computer":"OtelDemoDevice","EventID":1,"EventRecordID":7,"Keywords":"0x8000000000000000","Level":0,"Message":"Initiating changes for package KB5020874. Current state is Absent. Target state is Installed. Client id: WindowsUpdateAgent.","Opcode":0,"ProcessID":1848,"ProviderGuid":"{BD12F3B8-FC40-4A61-A307-B7A013A069C1}","ProviderName":"Microsoft-Windows-Servicing","Qualifiers":"","RelatedActivityID":"","StringInserts":["KB5020874",5000,"Absent",5112,"Installed","WindowsUpdateAgent"],"Task":1,"ThreadID":5496,"TimeCreated":"2023-02-10 05:41:19 +0000","UserID":"S-1-5-18","Version":0})

	for name, value := range properties {
		if strings.EqualFold(name, commonMessageKey) {
			body[lmLogsMessageKey] = value
		} else {
			body[name] = value
		}
	}
}

// parseMessageFromMetadata extracts the message value from the metadata attributes
func parseMessageFromMetadata(metadata map[string]interface{}, body map[string]interface{}) bool {
	var messageKeyFound bool

	for key, value := range metadata {
		// add property to metadata, we will remove it from metadata if it is message key
		body[key] = value
		if !messageKeyFound {
			// check if metadata property matches with any of metadataMessageKeys
			for _, metadataMessageKey := range getMessageMetadataKeys() {
				if strings.EqualFold(key, metadataMessageKey) {
					properties, ok := value.(map[string]interface{})
					if ok {
						for k, v := range properties {
							if strings.EqualFold(k, commonMessageKey) {
								body[lmLogsMessageKey] = v
								// remove message property from metadata, because it is already part of log message
								delete(properties, k)
								body[key] = properties
								messageKeyFound = true
								break
							}
						}
					}
				}
				if messageKeyFound {
					break
				}
			}
		}
	}
	return messageKeyFound
}

// pushToBatch adds incoming log requests to batch
func (batch *logsBatch) pushToBatch(req *LMLogIngestRequest) {
	batch.lock.Lock()
	defer batch.lock.Unlock()
	batch.data = append(batch.data, req)
}

// batchInterval returns the time interval for batching
func (batch *logsBatch) batchInterval() time.Duration {
	return batch.interval
}

// uri returns the endpoint/uri of log ingest API
func (logIngest *LMLogIngest) uri() string {
	return logIngestURI
}

// combineBatchedLogRequests prepares log payload from the requests present in batch after batch interval expires
func (batch *logsBatch) combineBatchedLogRequests() *LMLogIngestRequest {
	var combinedPayload []model.LogPayload

	batch.lock.Lock()
	defer batch.lock.Unlock()

	if len(batch.data) == 0 {
		return nil
	}
	for _, req := range batch.data {
		combinedPayload = append(combinedPayload, req.Payload...)
	}
	// flushing out log batch
	if batch.enabled {
		batch.data = nil
	}
	return &LMLogIngestRequest{Payload: combinedPayload}
}

// export exports logs to the LM platform
func (logIngest *LMLogIngest) export(req *LMLogIngestRequest, uri, method string) (*model.IngestResponse, error) {
	if len(req.Payload) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("error in marshaling log payload: %v", err)
	}
	token := logIngest.auth.GetCredentials(method, uri, body)

	cfg := client.RequestConfig{
		Client:      logIngest.client,
		RateLimiter: logIngest.rateLimiter,
		Url:         logIngest.url,
		Body:        body,
		Uri:         logIngest.uri(),
		Method:      method,
		Token:       token,
		Gzip:        logIngest.gzip,
	}

	resp, err := client.DoRequest(context.Background(), cfg, handleLogsExportResponse)
	if err != nil {
		return resp, fmt.Errorf("error while exporting logs: %w", err)
	}
	return resp, nil
}

// getMessageMetadataKeys returns the metadata keys in which message property can be found
func getMessageMetadataKeys() []string {
	return []string{"azure.properties"}
}

// handleLogsExportResponse handles the http response returned by LM platform
func handleLogsExportResponse(ctx context.Context, resp *http.Response) (*model.IngestResponse, error) {
	defer func() {
		// Discard any remaining response body when we are done reading.
		io.CopyN(io.Discard, resp.Body, maxHTTPResponseReadBytes) // nolint:errcheck
		resp.Body.Close()
	}()

	requestId, _ := uuid.Parse(resp.Header.Get("x-request-id"))

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		// Request is successful.
		return &model.IngestResponse{
			StatusCode: resp.StatusCode,
			Success:    true,
			RequestID:  requestId,
		}, nil
	}

	parsedResponse := readResponse(resp)

	apiCallResponse := &model.IngestResponse{StatusCode: resp.StatusCode}

	// Format the error message.
	var formattedErr error
	if parsedResponse != nil {
		var err error
		// error codes: https://www.logicmonitor.com/support/lm-logs/sending-logs-to-the-lm-logs-ingestion-api
		if resp.StatusCode == http.StatusMultiStatus {
			errs := []error{}
			apiCallResponse.Message = parsedResponse.Message

			for _, responseError := range parsedResponse.Errors {
				if responseError["error"] != nil {
					apiCallResponse.MultiStatus = append(apiCallResponse.MultiStatus, struct {
						Code  int    `json:"code"`
						Error string `json:"error"`
					}{
						Code:  responseError["code"].(int),
						Error: responseError["error"].(string),
					})
					errs = append(errs, fmt.Errorf("error code: [%d], error message: %s", responseError["code"].(int), responseError["error"].(string)))
				}
			}
			err = multierr.Combine(errs...)
		} else {
			err = errors.New(parsedResponse.Message)
		}
		formattedErr = fmt.Errorf(
			"error exporting items, request to %s responded with HTTP Status Code %d, Message: %s, Details=%s",
			resp.Request.URL, resp.StatusCode, parsedResponse.Message, err.Error())
	} else {
		formattedErr = fmt.Errorf(
			"error exporting items, request to %s responded with HTTP Status Code %d",
			resp.Request.URL, resp.StatusCode)
	}
	apiCallResponse.Error = formattedErr

	retryAfter := 0
	if val := resp.Header.Get(headerRetryAfter); val != "" {
		if seconds, err2 := strconv.Atoi(val); err2 == nil {
			retryAfter = seconds
		}
	}
	apiCallResponse.RetryAfter = retryAfter
	return apiCallResponse, nil
}

// Read the response and decode
// Returns nil if the response is empty or cannot be decoded.
func readResponse(resp *http.Response) *LMLogIngestResponse {
	var lmLogIngestResponse *LMLogIngestResponse
	// error codes: https://www.logicmonitor.com/support/lm-logs/sending-logs-to-the-lm-logs-ingestion-api
	if resp.StatusCode == 207 || (resp.StatusCode >= 400 && resp.StatusCode <= 599) {
		// Request failed. Read the body.
		maxRead := resp.ContentLength
		if maxRead == -1 || maxRead > maxHTTPResponseReadBytes {
			maxRead = maxHTTPResponseReadBytes
		}
		respBytes := make([]byte, maxRead)
		n, err := io.ReadFull(resp.Body, respBytes)
		if err == nil && n > 0 {
			lmLogIngestResponse = &LMLogIngestResponse{}
			err = json.Unmarshal(respBytes, lmLogIngestResponse)
			if err != nil {
				lmLogIngestResponse = nil
			}
		}
	}
	return lmLogIngestResponse
}
