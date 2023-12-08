package traces

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var (
	TestSpanStartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestSpanStartTimestamp = pcommon.NewTimestampFromTime(TestSpanStartTime)

	TestSpanEventTime      = time.Date(2020, 2, 11, 20, 26, 13, 123, time.UTC)
	TestSpanEventTimestamp = pcommon.NewTimestampFromTime(TestSpanEventTime)

	TestSpanEndTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestSpanEndTimestamp = pcommon.NewTimestampFromTime(TestSpanEndTime)
)

func TestNewLMTraceIngest(t *testing.T) {
	t.Run("should return Trace Ingest instance with default values", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		lli, err := NewLMTraceIngest(ctx)
		assert.NoError(t, err)
		assert.Equal(t, true, lli.batch.enabled)
		assert.Equal(t, defaultBatchingInterval, lli.batch.interval)
		assert.Equal(t, true, lli.gzip)
		assert.NotNil(t, lli.client)
	})

	t.Run("should return Trace Ingest instance with options applied", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		lli, err := NewLMTraceIngest(ctx, WithTraceBatchingInterval(5*time.Second))
		assert.NoError(t, err)
		assert.Equal(t, true, lli.batch.enabled)
		assert.Equal(t, 5*time.Second, lli.batch.interval)
	})
}

func TestSendTraces(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := LMTraceIngestResponse{
			Success: true,
			Message: "Accepted",
		}
		w.WriteHeader(http.StatusAccepted)
		assert.NoError(t, json.NewEncoder(w).Encode(&response))
	}))

	defer ts.Close()

	t.Run("send traces without batching", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		rateLimiter, _ := rateLimiter.NewLogRateLimiter(rateLimiter.LogRateLimiterSetting{RequestCount: 100})

		e := &LMTraceIngest{
			client:      ts.Client(),
			url:         ts.URL,
			auth:        utils.AuthParams{},
			rateLimiter: rateLimiter,
			batch:       &traceBatch{enabled: false},
		}

		_, err := e.SendTraces(context.Background(), createTraceData())
		assert.NoError(t, err)
	})

	t.Run("send traces with batching enabled", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		rateLimiter, _ := rateLimiter.NewLogRateLimiter(rateLimiter.LogRateLimiterSetting{RequestCount: 100})
		e := &LMTraceIngest{
			client:      ts.Client(),
			url:         ts.URL,
			auth:        utils.AuthParams{},
			rateLimiter: rateLimiter,
			batch:       &traceBatch{enabled: true, interval: 1 * time.Second, lock: &sync.Mutex{}, data: &LMTraceIngestRequest{TracesPayload: model.TracesPayload{TraceData: ptrace.NewTraces()}}},
		}
		_, err := e.SendTraces(context.Background(), createTraceData())
		assert.NoError(t, err)
	})
}

func TestPushToBatch(t *testing.T) {
	t.Run("should add traces to batch", func(t *testing.T) {

		traceIngest := LMTraceIngest{batch: NewTraceBatch()}

		testData := createTraceData()

		req, err := traceIngest.buildTracesRequest(context.Background(), createTraceData())
		assert.NoError(t, err)

		before := traceIngest.batch.data.TracesPayload.TraceData.SpanCount()

		traceIngest.batch.pushToBatch(req)

		expectedSpanCount := before + testData.SpanCount()

		assert.Equal(t, expectedSpanCount, traceIngest.batch.data.TracesPayload.TraceData.SpanCount())
	})
}

func TestHandleTracesExportResponse(t *testing.T) {
	t.Run("should handle success response", func(t *testing.T) {
		ingestResponse, err := handleTraceExportResponse(context.Background(), &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       ioutil.NopCloser(bytes.NewBufferString("Accepted")),
		})
		assert.NoError(t, err)
		assert.Equal(t, model.IngestResponse{
			Success:    true,
			StatusCode: http.StatusAccepted,
		}, *ingestResponse)
	})

	t.Run("should handle non multi-status response", func(t *testing.T) {
		data := []byte(`{
			"success": false,
			"message": "Too Many Requests"
		  }`)
		ingestResponse, err := handleTraceExportResponse(context.Background(), &http.Response{
			StatusCode:    http.StatusTooManyRequests,
			ContentLength: int64(len(data)),
			Request:       httptest.NewRequest(http.MethodPost, "https://example.logicmonitor.com"+otlpTraceIngestURI, nil),
			Body:          ioutil.NopCloser(bytes.NewReader(data)),
		})
		assert.NoError(t, err)
		assert.Equal(t, model.IngestResponse{
			Success:    false,
			StatusCode: http.StatusTooManyRequests,
			Error:      fmt.Errorf("error exporting items, request to https://example.logicmonitor.com%s responded with HTTP Status Code 429, Message=Too Many Requests", otlpTraceIngestURI),
		}, *ingestResponse)
	})
}

