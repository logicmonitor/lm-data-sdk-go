package metrics

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
	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

func TestNewLMMetricIngest(t *testing.T) {
	type args struct {
		option []Option
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "New LM Metric Ingest with Batching enabled",
			args: args{
				option: []Option{
					WithMetricBatchingEnabled(5 * time.Second),
				},
			},
		},
		{
			name: "New LM Metric Ingest without Batching enabled",
			args: args{
				option: []Option{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv()
			_, err := NewLMMetricIngest(context.Background(), tt.args.option...)
			if err != nil {
				t.Errorf("NewLMMetricIngest() error = %v", err)
				return
			}
		})
	}
	cleanUp()
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
		client *http.Client
		url    string
		auth   model.AuthProvider
	}

	rInput1, dsInput1, insInput1, dpInput1 := getInput()

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test metric export without batching",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   model.DefaultAuthenticator{},
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
			client: test.fields.client,
			url:    test.fields.url,
			auth:   test.fields.auth,
		}
		err := e.SendMetrics(context.Background(), test.args.rInput, test.args.dsInput, test.args.instInput, test.args.dpInput)
		if err != nil {
			t.Errorf("SendMetrics() error = %v", err)
			return
		}
	})
	cleanUp()
}

func TestSendMetricsResCreate(t *testing.T) {
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
		client *http.Client
		url    string
		auth   model.AuthProvider
	}

	rInput1, dsInput1, insInput1, dpInput1 := getInputResCreate()

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test metric export without batching",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   model.DefaultAuthenticator{},
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
			client: test.fields.client,
			url:    test.fields.url,
			auth:   test.fields.auth,
		}
		err := e.SendMetrics(context.Background(), test.args.rInput, test.args.dsInput, test.args.instInput, test.args.dpInput)
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
		client *http.Client
		url    string
		auth   model.AuthProvider
	}

	rInput1, dsInput1, insInput1, dpInput1 := getInput()

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test metric export error scenario",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   model.DefaultAuthenticator{},
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
			client: test.fields.client,
			url:    test.fields.url,
			auth:   test.fields.auth,
		}
		err := e.SendMetrics(context.Background(), test.args.rInput, test.args.dsInput, test.args.instInput, test.args.dpInput)
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
		client *http.Client
		url    string
		auth   model.AuthProvider
	}

	rInput1, dsInput1, insInput1, dpInput1 := getInput()

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test metric export with batching",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   model.DefaultAuthenticator{},
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
		prepareMetricsRequestCache()
		e := &LMMetricIngest{
			client:   test.fields.client,
			url:      test.fields.url,
			auth:     test.fields.auth,
			batch:    true,
			interval: 2 * time.Second,
		}
		err := e.SendMetrics(context.Background(), test.args.rInput, test.args.dsInput, test.args.instInput, test.args.dpInput)
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
		client: ts.Client(),
		url:    ts.URL,
		auth:   model.DefaultAuthenticator{},
		batch:  true,
	}

	prepareMetricsRequestCache()
	body := e.CreateRequestBody()
	if instArray, ok := instanceMap["GoSDK"]; ok {
		if len(instArray) != 2 {
			t.Errorf("MergeRequest() error = %s", "unable to merge request properly")
			return
		}
	}
	if body.MetricBodyList == nil {
		t.Errorf("error while creating metric request body..")
		return
	}
	metricBatch = nil
}

func getInput() (model.ResourceInput, model.DatasourceInput, model.InstanceInput, model.DataPointInput) {
	rInput1 := model.ResourceInput{
		ResourceName: "test-cart-service",
		ResourceID:   map[string]string{"system.displayname": "test-cart-service"},
	}

	dsInput1 := model.DatasourceInput{
		DataSourceName:  "GoSDK",
		DataSourceGroup: "Sdk",
	}

	insInput1 := model.InstanceInput{
		InstanceName:       "DataSDK",
		InstanceProperties: map[string]string{"test": "datasdk"},
	}

	dpInput1 := model.DataPointInput{
		DataPointName: "cpu",
		DataPointType: "COUNTER",
		Value:         map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "124"},
	}
	return rInput1, dsInput1, insInput1, dpInput1
}

