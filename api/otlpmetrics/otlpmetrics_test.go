package otlpmetrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/internal/testutil"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestNewLMOTLPMetricIngest(t *testing.T) {
	testutil.SetTestLMEnvVars()
	defer testutil.CleanupTestLMEnvVars()

	t.Run("should return instance with default values", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		client, err := NewLMOTLPMetricIngest(ctx)
		assert.NoError(t, err)
		assert.Equal(t, true, client.batch.enabled)
		assert.Equal(t, defaultBatchingInterval, client.batch.interval)
		assert.Equal(t, true, client.gzip)
		assert.NotNil(t, client.client)
	})

	t.Run("should apply options", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		client, err := NewLMOTLPMetricIngest(ctx, WithOTLPMetricBatchingInterval(5*time.Second))
		assert.NoError(t, err)
		assert.Equal(t, 5*time.Second, client.batch.interval)
	})
}

func TestSendOTLPMetrics(t *testing.T) {
	testutil.SetTestLMEnvVars()
	defer testutil.CleanupTestLMEnvVars()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, otlpMetricIngestURI, r.URL.Path)
		assert.Equal(t, "application/x-protobuf", r.Header.Get("Content-Type"))
		response := LMOTLPMetricIngestResponse{
			Success: true,
			Message: "Accepted",
		}
		w.WriteHeader(http.StatusAccepted)
		assert.NoError(t, json.NewEncoder(w).Encode(&response))
	}))
	defer ts.Close()

	t.Run("send without batching", func(t *testing.T) {
		rl, _ := rateLimiter.NewMetricsRateLimiter(rateLimiter.MetricsRateLimiterSetting{RequestCount: 100})
		e := &LMOTLPMetricIngest{
			client:      ts.Client(),
			url:         ts.URL,
			auth:        utils.AuthParams{},
			rateLimiter: rl,
			batch:       &otlpMetricBatch{enabled: false},
		}

		resp, err := e.SendOTLPMetrics(context.Background(), createMetricData())
		assert.NoError(t, err)
		assert.True(t, resp.Success)
	})

	t.Run("send with batching enabled", func(t *testing.T) {
		rl, _ := rateLimiter.NewMetricsRateLimiter(rateLimiter.MetricsRateLimiterSetting{RequestCount: 100})
		e := &LMOTLPMetricIngest{
			client:      ts.Client(),
			url:         ts.URL,
			auth:        utils.AuthParams{},
			rateLimiter: rl,
			batch: &otlpMetricBatch{
				enabled:  true,
				interval: 1 * time.Second,
				lock:     &sync.Mutex{},
				data:     &lmOTLPMetricIngestRequest{metricsPayload: model.OTLPMetricsPayload{MetricData: pmetric.NewMetrics()}},
			},
		}
		_, err := e.SendOTLPMetrics(context.Background(), createMetricData())
		assert.NoError(t, err)
	})
}

func TestPushToBatch(t *testing.T) {
	ingest := LMOTLPMetricIngest{batch: NewOTLPMetricBatch()}
	testData := createMetricData()
	n := testData.DataPointCount()
	req, err := ingest.buildRequest(context.Background(), testData)
	assert.NoError(t, err)

	before := ingest.batch.data.metricsPayload.MetricData.DataPointCount()
	ingest.batch.pushToBatch(req)
	assert.Equal(t, before+n, ingest.batch.data.metricsPayload.MetricData.DataPointCount())
}

func TestReadResponse(t *testing.T) {
	t.Run("success response", func(t *testing.T) {
		ingestResponse, err := readResponse(&http.Response{
			StatusCode: http.StatusAccepted,
			Body:       ioutil.NopCloser(bytes.NewBufferString("Accepted")),
		})
		require.NoError(t, err)
		assert.Equal(t, model.OTLPMetricsIngestAPIResponse{
			Success:    true,
			StatusCode: http.StatusAccepted,
		}, *ingestResponse)
	})

	t.Run("error response", func(t *testing.T) {
		data := []byte(`{
			"success": false,
			"message": "Too Many Requests"
		  }`)
		ingestResponse, err := readResponse(&http.Response{
			StatusCode:    http.StatusTooManyRequests,
			ContentLength: int64(len(data)),
			Request:       httptest.NewRequest(http.MethodPost, "https://example.logicmonitor.com"+otlpMetricIngestURI, nil),
			Body:          ioutil.NopCloser(bytes.NewReader(data)),
		})
		require.NoError(t, err)
		assert.Equal(t, model.OTLPMetricsIngestAPIResponse{
			Success:    false,
			StatusCode: http.StatusTooManyRequests,
			Error:      fmt.Errorf("readResponse: error exporting items, request to https://example.logicmonitor.com%s responded with HTTP Status Code 429, Message=Too Many Requests", otlpMetricIngestURI),
		}, *ingestResponse)
	})
}

func createMetricData() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("test.gauge")
	g := m.SetEmptyGauge()
	dp := g.DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.5)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(100, 0)))
	return md
}
