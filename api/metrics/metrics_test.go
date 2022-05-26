package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/logicmonitor/go-data-sdk/model"
	"github.com/logicmonitor/go-data-sdk/utils"
)

func TestNewLMMetricIngest(t *testing.T) {
	type args struct {
		batch    bool
		interval int
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "New LM Metric Ingest with Batching enabled",
			args: args{
				batch:    true,
				interval: 10,
			},
		},
		{
			name: "New LM Metric Ingest without Batching enabled",
			args: args{
				batch:    false,
				interval: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lmi := NewLMMetricIngest(tt.args.batch, tt.args.interval)
			if lmi == nil {
				t.Errorf("NewLMMetricIngest() error = %s", "unable to initialize LMMetricIngest")
				return
			}
		})
	}
}

func TestSendMetrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Metrics exported successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		rInput    model.ResourceInput
		dsInput   model.DatasourceInput
		instInput model.InstanceInput
		dpInput   model.DataPointInput
	}

	type fields struct {
		// Input configuration.
		batch    bool
		interval int
		client   *http.Client
		url      string
	}

	rInput1, dsInput1, insInput1, dpInput1 := getInput()

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test metric export without batching",
		fields: fields{
			batch:    false,
			interval: 10,
			client:   ts.Client(),
			url:      ts.URL,
		},
		args: args{
			rInput:    rInput1,
			dsInput:   dsInput1,
			instInput: insInput1,
			dpInput:   dpInput1,
		},
	}

	t.Run(test.name, func(t *testing.T) {

		os.Setenv("LM_COMPANY", "testenv")
		os.Setenv("LM_ACCESS_ID", "weryuifsjkf")
		os.Setenv("LM_ACCESS_KEY", "@dfsd4FDf999999FDE")
		e := &LMMetricIngest{
			Batch:    test.fields.batch,
			Interval: test.fields.interval,
			Client:   test.fields.client,
			URL:      test.fields.url,
		}
		_, err := e.SendMetrics(test.args.rInput, test.args.dsInput, test.args.instInput, test.args.dpInput)
		if err != nil {
			t.Errorf("SendMetrics() error = %v", err)
			return
		}
	})
}

func TestSendMetricsBatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Metrics exported successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		rInput    model.ResourceInput
		dsInput   model.DatasourceInput
		instInput model.InstanceInput
		dpInput   model.DataPointInput
	}

	type fields struct {
		// Input configuration.
		batch    bool
		interval int
		client   *http.Client
		url      string
	}

	rInput1, dsInput1, insInput1, dpInput1 := getInput()

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test metric export without batching",
		fields: fields{
			batch:    true,
			interval: 10,
			client:   ts.Client(),
			url:      ts.URL,
		},
		args: args{
			rInput:    rInput1,
			dsInput:   dsInput1,
			instInput: insInput1,
			dpInput:   dpInput1,
		},
	}

	t.Run(test.name, func(t *testing.T) {

		os.Setenv("LM_COMPANY", "testenv")
		os.Setenv("LM_ACCESS_ID", "weryuifsjkf")
		os.Setenv("LM_ACCESS_KEY", "@dfsd4FDf999999FDE")
		e := &LMMetricIngest{
			Batch:    test.fields.batch,
			Interval: test.fields.interval,
			Client:   test.fields.client,
			URL:      test.fields.url,
		}
		_, err := e.SendMetrics(test.args.rInput, test.args.dsInput, test.args.instInput, test.args.dpInput)
		if err != nil {
			t.Errorf("SendMetrics() error = %v", err)
			return
		}
	})
}

func TestAddRequest(t *testing.T) {
	var m sync.Mutex
	prepareMetricsRequestCache()
	newReq := getSingleRequest()
	before := len(batchedReq)
	addRequest(newReq, &m)
	after := len(batchedReq)
	if after != (before + 1) {
		t.Errorf("AddRequest() error = %s", "unable to add new request to metrics cache")
		return
	}
	batchedReq = nil
}

func TestMergeRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Metrics exported successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))
	e := &LMMetricIngest{
		Batch:    true,
		Interval: 10,
		Client:   ts.Client(),
		URL:      ts.URL,
	}

	prepareMetricsRequestCache()
	err := e.mergeAndCreateRequestBody()
	if instArray, ok := instanceMap["GoSDK"]; ok {
		if len(instArray) != 2 {
			t.Errorf("MergeRequest() error = %s", "unable to merge request properly")
			return
		}
	}
	if err != nil {
		t.Errorf("error while exporting metric = %v", err)
		return
	}
	batchedReq = nil
}

func getInput() (model.ResourceInput, model.DatasourceInput, model.InstanceInput, model.DataPointInput) {
	rInput1 := model.ResourceInput{
		ResourceName: "threat-hunters-hackathon-demo_OTEL_71086",
		ResourceID:   map[string]string{"system.displayname": "threat-hunters-hackathon-demo_OTEL_71086"},
	}

	dsInput1 := model.DatasourceInput{
		DataSourceName:        "GoSDK",
		DataSourceDisplayName: "GoSDK",
		DataSourceGroup:       "Sdk",
	}

	insInput1 := model.InstanceInput{
		InstanceName:       "DataSDK",
		InstanceProperties: map[string]string{"test": "datasdk"},
	}

	dpInput1 := model.DataPointInput{
		DataPointName:            "cpu",
		DataPointType:            "COUNTER",
		DataPointAggregationType: "SUM",
		Value:                    map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "124"},
	}
	return rInput1, dsInput1, insInput1, dpInput1
}

func getSingleRequest() model.MetricsInput {
	rInput1, dsInput1, insInput1, dpInput1 := getInput()
	mInput := model.MetricsInput{
		Resource:   rInput1,
		Datasource: dsInput1,
		Instance:   insInput1,
		DataPoint:  dpInput1,
	}
	return mInput
}

func prepareMetricsRequestCache() {
	rInput := model.ResourceInput{
		ResourceName: "threat-hunters-hackathon-demo_OTEL_71086",
		ResourceID:   map[string]string{"system.displayname": "threat-hunters-hackathon-demo_OTEL_71086"},
	}

	dsInput := model.DatasourceInput{
		DataSourceName:        "GoSDK",
		DataSourceDisplayName: "GoSDK",
		DataSourceGroup:       "Sdk",
	}

	insInput := model.InstanceInput{
		InstanceName:       "TelemetrySDK",
		InstanceProperties: map[string]string{"test": "telemetrysdk"},
	}

	dpInput := model.DataPointInput{
		DataPointName:            "cpu",
		DataPointType:            "GAUGE",
		DataPointAggregationType: "SUM",
		Value:                    map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "124"},
	}

	mInput := model.MetricsInput{
		Resource:   rInput,
		Datasource: dsInput,
		Instance:   insInput,
		DataPoint:  dpInput,
	}

	rInput1 := model.ResourceInput{
		ResourceName: "threat-hunters-hackathon-demo_OTEL_71086",
		//ResourceDescription: "Testing",
		ResourceID: map[string]string{"system.displayname": "threat-hunters-hackathon-demo_OTEL_71086"},
	}

	dsInput1 := model.DatasourceInput{
		DataSourceName:        "GoSDK",
		DataSourceDisplayName: "GoSDK",
		DataSourceGroup:       "Sdk",
	}

	insInput1 := model.InstanceInput{
		InstanceName:       "DataSDK",
		InstanceProperties: map[string]string{"test": "datasdk"},
	}

	dpInput1 := model.DataPointInput{
		DataPointName:            "cpu",
		DataPointType:            "COUNTER",
		DataPointAggregationType: "SUM",
		Value:                    map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "124"},
	}
	mInput1 := model.MetricsInput{
		Resource:   rInput1,
		Datasource: dsInput1,
		Instance:   insInput1,
		DataPoint:  dpInput1,
	}
	batchedReq = append(batchedReq, mInput, mInput1)
}
