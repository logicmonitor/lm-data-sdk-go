package traces

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/internal/client"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

const (
	otlpTraceIngestURI       = "/api/v1/traces"
	defaultBatchingInterval  = 10 * time.Second
	maxHTTPResponseReadBytes = 64 * 1024
	headerRetryAfter         = "Retry-After"
)

type LMTraceIngest struct {
	client             *http.Client
	url                string
	auth               utils.AuthParams
	gzip               bool
	rateLimiterSetting rateLimiter.TraceRateLimiterSetting
	rateLimiter        rateLimiter.RateLimiter
	batch              *traceBatch
}

type LMTraceIngestRequest struct {
	TracesPayload model.TracesPayload
}

type LMTraceIngestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type traceBatch struct {
	enabled  bool
	data     *LMTraceIngestRequest
	interval time.Duration
	lock     *sync.Mutex
}

// NewLMTraceIngest initializes LMTraceIngest
func NewLMTraceIngest(ctx context.Context, opts ...Option) (*LMTraceIngest, error) {
	traceIngest := LMTraceIngest{
		client:             client.Client(),
		auth:               utils.AuthParams{},
		gzip:               true,
		rateLimiterSetting: rateLimiter.TraceRateLimiterSetting{},
		batch:              NewTraceBatch(),
	}

	for _, opt := range opts {
		if err := opt(&traceIngest); err != nil {
			return nil, err
		}
	}

	var err error
	if traceIngest.url == "" {
		tracesURL, err := utils.URL()
		if err != nil {
			return nil, fmt.Errorf("error in forming Traces URL: %v", err)
		}
		traceIngest.url = tracesURL
	}

	traceIngest.rateLimiter, err = rateLimiter.NewTraceRateLimiter(traceIngest.rateLimiterSetting)
	if err != nil {
		return nil, err
	}
	go traceIngest.rateLimiter.Run(ctx)

	if traceIngest.batch.enabled {
		go traceIngest.processBatch(ctx)
	}
	return &traceIngest, nil
}

func NewTraceBatch() *traceBatch {
	return &traceBatch{enabled: true, interval: defaultBatchingInterval, lock: &sync.Mutex{}, data: &LMTraceIngestRequest{TracesPayload: model.TracesPayload{TraceData: ptrace.NewTraces()}}}
}

func (traceIngest *LMTraceIngest) processBatch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.NewTicker(traceIngest.batch.batchInterval()).C:
			req := traceIngest.batch.combineBatchedTraceRequests()
			if req == nil {
				return
			}
			_, err := traceIngest.export(req, traceIngest.uri(), http.MethodPost)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

// uri returns the endpoint/uri of trace ingest API
func (traceIngest *LMTraceIngest) uri() string {
	return otlpTraceIngestURI
}

// batchInterval returns the time interval for batching
func (batch *traceBatch) batchInterval() time.Duration {
	return batch.interval
}

// SendTraces is the entry point for receiving trace data
func (traceIngest *LMTraceIngest) SendTraces(ctx context.Context, td ptrace.Traces, o ...SendTracesOptionalParameters) (*model.IngestResponse, error) {
	req, err := traceIngest.buildTracesRequest(ctx, td, o...)
	if err != nil {
		return nil, err
	}

	if traceIngest.batch.enabled {
		traceIngest.batch.pushToBatch(req)
		return nil, nil
	}
	return traceIngest.export(req, otlpTraceIngestURI, http.MethodPost)
}

func (traceIngest *LMTraceIngest) buildTracesRequest(ctx context.Context, td ptrace.Traces, o ...SendTracesOptionalParameters) (*LMTraceIngestRequest, error) {
	tracesPayload := model.TracesPayload{
		TraceData: td,
	}

	return &LMTraceIngestRequest{TracesPayload: tracesPayload}, nil
}

// pushToBatch adds incoming trace requests to traceBatch internal cache
func (batch *traceBatch) pushToBatch(req *LMTraceIngestRequest) {
	batch.lock.Lock()
	defer batch.lock.Unlock()
	req.TracesPayload.TraceData.ResourceSpans().MoveAndAppendTo(ptrace.ResourceSpansSlice(batch.data.TracesPayload.TraceData.ResourceSpans()))
}

