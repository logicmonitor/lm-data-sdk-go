package traces

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
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
	type args struct {
		option []Option
	}

	tests := []struct {
		name                string
		args                args
		wantBatchingEnabled bool
		wantInterval        time.Duration
	}{
		{
			name: "New LMTraceIngest with Batching interval passed",
			args: args{
				option: []Option{
					WithTraceBatchingInterval(5 * time.Second),
				},
			},
			wantBatchingEnabled: true,
			wantInterval:        5 * time.Second,
		},
		{
			name: "New LMTraceIngest with Batching disabled",
			args: args{
				option: []Option{
					WithTraceBatchingDisabled(),
				},
			},
			wantBatchingEnabled: false,
			wantInterval:        10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setLMEnv()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			lli, err := NewLMTraceIngest(ctx, tt.args.option...)
			if err != nil {
				t.Errorf("NewLMTraceIngest() error = %v", err)
				return
			}
			if lli.interval != tt.wantInterval {
				t.Errorf("NewLMTraceIngest() want batch interval = %s , got = %s", tt.wantInterval, lli.interval)
				return
			}
			if lli.batch != tt.wantBatchingEnabled {
				t.Errorf("NewLMTraceIngest() want batching enabled = %t , got = %t", tt.wantBatchingEnabled, lli.batch)
				return
			}
		})
	}
	cleanupLMEnv()
}

func TestSendTraces(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Traces exported successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		traceData ptrace.Traces
	}

	type fields struct {
		client *http.Client
		url    string
		auth   utils.AuthParams
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test trace export without batching",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   utils.AuthParams{},
		},
		args: args{
			traceData: createTraceData(),
		},
	}

	t.Run(test.name, func(t *testing.T) {
		setLMEnv()
		rateLimiter, _ := rateLimiter.NewTraceRateLimiter(rateLimiter.RateLimiterSetting{RequestCount: 10})
		e := &LMTraceIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			rateLimiter: rateLimiter,
		}
		err := e.SendTraces(context.Background(), test.args.traceData)
		if err != nil {
			t.Errorf("SendTraces() error = %v", err)
			return
		}
	})
	cleanupLMEnv()
}

func TestSendTracesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: false,
			Message: "Connection Timeout!!",
		}
		body, _ := json.Marshal(response)
		w.WriteHeader(http.StatusBadGateway)
		w.Write(body)
	}))

	type args struct {
		traceData ptrace.Traces
	}

	type fields struct {
		client *http.Client
		url    string
		auth   utils.AuthParams
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test Connection Timeout",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   utils.AuthParams{},
		},
		args: args{
			traceData: createTraceData(),
		},
	}

	t.Run(test.name, func(t *testing.T) {
		setLMEnv()
		rateLimiter, _ := rateLimiter.NewTraceRateLimiter(rateLimiter.RateLimiterSetting{RequestCount: 100})
		e := &LMTraceIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			rateLimiter: rateLimiter,
		}
		err := e.SendTraces(context.Background(), test.args.traceData)
		if err == nil {
			t.Errorf("SendTraces() expected error but got = %v", err)
			return
		}
	})
	cleanupLMEnv()
}

func TestSendTraceBatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Traces exported successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		traceData ptrace.Traces
	}

	type fields struct {
		client *http.Client
		url    string
		auth   utils.AuthParams
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test traces export with batching",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   utils.AuthParams{},
		},
		args: args{
			traceData: createTraceData(),
		},
	}

	t.Run(test.name, func(t *testing.T) {
		setLMEnv()
		rateLimiter, _ := rateLimiter.NewTraceRateLimiter(rateLimiter.RateLimiterSetting{RequestCount: 100})
		e := &LMTraceIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			batch:       true,
			interval:    1 * time.Second,
			rateLimiter: rateLimiter,
		}
		err := e.SendTraces(context.Background(), test.args.traceData)
		if err != nil {
			t.Errorf("SendTraces() error = %v", err)
			return
		}
	})
	cleanupLMEnv()
}

func TestAddRequest(t *testing.T) {
	initializeTraceRequest()
	before := traceBatch.SpanCount
	traceInput := createTraceData()
	addRequest(traceInput)
	after := traceBatch.SpanCount
	if after != (before + 1) {
		t.Errorf("AddRequest() error = %s", "unable to add new request to cache")
		return
	}
}

func TestCreateTraceBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Traces exported successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))
	e := &LMTraceIngest{
		client: ts.Client(),
		url:    ts.URL,
	}

	initializeTraceRequest()
	traceInput1 := createTraceData()
	addRequest(traceInput1)
	traceInput2 := createTraceData()
	addRequest(traceInput2)

	body := e.CreateRequestBody()
	if body.TracePayload.SpanCount == 0 {
		t.Errorf("CreateRequestBody() Traces error = unable to create traces request body")
		return
	}
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
