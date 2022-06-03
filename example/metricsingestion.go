package main

import (
	"fmt"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/api/metrics"
	"github.com/logicmonitor/lm-data-sdk-go/model"
)

func main_metrics() {

	// fill the values
	rInput := model.ResourceInput{
		ResourceName: "demo_OTEL_71086",
		//ResourceDescription: "Testing",
		ResourceID: map[string]string{"system.displayname": "demo_OTEL_71086"},
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

	lmMetric := metrics.NewLMMetricIngest(false, 10)
	//lmMetric.Start()
	lmMetric.SendMetrics(rInput, dsInput, insInput, dpInput)

	time.Sleep(3 * time.Second)

	// fill the values
	rInput1 := model.ResourceInput{
		ResourceName: "demo_OTEL_71086",
		//ResourceDescription: "Testing",
		ResourceID: map[string]string{"system.displayname": "demo_OTEL_71086"},
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
	fmt.Println("Sending new metrics....")

	lmMetric.SendMetrics(rInput1, dsInput1, insInput1, dpInput1)

	time.Sleep(5 * time.Second)

	// fill the values
	rInput2 := model.ResourceInput{
		ResourceName: "demo_OTEL_71086",
		//ResourceDescription: "Testing",
		ResourceID: map[string]string{"system.displayname": "demo_OTEL_71086"},
	}

	dsInput2 := model.DatasourceInput{
		DataSourceName:        "GoSDK",
		DataSourceDisplayName: "GoSDK",
		DataSourceGroup:       "Sdk",
	}

	insInput2 := model.InstanceInput{
		InstanceName:       "TelemetrySDK",
		InstanceProperties: map[string]string{"test": "telemetrysdk"},
	}

	dpInput2 := model.DataPointInput{
		DataPointName:            "memory",
		DataPointType:            "GAUGE",
		DataPointAggregationType: "SUM",
		Value:                    map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "14"},
	}
	fmt.Println("Sending new metrics .......")

	lmMetric.SendMetrics(rInput2, dsInput2, insInput2, dpInput2)
}
