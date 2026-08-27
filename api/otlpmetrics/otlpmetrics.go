package otlpmetrics

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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
)

const (
	otlpMetricIngestURI      = "/api/v1/metrics"
	defaultBatchingInterval  = 10 * time.Second
	maxHTTPResponseReadBytes = 64 * 1024
	headerRetryAfter         = "Retry-After"
)

// LMOTLPMetricIngest sends native OTLP metrics to LogicMonitor AMP.
type LMOTLPMetricIngest struct {
	client             *http.Client
	url                string
	auth               utils.AuthParams
	gzip               bool
	rateLimiterSetting rateLimiter.MetricsRateLimiterSetting
	rateLimiter        rateLimiter.RateLimiter
	batch              *otlpMetricBatch
	collectorID        string
	userAgent          string
}

type lmOTLPMetricIngestRequest struct {
	metricsPayload model.OTLPMetricsPayload
}

type LMOTLPMetricIngestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type SendOTLPMetricResponse struct {
	StatusCode int    `json:"statusCode"`
	Success    bool   `json:"success"`
	Message    string `json:"message"`

	RetryAfter int `json:"retryAfter"`

	Error       error `json:"error"`
	MultiStatus []struct {
		Code  float64 `json:"code"`
		Error string  `json:"error"`
	} `json:"multiStatus"`
}

type otlpMetricBatch struct {
	enabled  bool
	data     *lmOTLPMetricIngestRequest
	interval time.Duration
	lock     *sync.Mutex
}

func NewLMOTLPMetricIngest(ctx context.Context, opts ...Option) (*LMOTLPMetricIngest, error) {
	ingest := LMOTLPMetricIngest{
		client:             client.New(),
		auth:               utils.AuthParams{},
		gzip:               true,
		rateLimiterSetting: rateLimiter.MetricsRateLimiterSetting{},
		batch:              NewOTLPMetricBatch(),
	}

	for _, opt := range opts {
		if err := opt(&ingest); err != nil {
			return nil, err
		}
	}

	var err error
	if ingest.url == "" {
		metricsURL, err := utils.URL()
		if err != nil {
			return nil, fmt.Errorf("NewLMOTLPMetricIngest: failed to create metrics URL: %v", err)
		}
		ingest.url = metricsURL
	}

	if ingest.rateLimiter == nil {
		ingest.rateLimiter, err = rateLimiter.NewMetricsRateLimiter(ingest.rateLimiterSetting)
		if err != nil {
			return nil, err
		}
		go ingest.rateLimiter.Run(ctx)
	}

	if ingest.batch.enabled {
		go ingest.processBatch(ctx)
	}
	return &ingest, nil
}

func NewOTLPMetricBatch() *otlpMetricBatch {
	return &otlpMetricBatch{
		enabled:  true,
		interval: defaultBatchingInterval,
		lock:     &sync.Mutex{},
		data: &lmOTLPMetricIngestRequest{
			metricsPayload: model.OTLPMetricsPayload{
				MetricData: pmetric.NewMetrics(),
			},
		},
	}
}

func (ingest *LMOTLPMetricIngest) processBatch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.NewTicker(ingest.batch.batchInterval()).C:
			req := ingest.batch.combineBatchedRequests()
			if req == nil {
				continue
			}
			_, err := ingest.export(req)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (batch *otlpMetricBatch) batchInterval() time.Duration {
	return batch.interval
}

// SendOTLPMetrics is the entry point for native OTLP metric data.
func (ingest *LMOTLPMetricIngest) SendOTLPMetrics(ctx context.Context, md pmetric.Metrics, o ...SendOTLPMetricsOptionalParameters) (*SendOTLPMetricResponse, error) {
	req, err := ingest.buildRequest(ctx, md, o...)
	if err != nil {
		return nil, err
	}

	if ingest.batch.enabled {
		ingest.batch.pushToBatch(req)
		return nil, nil
	}
	return ingest.export(req)
}

func (ingest *LMOTLPMetricIngest) buildRequest(ctx context.Context, md pmetric.Metrics, o ...SendOTLPMetricsOptionalParameters) (*lmOTLPMetricIngestRequest, error) {
	return &lmOTLPMetricIngestRequest{
		metricsPayload: model.OTLPMetricsPayload{MetricData: md},
	}, nil
}

func (batch *otlpMetricBatch) pushToBatch(req *lmOTLPMetricIngestRequest) {
	batch.lock.Lock()
	defer batch.lock.Unlock()
	req.metricsPayload.MetricData.ResourceMetrics().MoveAndAppendTo(batch.data.metricsPayload.MetricData.ResourceMetrics())
}