func setLMEnv() {
	os.Setenv("LOGICMONITOR_ACCOUNT", "testenv")
	os.Setenv("LOGICMONITOR_ACCESS_ID", "weryuifsjkf")
	os.Setenv("LOGICMONITOR_ACCESS_KEY", "@dfsd4FDf999999FDE")
}

func cleanupLMEnv() {
	os.Unsetenv("LOGICMONITOR_ACCOUNT")
	os.Unsetenv("LOGICMONITOR_ACCESS_ID")
	os.Unsetenv("LOGICMONITOR_ACCESS_KEY")
}

// func BenchmarkSendTraces(b *testing.B) {
// 	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		response := utils.Response{
// 			Success: true,
// 			Message: "Traces exported successfully!!",
// 		}
// 		body, _ := json.Marshal(response)
// 		time.Sleep(10 * time.Millisecond)
// 		w.Write(body)
// 	}))

// 	type args struct {
// 		traceData ptrace.Traces
// 	}

// 	type fields struct {
// 		client *http.Client
// 		url    string
// 		auth   utils.AuthParams
// 	}

// 	test := struct {
// 		name   string
// 		fields fields
// 		args   args
// 	}{
// 		name: "Test trace export without batching",
// 		fields: fields{
// 			client: ts.Client(),
// 			url:    ts.URL,
// 			auth:   utils.AuthParams{},
// 		},
// 		args: args{
// 			traceData: createTraceData(),
// 		},
// 	}
// 	setLMEnv()
// 	defer cleanupLMEnv()

// 	for i := 0; i < b.N; i++ {
// 		rateLimiter, _ := rateLimiter.NewTraceRateLimiter(rateLimiter.RateLimiterSetting{RequestCount: 350})
// 		e := &LMTraceIngest{
// 			client:      test.fields.client,
// 			url:         test.fields.url,
// 			auth:        test.fields.auth,
// 			rateLimiter: rateLimiter,
// 		}
// 		err := e.SendTraces(context.Background(), test.args.traceData)
// 		if err != nil {
// 			fmt.Print(err)
// 			return
// 		}
// 	}
// }

func createTraceData() ptrace.Traces {
	td := GenerateTracesOneEmptyInstrumentationLibrary()
	scopespan := td.ResourceSpans().At(0).ScopeSpans().At(0)
	fillSpanOne(scopespan.Spans().AppendEmpty())
	return td
}

func GenerateTracesOneEmptyInstrumentationLibrary() ptrace.Traces {
	td := GenerateTracesNoLibraries()
	td.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
	return td
}

func GenerateTracesNoLibraries() ptrace.Traces {
	td := GenerateTracesOneEmptyResourceSpans()
	return td
}

func GenerateTracesOneEmptyResourceSpans() ptrace.Traces {
	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty()
	return td
}

func fillSpanOne(span ptrace.Span) {
	span.SetName("operationA")
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
	span.SetDroppedAttributesCount(1)
	evs := span.Events()
	ev0 := evs.AppendEmpty()
	ev0.SetTimestamp(TestSpanEventTimestamp)
	ev0.SetName("event-with-attr")
	//initSpanEventAttributes(ev0.Attributes())
	ev0.SetDroppedAttributesCount(2)
	ev1 := evs.AppendEmpty()
	ev1.SetTimestamp(TestSpanEventTimestamp)
	ev1.SetName("event")
	ev1.SetDroppedAttributesCount(2)
	span.SetDroppedEventsCount(1)
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
}
