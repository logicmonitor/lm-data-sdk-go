package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

		setEnv()
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
	cleanUp()
}

func TestSendMetricsError(t *testing.T) {
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
		name: "Test metric export error scenario",
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

		setEnv()
		e := &LMMetricIngest{
			Batch:    test.fields.batch,
			Interval: test.fields.interval,
			Client:   test.fields.client,
			URL:      test.fields.url,
		}
		_, err := e.SendMetrics(test.args.rInput, test.args.dsInput, test.args.instInput, test.args.dpInput)
		if err == nil {
			t.Errorf("SendMetrics() expect error but got error = %v", err)
			return
		}
	})
	cleanUp()
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

		setEnv()
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
	cleanUp()
}

func TestAddRequest(t *testing.T) {
	prepareMetricsRequestCache()
	newReq := getSingleRequest()
	before := len(metricBatch)
	addRequest(newReq)
	after := len(metricBatch)
	if after != (before + 1) {
		t.Errorf("AddRequest() error = %s", "unable to add new request to metrics cache")
		return
	}
	metricBatch = nil
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
	_, err := e.CreateRequestBody()
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
	metricBatch = nil
}

func getInput() (model.ResourceInput, model.DatasourceInput, model.InstanceInput, model.DataPointInput) {
	rInput1 := model.ResourceInput{
		ResourceName: "test-demo_OTEL_71086",
		ResourceID:   map[string]string{"system.displayname": "test-demo_OTEL_71086"},
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
		ResourceName: "test-demo_OTEL_71086",
		ResourceID:   map[string]string{"system.displayname": "test-demo_OTEL_71086"},
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
		ResourceName: "test-demo_OTEL_71086",
		//ResourceDescription: "Testing",
		ResourceID: map[string]string{"system.displayname": "test-demo_OTEL_71086"},
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
	metricBatch = append(metricBatch, mInput, mInput1)
}

func TestUpdateResourceProperties(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Resource properties updated successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		rId     map[string]string
		resProp map[string]string
		patch   bool
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
		name: "Update resource properties",
		fields: fields{
			batch:    false,
			interval: 10,
			client:   ts.Client(),
			url:      ts.URL,
		},
		args: args{
			rId:     map[string]string{"system.displayname": "test-demo_OTEL_71086"},
			resProp: map[string]string{"new": "updatedprop"},
			patch:   false,
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setEnv()
		e := &LMMetricIngest{
			Batch:    test.fields.batch,
			Interval: test.fields.interval,
			Client:   test.fields.client,
			URL:      test.fields.url,
		}
		_, err := e.UpdateResourceProperties(test.args.rId, test.args.resProp, test.args.patch)
		if err != nil {
			t.Errorf("UpdateResourceProperties() error = %v", err)
			return
		}
	})
	cleanUp()
}

func TestUpdateResourcePropertiesValidation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Resource properties updated successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		rId     map[string]string
		resProp map[string]string
		patch   bool
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
		name: "Update resource properties validation check",
		fields: fields{
			batch:    false,
			interval: 10,
			client:   ts.Client(),
			url:      ts.URL,
		},
		args: args{
			rId:     map[string]string{"system.displayname": "test-demo_OTEL_71086"},
			resProp: map[string]string{"new": ""},
			patch:   false,
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setEnv()
		e := &LMMetricIngest{
			Batch:    test.fields.batch,
			Interval: test.fields.interval,
			Client:   test.fields.client,
			URL:      test.fields.url,
		}
		_, err := e.UpdateResourceProperties(test.args.rId, test.args.resProp, test.args.patch)
		if err == nil {
			t.Errorf("UpdateResourceProperties() expect error, but got error = nil")
			return
		}
	})
	cleanUp()
}

func TestUpdateResourcePropertiesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: false,
			Message: "Error caught...",
		}
		body, _ := json.Marshal(response)
		w.WriteHeader(http.StatusExpectationFailed)
		w.Write(body)
	}))

	type args struct {
		rId     map[string]string
		resProp map[string]string
		patch   bool
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
		name: "Update resource properties",
		fields: fields{
			batch:    false,
			interval: 10,
			client:   ts.Client(),
			url:      ts.URL,
		},
		args: args{
			rId:     map[string]string{"system.displayname": "test-demo_OTEL_71086"},
			resProp: map[string]string{"new": "updatedprop"},
			patch:   true,
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setEnv()
		e := &LMMetricIngest{
			Batch:    test.fields.batch,
			Interval: test.fields.interval,
			Client:   test.fields.client,
			URL:      test.fields.url,
		}
		_, err := e.UpdateResourceProperties(test.args.rId, test.args.resProp, test.args.patch)
		if err == nil {
			t.Errorf("UpdateResourceProperties() should generate error but error is nil")
			return
		}
	})
	cleanUp()
}

