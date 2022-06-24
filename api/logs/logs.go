package logs

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/internal"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

const (
	uri = "/log/ingest"
)

var (
	logBatch      []model.LogInput
	logBatchMutex sync.Mutex
)

type LMLogIngest struct {
	client   *http.Client
	url      string
	batch    bool
	interval time.Duration
	auth     model.AuthProvider
	gzip     bool
}

// NewLMLogIngest initializes LMLogIngest
func NewLMLogIngest(ctx context.Context, opts ...Option) (*LMLogIngest, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false, MinVersion: tls.VersionTLS12}
	clientTransport := (http.RoundTripper)(transport)
	client := http.Client{Transport: clientTransport, Timeout: 5 * time.Second}

	logsURL, err := utils.URL()
	if err != nil {
		return nil, fmt.Errorf("Error in forming Logs URL: %v", err)
	}
	lli := LMLogIngest{
		client: &client,
		url:    logsURL,
		auth:   model.DefaultAuthenticator{},
		gzip:   true,
	}
	for _, opt := range opts {
		if err := opt(&lli); err != nil {
			return nil, err
		}
	}
	if lli.batch {
		go internal.CreateAndExportData(&lli)
	}
	return &lli, nil
}

// SendLogs is the entry point for receiving log data
func (lli *LMLogIngest) SendLogs(ctx context.Context, logMessage string, resourceidMap, metadata map[string]string) error {
	timestamp := strconv.Itoa(int(time.Now().Unix()))
	logsV1 := model.LogInput{
		Message:    logMessage,
		ResourceID: resourceidMap,
		Metadata:   metadata,
		Timestamp:  timestamp}

	if lli.batch {
		addRequest(logsV1)
	} else {
		body := model.LogPayload{
			Message:    logMessage,
			ResourceID: resourceidMap,
			Metadata:   metadata,
			Timestamp:  timestamp,
		}
		bodyarr := append([]model.LogPayload{}, body)
		logPayloadList := internal.DataPayload{
			LogBodyList: bodyarr,
		}
		return lli.ExportData(logPayloadList, uri, http.MethodPost)
	}
	return nil
}

// addRequest adds incoming log requests to logBatch internal cache
func addRequest(logInput model.LogInput) {
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
	return uri
}

// CreateRequestBody prepares log payload from the requests present in cache after batch interval expires
func (lli *LMLogIngest) CreateRequestBody() internal.DataPayload {
	var logPayloadList []model.LogPayload
	logBatchMutex.Lock()
	defer logBatchMutex.Unlock()
	if len(logBatch) == 0 {
		return internal.DataPayload{}
	}
	for _, logsV1 := range logBatch {
		body := model.LogPayload{
			Message:    logsV1.Message,
			ResourceID: logsV1.ResourceID,
			Metadata:   logsV1.Metadata,
			Timestamp:  logsV1.Timestamp,
		}
		logPayloadList = append(logPayloadList, body)
	}
	payloadList := internal.DataPayload{
		LogBodyList: logPayloadList,
	}
	// flushing out log batch
	if lli.batch {
		logBatch = nil
	}
	return payloadList
}

// ExportData exports logs to the LM platform
func (lli *LMLogIngest) ExportData(payloadList internal.DataPayload, uri, method string) error {
	if len(payloadList.LogBodyList) > 0 {
		body, err := json.Marshal(payloadList.LogBodyList)
		if err != nil {
			return fmt.Errorf("error in marshaling log payload: %v", err)
		}
		token := lli.auth.GetCredentials(method, uri, body)
		_, err = internal.MakeRequest(lli.client, lli.url, body, uri, method, token, lli.gzip)
		if err != nil {
			return fmt.Errorf("error while exporting logs : %v", err)
		}
		return err
	}
	return nil
}
