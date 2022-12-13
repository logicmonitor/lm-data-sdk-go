package logs

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/internal/client"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	batch "github.com/logicmonitor/lm-data-sdk-go/pkg/batch"
	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

const (
	uri                     = "/log/ingest"
	message                 = "msg"
	resourceID              = "_lm.resourceId"
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
func (lli *LMLogIngest) SendLogs(ctx context.Context, logPayload model.LogInput) error {
	if lli.batch {
		addRequest(logPayload)
	} else {
		var body model.LogPayload
		body = make(map[string]interface{}, 0)
		body[message] = logPayload.Message
		body[resourceID] = logPayload.ResourceID
		body[timestampKey] = logPayload.Timestamp
		for k, v := range logPayload.Metadata {
			body[k] = v
		}

		bodyarr := append([]model.LogPayload{}, body)
		logPayloadList := model.DataPayload{
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
func (lli *LMLogIngest) CreateRequestBody() model.DataPayload {
	var logPayloadList []model.LogPayload
	logBatchMutex.Lock()
	defer logBatchMutex.Unlock()
	if len(logBatch) == 0 {
		return model.DataPayload{}
	}
	for _, logsV1 := range logBatch {
		var body model.LogPayload
		body = make(map[string]interface{}, 0)
		body[message] = logsV1.Message
		body[resourceID] = logsV1.ResourceID
		body[timestampKey] = logsV1.Timestamp
		for k, v := range logsV1.Metadata {
			body[k] = v
		}
		logPayloadList = append(logPayloadList, body)
	}
	payloadList := model.DataPayload{
		LogBodyList: logPayloadList,
	}
	// flushing out log batch
	if lli.batch {
		logBatch = nil
	}
	return payloadList
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
