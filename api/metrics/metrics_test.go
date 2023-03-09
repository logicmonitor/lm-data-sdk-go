package metrics

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
)

func TestNewLMMetricIngest(t *testing.T) {
	t.Run("should return Metric Ingest instance with default values", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		metricIngest, err := NewLMMetricIngest(ctx)
		assert.NoError(t, err)
		assert.Equal(t, true, metricIngest.batch.enabled)
		assert.Equal(t, defaultBatchingInterval, metricIngest.batch.interval)
		assert.Equal(t, true, metricIngest.gzip)
		assert.NotNil(t, metricIngest.client)
	})

	t.Run("should return LogIngest instance with options applied", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		metricIngest, err := NewLMMetricIngest(ctx, WithMetricBatchingInterval(5*time.Second))
		assert.NoError(t, err)
		assert.Equal(t, true, metricIngest.batch.enabled)
		assert.Equal(t, 5*time.Second, metricIngest.batch.interval)
	})
}

func TestSendMetrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := LMMetricIngestResponse{
			Success: true,
			Message: "Accepted",
		}
		w.WriteHeader(http.StatusAccepted)
		assert.NoError(t, json.NewEncoder(w).Encode(&response))
	}))

	defer ts.Close()

	t.Run("send metrics without batching", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		rateLimiter, _ := rateLimiter.NewLogRateLimiter(rateLimiter.LogRateLimiterSetting{RequestCount: 100})

		e := &LMMetricIngest{
			client:      ts.Client(),
			url:         ts.URL,
			auth:        utils.AuthParams{},
			rateLimiter: rateLimiter,
			batch:       &metricBatch{enabled: false},
		}

		resInput1, dsInput1, insInput1, dpInput1 := getTestInput()

		_, err := e.SendMetrics(context.Background(), resInput1, dsInput1, insInput1, dpInput1)
		assert.NoError(t, err)
	})

	t.Run("send logs with batching enabled", func(t *testing.T) {
		setLMEnv()
		defer cleanupLMEnv()

		rateLimiter, _ := rateLimiter.NewLogRateLimiter(rateLimiter.LogRateLimiterSetting{RequestCount: 100})
		e := &LMMetricIngest{
			client:      ts.Client(),
			url:         ts.URL,
			auth:        utils.AuthParams{},
			rateLimiter: rateLimiter,
			batch:       &metricBatch{enabled: true, interval: 1 * time.Second, lock: &sync.Mutex{}},
		}

		resInput1, dsInput1, insInput1, dpInput1 := getTestInput()

		_, err := e.SendMetrics(context.Background(), resInput1, dsInput1, insInput1, dpInput1)
		assert.NoError(t, err)
	})
}
func TestPushToBatch(t *testing.T) {
	t.Run("should add log message to batch", func(t *testing.T) {

		resInput, dsInput, insInput, dpInput := getTestInput()
		input := model.MetricsInput{
			Resource:   resInput,
			Datasource: dsInput,
			Instance:   insInput,
			DataPoint:  dpInput,
		}
		req, err := buildMetricRequest(context.Background(), input)
		assert.NoError(t, err)

		metricIngest := LMMetricIngest{batch: NewMetricBatch()}

		before := len(metricIngest.batch.data)

		metricIngest.batch.pushToBatch(req)

		after := len(metricIngest.batch.data)

		assert.Equal(t, before+1, after)
	})
}
func TestCombineBatchedMetricsRequests(t *testing.T) {
	t.Run("should merge the metrics requests", func(t *testing.T) {
		metricBatch := getTestMetricsBatch()
		combinedMetricsReq := metricBatch.combineBatchedMetricsRequests()
		assert.NotNil(t, combinedMetricsReq)
	})
}

