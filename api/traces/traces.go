package traces

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/internal/client"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	"github.com/logicmonitor/lm-data-sdk-go/pkg/batch"
	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

const (
	uri                     = "/api/v1/traces"
	defaultBatchingInterval = 10 * time.Second
)

var (
	traceBatch      *model.TracesRequest
	traceBatchMutex sync.Mutex
)

type LMTraceIngest struct {
	client             *http.Client
	url                string
	batch              bool
	interval           time.Duration
	auth               utils.AuthParams
	gzip               bool
	rateLimiterSetting rateLimiter.RateLimiterSetting
	rateLimiter        rateLimiter.RateLimiter
}

// NewLMTraceIngest initializes LMTraceIngest
func NewLMTraceIngest(ctx context.Context, opts ...Option) (*LMTraceIngest, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false, MinVersion: tls.VersionTLS12}
	clientTransport := (http.RoundTripper)(transport)
	client := http.Client{Transport: clientTransport, Timeout: 5 * time.Second}

	lti := LMTraceIngest{
		client:             &client,
		batch:              true,
		interval:           defaultBatchingInterval,
		auth:               utils.AuthParams{},
		gzip:               true,
		rateLimiterSetting: rateLimiter.RateLimiterSetting{},
	}

	for _, opt := range opts {
		if err := opt(&lti); err != nil {
			return nil, err
		}
	}

	var err error
	if lti.url == "" {
		tracesURL, err := utils.URL()
		if err != nil {
			return nil, fmt.Errorf("error in forming Traces URL: %v", err)
		}
		lti.url = tracesURL
	}

	lti.rateLimiter, err = rateLimiter.NewTraceRateLimiter(lti.rateLimiterSetting)
	if err != nil {
		return nil, err
	}
	go lti.rateLimiter.Run(ctx)

	initializeTraceRequest()
	if lti.batch {
		go batch.CreateAndExportData(&lti)
	}
	return &lti, nil
}

// URI returns the endpoint/uri of trace ingest API
func (lti *LMTraceIngest) URI() string {
	return uri
}

// BatchInterval returns the time interval for batching
func (lti LMTraceIngest) BatchInterval() time.Duration {
	return lti.interval
}

// SendTraces is the entry point for receiving trace data
func (lti *LMTraceIngest) SendTraces(ctx context.Context, traceData ptrace.Traces) error {
	if lti.batch {
		addRequest(traceData)
	} else {
		traceBatch.TraceData = traceData
		traceBatch.SpanCount = traceData.SpanCount()
		traceDataPayload := model.DataPayload{
			TracePayload: *traceBatch,
		}

		return lti.ExportData(traceDataPayload, uri, http.MethodPost)
	}
	return nil
}

func initializeTraceRequest() {
	traceBatch = &model.TracesRequest{TraceData: ptrace.NewTraces()}
}

// addRequest adds incoming trace requests to traceBatch internal cache
func addRequest(traceInput ptrace.Traces) {
	traceBatchMutex.Lock()
	defer traceBatchMutex.Unlock()
	newSpanCount := traceInput.SpanCount()
	if newSpanCount == 0 {
		return
	}

	traceBatch.SpanCount += newSpanCount
	traceInput.ResourceSpans().MoveAndAppendTo(traceBatch.TraceData.ResourceSpans())
}

func (lti *LMTraceIngest) CreateRequestBody() model.DataPayload {
	traceBatchMutex.Lock()
	defer traceBatchMutex.Unlock()
	if traceBatch.SpanCount == 0 {
		return model.DataPayload{}
	}
	payloadList := model.DataPayload{
		TracePayload: *traceBatch,
	}

	// flushing out trace batch
	if lti.batch {
		traceBatch.SpanCount = 0
		traceBatch.TraceData = ptrace.NewTraces()
	}
	return payloadList
}

// ExportData exports trace to the LM platform
func (lti *LMTraceIngest) ExportData(payloadList model.DataPayload, uri, method string) error {
	if payloadList.TracePayload.SpanCount != 0 {
		headers := make(map[string]string)
		headers["Content-Type"] = "application/x-protobuf"

		traceData := payloadList.TracePayload.TraceData
		tr := ptraceotlp.NewExportRequestFromTraces(traceData)
		body, err := tr.MarshalProto()
		if err != nil {
			return err
		}

		token := lti.auth.GetCredentials(method, lti.URI(), body)
		cfg := client.RequestConfig{
			Client:      lti.client,
			RateLimiter: lti.rateLimiter,
			Url:         lti.url,
			Body:        body,
			Uri:         lti.URI(),
			Method:      method,
			Token:       token,
			Gzip:        lti.gzip,
			Headers:     headers,
		}

		_, err = client.MakeRequest(context.Background(), cfg)
		if err != nil {
			return fmt.Errorf("error while exporting traces : %v", err)
		}
	}
	return nil
}