func TestUpdateInstanceProperties(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Instance properties updated successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		rId           map[string]string
		insProp       map[string]string
		dsName        string
		dsDisplayName string
		insName       string
		patch         bool
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
		name: "Update instance properties",
		fields: fields{
			batch:    false,
			interval: 10,
			client:   ts.Client(),
			url:      ts.URL,
		},
		args: args{
			rId:           map[string]string{"system.displayname": "test-demo_OTEL_71086"},
			insProp:       map[string]string{"new": "updatedprop"},
			dsName:        "TestDS",
			dsDisplayName: "TestDisplayName",
			insName:       "DataSDK",
			patch:         false,
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setEnv()
		e := &LMMetricIngest{
			Batch:    test.fields.batch,
			Interval: test.fields.interval,
			Client:   test.fields.client,
			URL:      test.fields.url,
		}
		_, err := e.UpdateInstanceProperties(test.args.rId, test.args.insProp, test.args.dsName, test.args.dsDisplayName, test.args.insName, test.args.patch)
		if err != nil {
			t.Errorf("UpdateInstanceProperties() error = %v", err)
			return
		}
	})

	cleanUp()
}

func TestUpdateInstancePropertiesValidation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: true,
			Message: "Instance properties updated successfully!!",
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		rId           map[string]string
		insProp       map[string]string
		dsName        string
		dsDisplayName string
		insName       string
		patch         bool
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
		name: "Update instance properties validation check",
		fields: fields{
			batch:    false,
			interval: 10,
			client:   ts.Client(),
			url:      ts.URL,
		},
		args: args{
			rId:           map[string]string{"system.displayname": "test-demo_OTEL_71086"},
			insProp:       map[string]string{"new": ""},
			dsName:        "TestDS",
			dsDisplayName: "TestDisplayName",
			insName:       "DataSDK",
			patch:         false,
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setEnv()
		e := &LMMetricIngest{
			Batch:    test.fields.batch,
			Interval: test.fields.interval,
			Client:   test.fields.client,
			URL:      test.fields.url,
		}
		_, err := e.UpdateInstanceProperties(test.args.rId, test.args.insProp, test.args.dsName, test.args.dsDisplayName, test.args.insName, test.args.patch)
		if err == nil {
			t.Errorf("UpdateInstanceProperties() expect error  but got error = nil")
			return
		}
	})

	cleanUp()
}

func TestUpdateInstancePropertiesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := utils.Response{
			Success: false,
			Message: "Error caught...",
		}
		body, _ := json.Marshal(response)
		w.WriteHeader(http.StatusExpectationFailed)
		w.Write(body)
	}))

	type args struct {
		rId           map[string]string
		insProp       map[string]string
		dsName        string
		dsDisplayName string
		insName       string
		patch         bool
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
		name: "Update instance properties",
		fields: fields{
			batch:    false,
			interval: 10,
			client:   ts.Client(),
			url:      ts.URL,
		},
		args: args{
			rId:           map[string]string{"system.displayname": "test-demo_OTEL_71086"},
			insProp:       map[string]string{"new": "updatedprop"},
			dsName:        "TestDS",
			dsDisplayName: "TestDisplayName",
			insName:       "DataSDK",
			patch:         true,
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setEnv()
		e := &LMMetricIngest{
			Batch:    test.fields.batch,
			Interval: test.fields.interval,
			Client:   test.fields.client,
			URL:      test.fields.url,
		}
		_, err := e.UpdateInstanceProperties(test.args.rId, test.args.insProp, test.args.dsName, test.args.dsDisplayName, test.args.insName, test.args.patch)
		if err == nil {
			t.Errorf("UpdateInstanceProperties() error expected but got error = nil")
			return
		}
	})

	cleanUp()
}

func setEnv() {
	os.Setenv("LM_COMPANY", "testenv")
	os.Setenv("LM_ACCESS_ID", "weryuifsjkf")
	os.Setenv("LM_ACCESS_KEY", "@dfsd4FDf999999FDE")
}

func cleanUp() {
	os.Unsetenv("LM_COMPANY")
	os.Unsetenv("LM_ACCESS_ID")
	os.Unsetenv("LM_ACCESS_KEY")
}