func TestUpdateResourceProperties(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := LMMetricIngestResponse{
			Success: true,
			Message: "Resource properties updated successfully!!",
		}
		assert.NoError(t, json.NewEncoder(w).Encode(&response))
	}))

	type args struct {
		resName string
		rId     map[string]string
		resProp map[string]string
		patch   bool
	}

	type fields struct {
		client      *http.Client
		url         string
		auth        utils.AuthParams
		rateLimiter *rateLimiter.MetricsRateLimiter
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
			auth:   utils.AuthParams{},
		},
		args: args{
			resName: "TestResource",
			rId:     map[string]string{"system.displayname": "test-cart-service"},
			resProp: map[string]string{"new": "updatedprop"},
			patch:   false,
		},
	}

	t.Run(test.name, func(t *testing.T) {
		setLMEnv()
		rateLimiter, _ := rateLimiter.NewMetricsRateLimiter(rateLimiter.MetricsRateLimiterSetting{RequestCount: 100})
		e := &LMMetricIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			rateLimiter: rateLimiter,
		}
		_, err := e.UpdateResourceProperties(test.args.resName, test.args.rId, test.args.resProp, test.args.patch)
		if err != nil {
			t.Errorf("UpdateResourceProperties() error = %v", err)
			return
		}
	})
	cleanupLMEnv()
}

func TestUpdateResourcePropertiesValidation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := LMMetricIngestResponse{
			Success: true,
			Message: "Resource properties updated successfully!!",
		}
		assert.NoError(t, json.NewEncoder(w).Encode(&response))
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
		auth   utils.AuthParams
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
			auth:   utils.AuthParams{},
		},
		args: args{
			resName: "Test",
			rId:     map[string]string{"system.displayname": "test-cart-service"},
			resProp: map[string]string{"new": ""},
			patch:   false,
		},
	}

	t.Run(test.name, func(t *testing.T) {
		setLMEnv()
		rateLimiter, _ := rateLimiter.NewMetricsRateLimiter(rateLimiter.MetricsRateLimiterSetting{RequestCount: 100})
		e := &LMMetricIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			rateLimiter: rateLimiter,
		}
		_, err := e.UpdateResourceProperties(test.args.resName, test.args.rId, test.args.resProp, test.args.patch)
		if err == nil {
			t.Errorf("UpdateResourceProperties() expect error, but got error = nil")
			return
		}
	})
	cleanupLMEnv()
}

func TestUpdateInstanceProperties(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := LMMetricIngestResponse{
			Success: true,
			Message: "Instance properties updated successfully!!",
		}
		assert.NoError(t, json.NewEncoder(w).Encode(&response))
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
		auth   utils.AuthParams
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
			auth:   utils.AuthParams{},
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
		setLMEnv()
		rateLimiter, _ := rateLimiter.NewMetricsRateLimiter(rateLimiter.MetricsRateLimiterSetting{RequestCount: 100})
		e := &LMMetricIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			rateLimiter: rateLimiter,
		}
		_, err := e.UpdateInstanceProperties(test.args.rId, test.args.insProp, test.args.dsName, test.args.dsDisplayName, test.args.insName, test.args.patch)
		if err != nil {
			t.Errorf("UpdateInstanceProperties() error = %v", err)
			return
		}
	})

	cleanupLMEnv()
}

func TestUpdateInstancePropertiesValidation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := LMMetricIngestResponse{
			Success: true,
			Message: "Instance properties updated successfully!!",
		}
		assert.NoError(t, json.NewEncoder(w).Encode(&response))
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
		auth   utils.AuthParams
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
			auth:   utils.AuthParams{},
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
		setLMEnv()
		rateLimiter, _ := rateLimiter.NewMetricsRateLimiter(rateLimiter.MetricsRateLimiterSetting{RequestCount: 100})

		e := &LMMetricIngest{
			client:      test.fields.client,
			url:         test.fields.url,
			auth:        test.fields.auth,
			rateLimiter: rateLimiter,
		}
		_, err := e.UpdateInstanceProperties(test.args.rId, test.args.insProp, test.args.dsName, test.args.dsDisplayName, test.args.insName, test.args.patch)
		if err == nil {
			t.Errorf("UpdateInstanceProperties() expect error  but got error = nil")
			return
		}
	})

	cleanupLMEnv()
}

