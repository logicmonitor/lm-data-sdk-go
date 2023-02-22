package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"github.com/logicmonitor/lm-data-sdk-go/utils/translator"
	"github.com/stretchr/testify/assert"
)

func TestNewLMLogIngest(t *testing.T) {
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
			name: "New LMLog Ingest with Batching interval passed",
			args: args{
				option: []Option{
					WithLogBatchingInterval(5 * time.Second),
				},
			},
			wantBatchingEnabled: true,
			wantInterval:        5 * time.Second,
		},
		{
			name: "New LMLog Ingest without Batching enabled",
			args: args{
				option: []Option{
					WithLogBatchingDisabled(),
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

			lli, err := NewLMLogIngest(ctx, tt.args.option...)
			if err != nil {
				t.Errorf("NewLMLogIngest() error = %v", err)
				return
			}
			if lli.interval != tt.wantInterval {
				t.Errorf("NewLMLogIngest() want batch interval = %s , got = %s", tt.wantInterval, lli.interval)
				return
			}
			if lli.batch != tt.wantBatchingEnabled {
				t.Errorf("NewLMLogIngest() want batching enabled = %t , got = %t", tt.wantBatchingEnabled, lli.batch)
				return
			}
		})
	}
	cleanupLMEnv()
}

func TestSendLogs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Logs exported successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		log        string
		resourceId map[string]interface{}
		metadata   map[string]interface{}
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
		name: "Test log export without batching",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   utils.AuthParams{},
		},
		args: args{
			log:        "This is test message",
			resourceId: map[string]interface{}{"test": "resource"},
			metadata:   map[string]interface{}{"test": "metadata"},
		},
	}

	t.Run(test.name, func(t *testing.T) {
		setLMEnv()
		rateLimiter, _ := rateLimiter.NewLogRateLimiter(rateLimiter.RateLimiterSetting{RequestCount: 100})
		e := &LMLogIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			rateLimiter: rateLimiter,
		}
		payload := translator.ConvertToLMLogInput(test.args.log, time.Now().String(), test.args.resourceId, test.args.metadata)
		err := e.SendLogs(context.Background(), payload)
		if err != nil {
			t.Errorf("SendLogs() error = %v", err)
			return
		}
	})
	cleanupLMEnv()
}

func TestSendLogsError(t *testing.T) {
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
		log        string
		resourceId map[string]interface{}
		metadata   map[string]interface{}
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
			log:        "This is test message",
			resourceId: map[string]interface{}{"test": "resource"},
			metadata:   map[string]interface{}{"test": "metadata"},
		},
	}

	t.Run(test.name, func(t *testing.T) {
		setLMEnv()
		rateLimiter, _ := rateLimiter.NewLogRateLimiter(rateLimiter.RateLimiterSetting{RequestCount: 100})
		e := &LMLogIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			rateLimiter: rateLimiter,
		}
		payload := translator.ConvertToLMLogInput(test.args.log, time.Now().String(), test.args.resourceId, test.args.metadata)
		err := e.SendLogs(context.Background(), payload)
		if err == nil {
			t.Errorf("SendLogs() expected error but got = %v", err)
			return
		}
	})
	cleanupLMEnv()
}

func TestSendLogsBatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Logs exported successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		log        string
		resourceId map[string]interface{}
		metadata   map[string]interface{}
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
		name: "Test log export with batching",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   utils.AuthParams{},
		},
		args: args{
			log:        "This is test batch message",
			resourceId: map[string]interface{}{"test": "resource"},
			metadata:   map[string]interface{}{"test": "metadata"},
		},
	}

	t.Run(test.name, func(t *testing.T) {
		setLMEnv()
		rateLimiter, _ := rateLimiter.NewLogRateLimiter(rateLimiter.RateLimiterSetting{RequestCount: 100})
		e := &LMLogIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			batch:       true,
			interval:    1 * time.Second,
			rateLimiter: rateLimiter,
		}
		payload := translator.ConvertToLMLogInput(test.args.log, time.Now().String(), test.args.resourceId, test.args.metadata)
		err := e.SendLogs(context.Background(), payload)
		if err != nil {
			t.Errorf("SendLogs() error = %v", err)
			return
		}
	})
	cleanupLMEnv()
}

func TestPushToBatch(t *testing.T) {
	logInput := model.LogInput{
		Message:    "This is 1st message",
		ResourceID: map[string]interface{}{"test": "resource"},
		Metadata:   map[string]interface{}{"test": "metadata"},
		//Timestamp:  "",
	}
	before := len(logBatch)
	pushToBatch(logInput)
	after := len(logBatch)
	if after != (before + 1) {
		t.Errorf("AddRequest() error = %s", "unable to add new request to cache")
		return
	}
	logBatch = nil
}

func TestCreateRestLogsBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Logs exported successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))
	e := &LMLogIngest{
		client: ts.Client(),
		url:    ts.URL,
	}

	logInput1 := model.LogInput{
		Message:    "This is 1st message",
		ResourceID: map[string]interface{}{"test": "resource"},
		Metadata:   map[string]interface{}{"test": "metadata"},
		//Timestamp:  "",
	}
	logInput2 := model.LogInput{
		Message:    "This is 2nd message",
		ResourceID: map[string]interface{}{"test": "resource"},
		Metadata:   map[string]interface{}{"test": "metadata"},
		//Timestamp:  "",
	}
	logInput3 := model.LogInput{
		Message:    "This is 3rd message",
		ResourceID: map[string]interface{}{"test": "resource"},
		Metadata:   map[string]interface{}{"test": "metadata"},
		//Timestamp:  "",
	}
	logBatch = append(logBatch, logInput1, logInput2, logInput3)

	body := e.CreateRequestBody()
	if len(body.LogBodyList) == 0 {
		t.Errorf("CreateRequestBody() Logs error = unable to create log request body")
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

func BenchmarkSendLogs(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Logs exported successfully!!",
		}
		body, _ := json.Marshal(response)
		time.Sleep(10 * time.Millisecond)
		w.Write(body)
	}))

	type args struct {
		log        string
		resourceId map[string]interface{}
		metadata   map[string]interface{}
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
		name: "Test log export without batching",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   utils.AuthParams{},
		},
		args: args{
			log:        "This is test message",
			resourceId: map[string]interface{}{"test": "resource"},
			metadata:   map[string]interface{}{"test": "metadata"},
		},
	}
	setLMEnv()
	defer cleanupLMEnv()

	for i := 0; i < b.N; i++ {
		rateLimiter, _ := rateLimiter.NewLogRateLimiter(rateLimiter.RateLimiterSetting{RequestCount: 350})
		e := &LMLogIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			rateLimiter: rateLimiter,
		}
		payload := translator.ConvertToLMLogInput(test.args.log, time.Now().String(), test.args.resourceId, test.args.metadata)
		err := e.SendLogs(context.Background(), payload)
		if err != nil {
			fmt.Print(err)
			return
		}
	}
}

func TestBuildPayload(t *testing.T) {
	type args struct {
		log        interface{}
		timestamp  string
		resourceId map[string]interface{}
		metadata   map[string]interface{}
	}

	tests := []struct {
		name            string
		args            args
		expectedPayload model.LogPayload
	}{
		{
			name: "log message value in string format",
			args: args{
				log:        "This is test batch message",
				timestamp:  "04:33:37.4203915 +0000 UTC",
				resourceId: map[string]interface{}{"host.name": "test"},
				metadata:   map[string]interface{}{"cloud.provider": "aws"},
			},
			expectedPayload: map[string]interface{}{
				lmLogsMessageKey: "This is test batch message",
				resourceIDKey:    map[string]interface{}{"host.name": "test"},
				timestampKey:     "04:33:37.4203915 +0000 UTC",
				"cloud.provider": "aws",
			},
		},
		{
			name: "log message value in map format",
			args: args{
				log:        map[string]interface{}{"channel": "Security", "computer": "OtelDemoDevice", "details": map[string]interface{}{"Account For Which Logon Failed": map[string]interface{}{"Account Domain": "OTELDEMODEVICE", "Account Name": "Administrator Security", "ID": "S-1-0-0"}}, "message": "An account failed to log on."},
				timestamp:  "04:33:37.4203915 +0000 UTC",
				resourceId: map[string]interface{}{"host.name": "test"},
				metadata:   map[string]interface{}{"cloud.provider": "azure"},
			},
			expectedPayload: map[string]interface{}{
				lmLogsMessageKey: "An account failed to log on.",
				resourceIDKey:    map[string]interface{}{"host.name": "test"},
				timestampKey:     "04:33:37.4203915 +0000 UTC",
				"cloud.provider": "azure",
				"channel":        "Security",
				"computer":       "OtelDemoDevice",
				"details":        map[string]interface{}{"Account For Which Logon Failed": map[string]interface{}{"Account Domain": "OTELDEMODEVICE", "Account Name": "Administrator Security", "ID": "S-1-0-0"}},
			},
		},
		{
			name: "log message value from metadata",
			args: args{
				log:        nil,
				timestamp:  "04:33:37.4203915 +0000 UTC",
				resourceId: map[string]interface{}{"host.name": "test"},
				metadata: map[string]interface{}{"azure.category": "FunctionAppLogs", "azure.properties": map[string]interface{}{
					"appName":   "adityadotnet",
					"category":  "Function.ConnectDB",
					"eventId":   1,
					"eventName": "FunctionStarted",
					"level":     "Information",
					"message":   "Executing 'Functions.ConnectDB' (Reason='This function was programmatically called via the host APIs.",
				}},
			},
			expectedPayload: map[string]interface{}{
				lmLogsMessageKey: "Executing 'Functions.ConnectDB' (Reason='This function was programmatically called via the host APIs.",
				resourceIDKey:    map[string]interface{}{"host.name": "test"},
				timestampKey:     "04:33:37.4203915 +0000 UTC",
				"azure.category": "FunctionAppLogs",
				"azure.properties": map[string]interface{}{
					"appName":   "adityadotnet",
					"category":  "Function.ConnectDB",
					"eventId":   1,
					"eventName": "FunctionStarted",
					"level":     "Information",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logInput := translator.ConvertToLMLogInput(tt.args.log, tt.args.timestamp, tt.args.resourceId, tt.args.metadata)
			payload := buildPayload(logInput)
			assert.Equal(t, tt.expectedPayload, payload)
		})
	}
}
