package logs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/logicmonitor/lm-data-sdk-go/model"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

func TestNewLMLogIngest(t *testing.T) {
	type args struct {
		batch    bool
		interval int
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "New LMLog Ingest with Batching enabled",
			args: args{
				batch:    true,
				interval: 10,
			},
		},
		{
			name: "New LMLog Ingest without Batching enabled",
			args: args{
				batch:    false,
				interval: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lli := NewLMLogIngest(tt.args.batch, tt.args.interval)
			if lli == nil {
				t.Errorf("NewLMLogIngest() error = %s", "unable to initialize LMLogIngest")
				return
			}
		})
	}
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
		// Input configuration.
		batch    bool
		interval int
		client   *http.Client
		url      string
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test log export without batching",
		fields: fields{
			batch:    false,
			interval: 10,
			client:   ts.Client(),
			url:      ts.URL,
		},
		args: args{
			log:        "This is test message",
			resourceId: map[string]string{"test": "resource"},
			metadata:   map[string]string{"test": "metadata"},
		},
	}

	t.Run(test.name, func(t *testing.T) {

		os.Setenv("LM_COMPANY", "testenv")
		os.Setenv("LM_ACCESS_ID", "weryuifsjkf")
		os.Setenv("LM_ACCESS_KEY", "@dfsd4FDf999999FDE")
		e := &LMLogIngest{
			batch:    test.fields.batch,
			interval: test.fields.interval,
			client:   test.fields.client,
			url:      test.fields.url,
		}
		_, err := e.SendLogs(test.args.log, test.args.resourceId, test.args.metadata)
		if err != nil {
			t.Errorf("SendLogs() error = %v", err)
			return
		}
	})
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
		// Input configuration.
		batch    bool
		interval int
		client   *http.Client
		url      string
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test log export with batching",
		fields: fields{
			batch:    true,
			interval: 10,
			client:   ts.Client(),
			url:      ts.URL,
		},
		args: args{
			log:        "This is test batch message",
			resourceId: map[string]string{"test": "resource"},
			metadata:   map[string]string{"test": "metadata"},
		},
	}

	t.Run(test.name, func(t *testing.T) {

		os.Setenv("LM_COMPANY", "testenv")
		os.Setenv("LM_ACCESS_ID", "weryuifsjkf")
		os.Setenv("LM_ACCESS_KEY", "@dfsd4FDf999999FDE")
		e := &LMLogIngest{
			batch:    test.fields.batch,
			interval: test.fields.interval,
			client:   test.fields.client,
			url:      test.fields.url,
		}
		_, err := e.SendLogs(test.args.log, test.args.resourceId, test.args.metadata)
		if err != nil {
			t.Errorf("SendLogs() error = %v", err)
			return
		}
	})
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
		batch:    false,
		interval: 0,
		client:   ts.Client(),
		url:      ts.URL,
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

	_, err := e.CreateRequestBody()
	if err != nil {
		t.Errorf("CreateRequestBody() Logs error = %v", err)
		return
	}
}
