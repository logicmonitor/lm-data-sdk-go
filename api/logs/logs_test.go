package logs

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
	"github.com/logicmonitor/lm-data-sdk-go/utils/translator"
	"github.com/stretchr/testify/assert"
)

func TestNewLMLogIngest(t *testing.T) {

	t.Run("should return LogIngest instance with default values", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		lli, err := NewLMLogIngest(ctx)
		assert.NoError(t, err)
		assert.Equal(t, true, lli.batch.enabled)
		assert.Equal(t, defaultBatchingInterval, lli.batch.interval)
		assert.Equal(t, true, lli.gzip)
		assert.NotNil(t, lli.client)
	})

	t.Run("should return LogIngest instance with options applied", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		lli, err := NewLMLogIngest(ctx, WithLogBatchingInterval(5*time.Second))
		assert.NoError(t, err)
		assert.Equal(t, true, lli.batch.enabled)
		assert.Equal(t, 5*time.Second, lli.batch.interval)
	})
}

func TestSendLogs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := LMLogIngestResponse{
			Success: true,
			Message: "Accepted",
		}
		w.WriteHeader(http.StatusAccepted)
		assert.NoError(t, json.NewEncoder(w).Encode(&response))
	}))

	defer ts.Close()

	t.Run("send logs without batching", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		rateLimiter, _ := rateLimiter.NewLogRateLimiter(rateLimiter.LogRateLimiterSetting{RequestCount: 100})

		e := &LMLogIngest{
			client:      ts.Client(),
			url:         ts.URL,
			auth:        utils.AuthParams{},
			rateLimiter: rateLimiter,
			batch:       &logsBatch{enabled: false},
		}

		message := "This is test message"
		resourceId := map[string]interface{}{"test": "resource"}
		metadata := map[string]interface{}{"test": "metadata"}

		payload := translator.ConvertToLMLogInput(message, time.Now().String(), resourceId, metadata)
		_, err := e.SendLogs(context.Background(), []model.LogInput{payload})
		assert.NoError(t, err)
	})

	t.Run("send logs with batching enabled", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		rateLimiter, _ := rateLimiter.NewLogRateLimiter(rateLimiter.LogRateLimiterSetting{RequestCount: 100})
		e := &LMLogIngest{
			client:      ts.Client(),
			url:         ts.URL,
			auth:        utils.AuthParams{},
			rateLimiter: rateLimiter,
			batch:       &logsBatch{enabled: true, interval: 1 * time.Second, lock: &sync.Mutex{}},
		}

		message := "This is test message"
		resourceId := map[string]interface{}{"test": "resource"}
		metadata := map[string]interface{}{"test": "metadata"}

		payload := translator.ConvertToLMLogInput(message, time.Now().String(), resourceId, metadata)
		_, err := e.SendLogs(context.Background(), []model.LogInput{payload})
		assert.NoError(t, err)
	})
}

func TestPushToBatch(t *testing.T) {
	t.Run("should add log message to batch", func(t *testing.T) {

		logInput := model.LogInput{
			Message:    "This is 1st message",
			ResourceID: map[string]interface{}{"test": "resource"},
			Metadata:   map[string]interface{}{"test": "metadata"},
		}

		logIngest := LMLogIngest{batch: NewLogBatch()}

		req, err := buildLogRequest(context.Background(), []model.LogInput{logInput})
		assert.NoError(t, err)

		before := len(logIngest.batch.data)

		logIngest.batch.pushToBatch(req)

		after := len(logIngest.batch.data)

		assert.Equal(t, before+1, after)
	})
}

func TestCombineBatchedLogRequests(t *testing.T) {
	t.Run("should merge the log requests", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := LMLogIngestResponse{
				Success: true,
				Message: "Accepted",
			}
			w.WriteHeader(http.StatusAccepted)
			assert.NoError(t, json.NewEncoder(w).Encode(&response))
		}))
		logIngest := &LMLogIngest{
			client: ts.Client(),
			url:    ts.URL,
			batch:  NewLogBatch(),
		}

		logInput1 := model.LogInput{
			Message:    "This is 1st message",
			ResourceID: map[string]interface{}{"test": "resource"},
			Metadata:   map[string]interface{}{"test": "metadata"},
		}
		logInput2 := model.LogInput{
			Message:    "This is 2nd message",
			ResourceID: map[string]interface{}{"test": "resource"},
			Metadata:   map[string]interface{}{"test": "metadata"},
		}
		logInput3 := model.LogInput{
			Message:    "This is 3rd message",
			ResourceID: map[string]interface{}{"test": "resource"},
			Metadata:   map[string]interface{}{"test": "metadata"},
		}

		req, err := buildLogRequest(context.Background(), []model.LogInput{logInput1, logInput2, logInput3})
		assert.NoError(t, err)

		logIngest.batch.pushToBatch(req)

		combinedReq := logIngest.batch.combineBatchedLogRequests()
		assert.Equal(t, 3, len(combinedReq.Payload))
	})
}