func (batch *otlpMetricBatch) combineBatchedRequests() *lmOTLPMetricIngestRequest {
	batch.lock.Lock()
	defer batch.lock.Unlock()

	if batch.data.metricsPayload.MetricData.DataPointCount() == 0 {
		return nil
	}

	req := &lmOTLPMetricIngestRequest{metricsPayload: batch.data.metricsPayload}
	if batch.enabled {
		batch.data.metricsPayload.MetricData = pmetric.NewMetrics()
	}
	return req
}

func (ingest *LMOTLPMetricIngest) export(req *lmOTLPMetricIngestRequest) (*SendOTLPMetricResponse, error) {
	if req.metricsPayload.MetricData.DataPointCount() == 0 {
		return nil, nil
	}
	headers := make(map[string]string)
	headers["Content-Type"] = "application/x-protobuf"

	if ingest.collectorID != "" {
		headers["Collector-ID"] = ingest.collectorID
	}

	body, err := pmetricotlp.NewExportRequestFromMetrics(req.metricsPayload.MetricData).MarshalProto()
	if err != nil {
		return nil, err
	}

	token, err := ingest.auth.GetCredentials(http.MethodPost, otlpMetricIngestURI, body)
	if err != nil {
		return nil, fmt.Errorf("LMOTLPMetricIngest.export: failed to get auth credentials: %w", err)
	}

	cfg := client.RequestConfig{
		Client:      ingest.client,
		RateLimiter: ingest.rateLimiter,
		Url:         ingest.url,
		Body:        body,
		Uri:         otlpMetricIngestURI,
		Method:      http.MethodPost,
		Token:       token,
		Gzip:        ingest.gzip,
		Headers:     headers,
		UserAgent:   ingest.userAgent,
	}

	resp, err := client.DoRequest(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("LMOTLPMetricIngest.export: otlp metrics export request failed: %w", err)
	}
	parsedResp, err := readResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("LMOTLPMetricIngest.export: failed to read response: %w", err)
	}

	sendResp := &SendOTLPMetricResponse{
		StatusCode:  parsedResp.StatusCode,
		Success:     parsedResp.Success,
		Message:     parsedResp.Message,
		Error:       parsedResp.Error,
		MultiStatus: parsedResp.MultiStatus,
		RetryAfter:  parsedResp.RetryAfter,
	}

	if !sendResp.Success {
		return sendResp, fmt.Errorf("LMOTLPMetricIngest.export: failed to export otlp metrics: %w", sendResp.Error)
	}
	return sendResp, nil
}

func readResponse(resp *http.Response) (*model.OTLPMetricsIngestAPIResponse, error) {
	defer func() {
		io.CopyN(io.Discard, resp.Body, maxHTTPResponseReadBytes) // nolint:errcheck
		resp.Body.Close()
	}()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return &model.OTLPMetricsIngestAPIResponse{
			StatusCode: resp.StatusCode,
			Success:    true,
		}, nil
	}

	parsedResponse := decodeResponse(resp)

	var formattedErr error
	if parsedResponse != nil {
		formattedErr = fmt.Errorf(
			"readResponse: error exporting items, request to %s responded with HTTP Status Code %d, Message=%s",
			resp.Request.URL, resp.StatusCode, parsedResponse.Message)
	} else {
		formattedErr = fmt.Errorf(
			"readResponse: error exporting items, request to %s responded with HTTP Status Code %d",
			resp.Request.URL, resp.StatusCode)
	}
	retryAfter := 0
	if val := resp.Header.Get(headerRetryAfter); val != "" {
		if seconds, err2 := strconv.Atoi(val); err2 == nil {
			retryAfter = seconds
		}
	}
	return &model.OTLPMetricsIngestAPIResponse{
		StatusCode: resp.StatusCode,
		Success:    false,
		Error:      formattedErr,
		RetryAfter: retryAfter,
	}, nil
}

func decodeResponse(resp *http.Response) *LMOTLPMetricIngestResponse {
	var ingestResponse *LMOTLPMetricIngestResponse
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		maxRead := resp.ContentLength
		if maxRead == -1 || maxRead > maxHTTPResponseReadBytes {
			maxRead = maxHTTPResponseReadBytes
		}
		respBytes := make([]byte, maxRead)
		n, err := io.ReadFull(resp.Body, respBytes)
		if err == nil && n > 0 {
			ingestResponse = &LMOTLPMetricIngestResponse{}
			if json.Unmarshal(respBytes, ingestResponse) != nil {
				ingestResponse = nil
			}
		}
	}
	return ingestResponse
}