func getInputResCreate() (model.ResourceInput, model.DatasourceInput, model.InstanceInput, model.DataPointInput) {
	rInput1 := model.ResourceInput{
		ResourceName: "test-cart-service",
		ResourceID:   map[string]string{"system.displayname": "test-cart-service"},
		IsCreate:     true,
	}

	dsInput1 := model.DatasourceInput{
		DataSourceName:  "GoSDK",
		DataSourceGroup: "Sdk",
	}

	insInput1 := model.InstanceInput{
		InstanceName:       "DataSDK",
		InstanceProperties: map[string]string{"test": "datasdk"},
	}

	dpInput1 := model.DataPointInput{
		DataPointName: "cpu",
		DataPointType: "COUNTER",
		Value:         map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "124"},
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
		ResourceName: "test-cart-service",
		ResourceID:   map[string]string{"system.displayname": "test-cart-service"},
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
		ResourceName: "test-payment-service",
		ResourceID:   map[string]string{"system.displayname": "test-cart-service"},
		IsCreate:     true,
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
		DataPointName:            "memory",
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
	dpInput2 := model.DataPointInput{
		DataPointName:            "cpu",
		DataPointType:            "COUNTER",
		DataPointAggregationType: "SUM",
		Value:                    map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "14"},
	}
	mInput2 := model.MetricsInput{
		Resource:   rInput1,
		Datasource: dsInput1,
		Instance:   insInput1,
		DataPoint:  dpInput2,
	}
	metricBatch = append(metricBatch, mInput, mInput1, mInput2)
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
		resName string
		rId     map[string]string
		resProp map[string]string
		patch   bool
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
		name: "Update resource properties",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   model.DefaultAuthenticator{},
		},
		args: args{
			resName: "TestResource",
			rId:     map[string]string{"system.displayname": "test-cart-service"},
			resProp: map[string]string{"new": "updatedprop"},
			patch:   false,
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setEnv()
		e := &LMMetricIngest{
			client: test.fields.client,
			url:    test.fields.url,
			auth:   test.fields.auth,
		}
		err := e.UpdateResourceProperties(test.args.resName, test.args.rId, test.args.resProp, test.args.patch)
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
		resName string
		rId     map[string]string
		resProp map[string]string
		patch   bool
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
		name: "Update resource properties validation check",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   model.DefaultAuthenticator{},
		},
		args: args{
			resName: "Test",
			rId:     map[string]string{"system.displayname": "test-cart-service"},
			resProp: map[string]string{"new": ""},
			patch:   false,
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setEnv()
		e := &LMMetricIngest{
			client: test.fields.client,
			url:    test.fields.url,
			auth:   test.fields.auth,
		}
		err := e.UpdateResourceProperties(test.args.resName, test.args.rId, test.args.resProp, test.args.patch)
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
		resName string
		rId     map[string]string
		resProp map[string]string
		patch   bool
	}

	type fields struct {
		client *http.Client
		url    string
		auth   model.DefaultAuthenticator
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Update resource properties",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   model.DefaultAuthenticator{},
		},
		args: args{
			resName: "Test",
			rId:     map[string]string{"system.displayname": "test-cart-service"},
			resProp: map[string]string{"new": "updatedprop"},
			patch:   true,
		},
	}

	t.Run(test.name, func(t *testing.T) {

		setEnv()
		e := &LMMetricIngest{
			client: test.fields.client,
			url:    test.fields.url,
			auth:   test.fields.auth,
		}
		err := e.UpdateResourceProperties(test.args.resName, test.args.rId, test.args.resProp, test.args.patch)
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
		client *http.Client
		url    string
		auth   model.AuthProvider
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Update instance properties",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   model.DefaultAuthenticator{},
		},
		args: args{
			rId:           map[string]string{"system.displayname": "test-cart-service"},
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
			client: test.fields.client,
			url:    test.fields.url,
			auth:   test.fields.auth,
		}
		err := e.UpdateInstanceProperties(test.args.rId, test.args.insProp, test.args.dsName, test.args.dsDisplayName, test.args.insName, test.args.patch)
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
		client *http.Client
		url    string
		auth   model.AuthProvider
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Update instance properties validation check",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   model.DefaultAuthenticator{},
		},
		args: args{
			rId:           map[string]string{"system.displayname": "test-cart-service"},
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
			client: test.fields.client,
			url:    test.fields.url,
			auth:   test.fields.auth,
		}
		err := e.UpdateInstanceProperties(test.args.rId, test.args.insProp, test.args.dsName, test.args.dsDisplayName, test.args.insName, test.args.patch)
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
		client *http.Client
		url    string
		auth   model.AuthProvider
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Update instance properties",
		fields: fields{
			client: ts.Client(),
			url:    ts.URL,
			auth:   model.DefaultAuthenticator{},
		},
		args: args{
			rId:           map[string]string{"system.displayname": "test-cart-service"},
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
			client: test.fields.client,
			url:    test.fields.url,
			auth:   test.fields.auth,
		}
		err := e.UpdateInstanceProperties(test.args.rId, test.args.insProp, test.args.dsName, test.args.dsDisplayName, test.args.insName, test.args.patch)
		if err == nil {
			t.Errorf("UpdateInstanceProperties() error expected but got error = nil")
			return
		}
	})

	cleanUp()
}

func setEnv() {
	os.Setenv("LM_ACCOUNT", "testenv")
	os.Setenv("LM_ACCESS_ID", "weryuifsjkf")
	os.Setenv("LM_ACCESS_KEY", "@dfsd4FDf999999FDE")
}

func cleanUp() {
	os.Unsetenv("LM_ACCOUNT")
	os.Unsetenv("LM_ACCESS_ID")
	os.Unsetenv("LM_ACCESS_KEY")
}
