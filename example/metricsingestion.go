package main

import (
	"context"
	"fmt"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/api/metrics"
	"github.com/logicmonitor/lm-data-sdk-go/model"
)

func main_metrics() {
	var options []metrics.Option
	options = []metrics.Option{
		metrics.WithMetricBatchingEnabled(3 * time.Second),
	}

	lmMetric, err := metrics.NewLMMetricIngest(context.Background(), options...)
	if err != nil {
		fmt.Println("Error in initializing metric ingest :", err)
		return
	}

	rInput, dsInput, insInput, dpInput := createInput1()
	_, err = lmMetric.SendMetrics(context.Background(), rInput, dsInput, insInput, dpInput)
	if err != nil {
		fmt.Println("Error in sending 1st metric: ", err)
	}
	time.Sleep(3 * time.Second)

	rInput1, dsInput1, insInput1, dpInput1 := createInput2()
	_, err = lmMetric.SendMetrics(context.Background(), rInput1, dsInput1, insInput1, dpInput1)
	if err != nil {
		fmt.Println("Error in sending 2nd metric: ", err)
	}
	time.Sleep(5 * time.Second)

	rInput2, dsInput2, insInput2, dpInput2 := createInput3()
	_, err = lmMetric.SendMetrics(context.Background(), rInput2, dsInput2, insInput2, dpInput2)
	if err != nil {
		fmt.Println("Error in sending 3rd metric: ", err)
	}

	resName := "TestLogexporterSDK_OTEL_30033"
	resProp := map[string]string{"new": "updatedprop"}
	rId := map[string]string{"system.displayname": "TestLogexporterSDK_OTEL_30033"}
	insProp := map[string]string{"new": "updatedprop"}
	dsName := "TestDS"
	dsDisplayName := "TestDisplayName"
	insName := "DataSDK"
	patch := true

	_, err = lmMetric.UpdateInstanceProperties(rId, insProp, dsName, dsDisplayName, insName, patch)
	if err != nil {
		fmt.Println("Error in updating instance properties: ", err)
	}

	_, err = lmMetric.UpdateResourceProperties(resName, rId, resProp, patch)
	if err != nil {
		fmt.Println("Error in updating resource properties: ", err)
	}
}

func createInput1() (model.ResourceInput, model.DatasourceInput, model.InstanceInput, model.DataPointInput) {
	// fill the values
	rInput := model.ResourceInput{
		ResourceName: "demo_OTEL_71086",
		ResourceID:   map[string]string{"system.displayname": "TestLogexporterSDK_OTEL_30033"},
	}

	dsInput := model.DatasourceInput{
		DataSourceName:        "GoSDK",
		DataSourceDisplayName: "GoSDK",
		DataSourceGroup:       "Sdk",
	}

	insInput := model.InstanceInput{
		InstanceName:       "DataSDK",
		InstanceProperties: map[string]string{"test": "datasdk"},
	}

	dpInput := model.DataPointInput{
		DataPointName:            "cpu",
		DataPointType:            "COUNTER",
		DataPointAggregationType: "SUM",
		Value:                    map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "124"},
	}
	return rInput, dsInput, insInput, dpInput
}

func createInput2() (model.ResourceInput, model.DatasourceInput, model.InstanceInput, model.DataPointInput) {
	// fill the values
	rInput := model.ResourceInput{
		ResourceName: "demo_OTEL_71086",
		ResourceID:   map[string]string{"system.displayname": "TestLogexporterSDK_OTEL_30033"},
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
	return rInput, dsInput, insInput, dpInput
}

func createInput3() (model.ResourceInput, model.DatasourceInput, model.InstanceInput, model.DataPointInput) {
	// fill the values
	rInput := model.ResourceInput{
		ResourceName: "demo_OTEL_71086",
		ResourceID:   map[string]string{"system.displayname": "TestLogexporterSDK_OTEL_30033"},
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
		DataPointName:            "memory",
		DataPointType:            "GAUGE",
		DataPointAggregationType: "SUM",
		Value:                    map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "14"},
	}
	return rInput, dsInput, insInput, dpInput
}
