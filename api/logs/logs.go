package logs

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/internal/client"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	batch "github.com/logicmonitor/lm-data-sdk-go/pkg/batch"
	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

const (
	logIngestURI            = "/log/ingest"
	lmLogsMessageKey        = "msg"
	commonMessageKey        = "message"
	resourceIDKey           = "_lm.resourceId"
	timestampKey            = "timestamp"
	defaultBatchingInterval = 10 * time.Second
)

var (
	logBatch      []model.LogInput
	logBatchMutex sync.Mutex
)

type LMLogIngest struct {
	client             *http.Client
	url                string
	batch              bool
	interval           time.Duration
	auth               utils.AuthParams
	gzip               bool
	rateLimiterSetting rateLimiter.RateLimiterSetting
	rateLimiter        rateLimiter.RateLimiter
}

// NewLMLogIngest initializes LMLogIngest
func NewLMLogIngest(ctx context.Context, opts ...Option) (*LMLogIngest, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false, MinVersion: tls.VersionTLS12}
	clientTransport := (http.RoundTripper)(transport)
	client := http.Client{Transport: clientTransport, Timeout: 5 * time.Second}

	lli := LMLogIngest{
		client:             &client,
		batch:              true,
		interval:           defaultBatchingInterval,
		auth:               utils.AuthParams{},
		gzip:               true,
		rateLimiterSetting: rateLimiter.RateLimiterSetting{},
	}

	for _, opt := range opts {
		if err := opt(&lli); err != nil {
			return nil, err
		}
	}

	var err error
	if lli.url == "" {
		logsURL, err := utils.URL()
		if err != nil {
			return nil, fmt.Errorf("error in forming Logs URL: %v", err)
		}
		lli.url = logsURL
	}

	lli.rateLimiter, err = rateLimiter.NewLogRateLimiter(lli.rateLimiterSetting)
	if err != nil {
		return nil, err
	}
	go lli.rateLimiter.Run(ctx)
	if lli.batch {
		go batch.CreateAndExportData(&lli)
	}
	return &lli, nil
}

// SendLogs is the entry point for receiving log data
func (lli *LMLogIngest) SendLogs(ctx context.Context, logInput model.LogInput) error {
	if lli.batch {
		pushToBatch(logInput)
		return nil
	}

	payload := buildPayload(logInput)

	bodyarr := append([]model.LogPayload{}, payload)
	logPayloadList := model.DataPayload{
		LogBodyList: bodyarr,
	}
	return lli.ExportData(logPayloadList, logIngestURI, http.MethodPost)
}

// buildPayload creates log payload from the received LogInput
func buildPayload(logInput model.LogInput) model.LogPayload {
	body := make(map[string]interface{}, 0)

	var messageFound bool

	switch value := logInput.Message.(type) {
	case string:
		parseStringMessage(value, body)
		messageFound = true
		for k, v := range logInput.Metadata {
			body[k] = v
		}

	case map[string]interface{}:
		parseMapMessage(value, body)
		messageFound = true
		for k, v := range logInput.Metadata {
			body[k] = v
		}
	}

	// if message not found in body, check for metadata attributes
	if !messageFound {
		messageFound = parseMessageFromMetadata(logInput.Metadata, body)
	}

	// set message field with empty value if not set yet
	if !messageFound {
		body[lmLogsMessageKey] = nil
	}

	body[resourceIDKey] = logInput.ResourceID
	body[timestampKey] = logInput.Timestamp

	return body
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

// pushToBatch adds incoming log requests to logBatch internal cache
func pushToBatch(logInput model.LogInput) {
	logBatchMutex.Lock()
	defer logBatchMutex.Unlock()
	logBatch = append(logBatch, logInput)
}

// BatchInterval returns the time interval for batching
func (lli *LMLogIngest) BatchInterval() time.Duration {
	return lli.interval
}

// URI returns the endpoint/uri of log ingest API
func (lli *LMLogIngest) URI() string {
	return logIngestURI
}

// CreateRequestBody prepares log payload from the requests present in cache after batch interval expires
func (lli *LMLogIngest) CreateRequestBody() model.DataPayload {
	var logPayloadList []model.LogPayload

	logBatchMutex.Lock()
	defer logBatchMutex.Unlock()

	if len(logBatch) == 0 {
		return model.DataPayload{}
	}
	for _, logInput := range logBatch {
		payload := buildPayload(logInput)
		logPayloadList = append(logPayloadList, payload)
	}
	dataPayload := model.DataPayload{
		LogBodyList: logPayloadList,
	}
	// flushing out log batch
	if lli.batch {
		logBatch = nil
	}
	return dataPayload
}

// ExportData exports logs to the LM platform
func (lli *LMLogIngest) ExportData(payloadList model.DataPayload, uri, method string) error {
	if len(payloadList.LogBodyList) > 0 {
		body, err := json.Marshal(payloadList.LogBodyList)
		if err != nil {
			return fmt.Errorf("error in marshaling log payload: %v", err)
		}
		token := lli.auth.GetCredentials(method, uri, body)

		cfg := client.RequestConfig{
			Client:      lli.client,
			RateLimiter: lli.rateLimiter,
			Url:         lli.url,
			Body:        body,
			Uri:         lli.URI(),
			Method:      method,
			Token:       token,
			Gzip:        lli.gzip,
		}

		_, err = client.MakeRequest(context.Background(), cfg)
		if err != nil {
			return fmt.Errorf("error while exporting logs : %v", err)
		}
		return err
	}
	return nil
}

// getMessageMetadataKeys returns the metadata keys in which message property can be found
func getMessageMetadataKeys() []string {
	return []string{"azure.properties"}
}