func getTestInput() (model.ResourceInput, model.DatasourceInput, model.InstanceInput, model.DataPointInput) {
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

func getTestInputResCreate() (model.ResourceInput, model.DatasourceInput, model.InstanceInput, model.DataPointInput) {
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

func getTestMetricsBatch() *metricBatch {
	metricBatch := NewMetricBatch()

	rInput1 := model.ResourceInput{
		ResourceName: "test-cart-service",
		ResourceID:   map[string]string{"system.displayname": "test-cart-service"},
	}

	dsInput1 := model.DatasourceInput{
		DataSourceName:        "GoSDK",
		DataSourceDisplayName: "GoSDK",
		DataSourceGroup:       "Sdk",
	}

	insInput1 := model.InstanceInput{
		InstanceName:       "TelemetrySDK",
		InstanceProperties: map[string]string{"test": "telemetrysdk"},
	}

	dpInput1 := model.DataPointInput{
		DataPointName:            "cpu",
		DataPointType:            "GAUGE",
		DataPointAggregationType: "SUM",
		Value:                    map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "124"},
	}

	metricInput1 := model.MetricsInput{
		Resource:   rInput1,
		Datasource: dsInput1,
		Instance:   insInput1,
		DataPoint:  dpInput1,
	}

	req1, _ := buildMetricRequest(context.Background(), metricInput1)

	metricBatch.pushToBatch(req1)

	rInput2 := model.ResourceInput{
		ResourceName: "test-payment-service",
		ResourceID:   map[string]string{"system.displayname": "test-cart-service"},
		IsCreate:     true,
	}

	dsInput2 := model.DatasourceInput{
		DataSourceName:        "GoSDK",
		DataSourceDisplayName: "GoSDK",
		DataSourceGroup:       "Sdk",
	}

	insInput2 := model.InstanceInput{
		InstanceName:       "DataSDK",
		InstanceProperties: map[string]string{"test": "datasdk"},
	}

	dpInput2 := model.DataPointInput{
		DataPointName:            "memory",
		DataPointType:            "COUNTER",
		DataPointAggregationType: "SUM",
		Value:                    map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "124"},
	}

	metricInput2 := model.MetricsInput{
		Resource:   rInput2,
		Datasource: dsInput2,
		Instance:   insInput2,
		DataPoint:  dpInput2,
	}

	req2, _ := buildMetricRequest(context.Background(), metricInput2)

	metricBatch.pushToBatch(req2)

	return metricBatch
}

func TestHandleMetricsExportResponse(t *testing.T) {
	t.Run("should handle success response", func(t *testing.T) {
		ingestResponse, err := handleMetricsExportResponse(context.Background(), &http.Response{
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
				"error": "Resource not found"
			  }
			]
		  }`)
		ingestResponse, err := handleMetricsExportResponse(context.Background(), &http.Response{
			StatusCode:    http.StatusMultiStatus,
			ContentLength: int64(len(data)),
			Request:       httptest.NewRequest(http.MethodPost, "https://example.logicmonitor.com"+uri, nil),
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
			Error:   fmt.Errorf("error exporting items, request to https://example.logicmonitor.com/v2/metric/ingest responded with HTTP Status Code 207, Message: Some events were not accepted. See the 'errors' property for additional information., Details: error code: [4001], error message: Resource not found"),
			Message: "Some events were not accepted. See the 'errors' property for additional information.",
		}, *ingestResponse)
	})

	t.Run("should handle non multi-status response", func(t *testing.T) {
		data := []byte(`{
			"success": false,
			"message": "Too Many Requests"
		  }`)
		ingestResponse, err := handleMetricsExportResponse(context.Background(), &http.Response{
			StatusCode:    http.StatusTooManyRequests,
			ContentLength: int64(len(data)),
			Request:       httptest.NewRequest(http.MethodPost, "https://example.logicmonitor.com"+uri, nil),
			Body:          ioutil.NopCloser(bytes.NewReader(data)),
		})
		assert.NoError(t, err)
		assert.Equal(t, model.IngestResponse{
			Success:    false,
			StatusCode: http.StatusTooManyRequests,
			Error:      fmt.Errorf("error exporting items, request to https://example.logicmonitor.com%s responded with HTTP Status Code 429, Message: Too Many Requests, Details: Too Many Requests", uri),
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