func (batch *traceBatch) combineBatchedTraceRequests() *LMTraceIngestRequest {
	batch.lock.Lock()
	defer batch.lock.Unlock()

	if batch.data.TracesPayload.TraceData.SpanCount() == 0 {
		return nil
	}

	req := &LMTraceIngestRequest{TracesPayload: batch.data.TracesPayload}

	// flushing out trace batch
	if batch.enabled {
		batch.data.TracesPayload.TraceData = ptrace.NewTraces()
	}
	return req
}

// export exports trace to the LM platform
func (traceIngest *LMTraceIngest) export(req *LMTraceIngestRequest, uri, method string) (*model.IngestResponse, error) {
	if req.TracesPayload.TraceData.SpanCount() == 0 {
		return nil, nil
	}
	headers := make(map[string]string)
	headers["Content-Type"] = "application/x-protobuf"

	traceData := req.TracesPayload.TraceData
	tr := ptraceotlp.NewExportRequestFromTraces(traceData)
	body, err := tr.MarshalProto()
	if err != nil {
		return nil, err
	}

	token := traceIngest.auth.GetCredentials(method, traceIngest.uri(), body)
	cfg := client.RequestConfig{
		Client:          traceIngest.client,
		RateLimiter:     traceIngest.rateLimiter,
		Url:             traceIngest.url,
		Body:            body,
		Uri:             traceIngest.uri(),
		Method:          method,
		Token:           token,
		Gzip:            traceIngest.gzip,
		Headers:         headers,
		PayloadMetadata: rateLimiter.TracePayloadMetadata{RequestSpanCount: uint64(req.TracesPayload.TraceData.SpanCount())},
	}

	resp, err := client.DoRequest(context.Background(), cfg, handleTraceExportResponse)
	if err != nil {
		return resp, fmt.Errorf("error while exporting traces: %w", err)
	}
	return resp, nil
}

// handleLogsExportResponse handles the http response returned by LM platform
func handleTraceExportResponse(ctx context.Context, resp *http.Response) (*model.IngestResponse, error) {
	defer func() {
		// Discard any remaining response body when we are done reading.
		io.CopyN(io.Discard, resp.Body, maxHTTPResponseReadBytes) // nolint:errcheck
		resp.Body.Close()
	}()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		// Request is successful.
		return &model.IngestResponse{
			StatusCode: resp.StatusCode,
			Success:    true,
		}, nil
	}

	respStatus := readResponse(resp)

	// Format the error message. Use the status if it is present in the response.
	var formattedErr error
	if respStatus != nil {
		formattedErr = fmt.Errorf(
			"error exporting items, request to %s responded with HTTP Status Code %d, Message=%s",
			resp.Request.URL, resp.StatusCode, respStatus.Message)
	} else {
		formattedErr = fmt.Errorf(
			"error exporting items, request to %s responded with HTTP Status Code %d",
			resp.Request.URL, resp.StatusCode)
	}
	retryAfter := 0
	if val := resp.Header.Get(headerRetryAfter); val != "" {
		if seconds, err2 := strconv.Atoi(val); err2 == nil {
			retryAfter = seconds
		}
	}
	return &model.IngestResponse{
		StatusCode: resp.StatusCode,
		Success:    false,
		Error:      formattedErr,
		RetryAfter: retryAfter,
	}, nil
}

// Read the response and decode
// Returns nil if the response is empty or cannot be decoded.
func readResponse(resp *http.Response) *LMTraceIngestResponse {
	var lmTraceIngestResponse *LMTraceIngestResponse
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		// Request failed. Read the body.
		maxRead := resp.ContentLength
		if maxRead == -1 || maxRead > maxHTTPResponseReadBytes {
			maxRead = maxHTTPResponseReadBytes
		}
		respBytes := make([]byte, maxRead)
		n, err := io.ReadFull(resp.Body, respBytes)
		if err == nil && n > 0 {
			lmTraceIngestResponse = &LMTraceIngestResponse{}
			err = json.Unmarshal(respBytes, lmTraceIngestResponse)
			if err != nil {
				lmTraceIngestResponse = nil
			}
		}
	}
	return lmTraceIngestResponse
}