func BenchmarkSendLogs(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := LMLogIngestResponse{
			Success: true,
			Message: "Accepted",
		}
		assert.NoError(b, json.NewEncoder(w).Encode(&response))
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
		rateLimiter, _ := rateLimiter.NewLogRateLimiter(rateLimiter.LogRateLimiterSetting{RequestCount: 350})
		e := &LMLogIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			rateLimiter: rateLimiter,
		}
		payload := translator.ConvertToLMLogInput(test.args.log, time.Now().String(), test.args.resourceId, test.args.metadata)
		_, err := e.SendLogs(context.Background(), []model.LogInput{payload})
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
		expectedPayload []model.LogPayload
	}{
		{
			name: "log message value in string format",
			args: args{
				log:        "This is test batch message",
				timestamp:  "04:33:37.4203915 +0000 UTC",
				resourceId: map[string]interface{}{"host.name": "test"},
				metadata:   map[string]interface{}{"cloud.provider": "aws"},
			},
			expectedPayload: []model.LogPayload{
				{
					lmLogsMessageKey: "This is test batch message",
					resourceIDKey:    map[string]interface{}{"host.name": "test"},
					timestampKey:     "04:33:37.4203915 +0000 UTC",
					"cloud.provider": "aws",
				},
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
			expectedPayload: []model.LogPayload{
				{
					lmLogsMessageKey: "An account failed to log on.",
					resourceIDKey:    map[string]interface{}{"host.name": "test"},
					timestampKey:     "04:33:37.4203915 +0000 UTC",
					"cloud.provider": "azure",
					"channel":        "Security",
					"computer":       "OtelDemoDevice",
					"details":        map[string]interface{}{"Account For Which Logon Failed": map[string]interface{}{"Account Domain": "OTELDEMODEVICE", "Account Name": "Administrator Security", "ID": "S-1-0-0"}},
				},
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
			expectedPayload: []model.LogPayload{
				{
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logInput := translator.ConvertToLMLogInput(tt.args.log, tt.args.timestamp, tt.args.resourceId, tt.args.metadata)
			payload := buildLogPayload([]model.LogInput{logInput})
			assert.Equal(t, tt.expectedPayload, payload)
		})
	}
}

func TestHandleLogsExportResponse(t *testing.T) {
	t.Run("should handle success response", func(t *testing.T) {
		ingestResponse, err := handleLogsExportResponse(context.Background(), &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       ioutil.NopCloser(bytes.NewBufferString("Accepted")),
		})
		assert.NoError(t, err)
		assert.Equal(t, model.IngestResponse{
			Success:    true,
			StatusCode: http.StatusAccepted,
		}, *ingestResponse)
	})

	t.Run("should handle multi-status response", func(t *testing.T) {
		data := []byte(`{
			"success": false,
			"message": "Some events were not accepted. See the 'errors' property for additional information.",
			"errors": [
			  {
				"code": 4001,
				"error": "Resource not found",
				"event": {
				  "_lm.resourceId": {
					"system.deviceId": "kish"
				  },
				  "message": "test"
				}
			  }
			]
		  }`)
		ingestResponse, err := handleLogsExportResponse(context.Background(), &http.Response{
			StatusCode:    http.StatusMultiStatus,
			ContentLength: int64(len(data)),
			Request:       httptest.NewRequest(http.MethodPost, "https://example.logicmonitor.com"+logIngestURI, nil),
			Body:          ioutil.NopCloser(bytes.NewReader(data)),
		})
		assert.NoError(t, err)
		assert.Equal(t, model.IngestResponse{
			Success:    false,
			StatusCode: http.StatusMultiStatus,
			MultiStatus: []struct {
				Code  float64 `json:"code"`
				Error string  `json:"error"`
			}{
				{
					Code:  float64(4001),
					Error: "Resource not found",
				},
			},
			Error:   fmt.Errorf("error exporting items, request to https://example.logicmonitor.com/log/ingest responded with HTTP Status Code 207, Message: Some events were not accepted. See the 'errors' property for additional information., Details=error code: [4001], error message: Resource not found"),
			Message: "Some events were not accepted. See the 'errors' property for additional information.",
		}, *ingestResponse)
	})

	t.Run("should handle non multi-status response", func(t *testing.T) {
		data := []byte(`{
			"success": false,
			"message": "Too Many Requests"
		  }`)
		ingestResponse, err := handleLogsExportResponse(context.Background(), &http.Response{
			StatusCode:    http.StatusTooManyRequests,
			ContentLength: int64(len(data)),
			Request:       httptest.NewRequest(http.MethodPost, "https://example.logicmonitor.com"+logIngestURI, nil),
			Body:          ioutil.NopCloser(bytes.NewReader(data)),
		})
		assert.NoError(t, err)
		assert.Equal(t, model.IngestResponse{
			Success:    false,
			StatusCode: http.StatusTooManyRequests,
			Error:      fmt.Errorf("error exporting items, request to https://example.logicmonitor.com/log/ingest responded with HTTP Status Code 429, Message: Too Many Requests, Details=Too Many Requests"),
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
