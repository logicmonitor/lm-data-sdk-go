package logs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

func TestNewLMLogIngest(t *testing.T) {
	type args struct {
		option []Option
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "New LMLog Ingest with Batching enabled",
			args: args{
				option: []Option{
					WithLogBatchingEnabled(5 * time.Second),
				},
			},
		},
		{
			name: "New LMLog Ingest without Batching enabled",
			args: args{
				option: []Option{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv()

			_, err := NewLMLogIngest(context.Background(), tt.args.option...)
			if err != nil {
				t.Errorf("NewLMLogIngest() error = %v", err)
				return
			}
		})
	}
	cleanupEnv()
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
		resourceId map[string]string
		metadata   map[string]string
	}

	type fields struct {
		client *http.Client
		url    string
		auth   model.AuthProvider
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
			auth:   model.DefaultAuthenticator{},
		},
		args: args{
			log:        "This is test message",
			resourceId: map[string]string{"test": "resource"},
			metadata:   map[string]string{"test": "metadata"},
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setEnv()
		e := &LMLogIngest{
			client: test.fields.client,
			url:    test.fields.url,
			auth:   test.fields.auth,
		}
		_, err := e.SendLogs(context.Background(), test.args.log, test.args.resourceId, test.args.metadata)
		if err != nil {
			t.Errorf("SendLogs() error = %v", err)
			return
		}
	})
	cleanupEnv()
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
		resourceId map[string]string
		metadata   map[string]string
	}

	type fields struct {
		client *http.Client
		url    string
		auth   model.AuthProvider
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
			auth:   model.DefaultAuthenticator{},
		},
		args: args{
			log:        "This is test message",
			resourceId: map[string]string{"test": "resource"},
			metadata:   map[string]string{"test": "metadata"},
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setEnv()
		e := &LMLogIngest{
			client: test.fields.client,
			url:    test.fields.url,
			auth:   test.fields.auth,
		}
		_, err := e.SendLogs(context.Background(), test.args.log, test.args.resourceId, test.args.metadata)
		if err == nil {
			t.Errorf("SendLogs() expected error but got = %v", err)
			return
		}
	})
	cleanupEnv()
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
		resourceId map[string]string
		metadata   map[string]string
	}

	type fields struct {
		client *http.Client
		url    string
		auth   model.AuthProvider
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
			auth:   model.DefaultAuthenticator{},
		},
		args: args{
			log:        "This is test batch message",
			resourceId: map[string]string{"test": "resource"},
			metadata:   map[string]string{"test": "metadata"},
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setLMEnv()
		option := []Option{
			WithLogBatchingEnabled(5 * time.Second),
		}
		e, err := NewLMLogIngest(context.Background(), option...)
		_, err = e.SendLogs(context.Background(), test.args.log, test.args.resourceId, test.args.metadata)
		if err != nil {
			t.Errorf("SendLogs() error = %v", err)
			return
		}
	})
	cleanupLMEnv()
}

func TestAddRequest(t *testing.T) {
	logInput := model.LogInput{
		Message:    "This is 1st message",
		ResourceID: map[string]string{"test": "resource"},
		Metadata:   map[string]string{"test": "metadata"},
		//Timestamp:  "",
	}
	before := len(logBatch)
	addRequest(logInput)
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
		ResourceID: map[string]string{"test": "resource"},
		Metadata:   map[string]string{"test": "metadata"},
		//Timestamp:  "",
	}
	logInput2 := model.LogInput{
		Message:    "This is 2nd message",
		ResourceID: map[string]string{"test": "resource"},
		Metadata:   map[string]string{"test": "metadata"},
		//Timestamp:  "",
	}
	logInput3 := model.LogInput{
		Message:    "This is 3rd message",
		ResourceID: map[string]string{"test": "resource"},
		Metadata:   map[string]string{"test": "metadata"},
		//Timestamp:  "",
	}
	logBatch = append(logBatch, logInput1, logInput2, logInput3)

	body := e.CreateRequestBody()
	if len(body.LogBodyList) == 0 {
		t.Errorf("CreateRequestBody() Logs error = unable to create log request body")
		return
	}
}

func setEnv() {
	os.Setenv("LM_ACCOUNT", "testenv")
	os.Setenv("LM_ACCESS_ID", "weryuifsjkf")
	os.Setenv("LM_ACCESS_KEY", "@dfsd4FDf999999FDE")
}
func setLMEnv() {
	os.Setenv("LOGICMONITOR_ACCOUNT", "testenv")
	os.Setenv("LOGICMONITOR_ACCESS_ID", "weryuifsjkf")
	os.Setenv("LOGICMONITOR_ACCESS_KEY", "@dfsd4FDf999999FDE")
}

func cleanupEnv() {
	os.Unsetenv("LM_ACCOUNT")
	os.Unsetenv("LM_ACCESS_ID")
	os.Unsetenv("LM_ACCESS_KEY")
}
func cleanupLMEnv() {
	os.Unsetenv("LOGICMONITOR_ACCOUNT")
	os.Unsetenv("LOGICMONITOR_ACCESS_ID")
	os.Unsetenv("LOGICMONITOR_ACCESS_KEY")
}
