# LogicMonitor Golang Data SDK

LogicMonitor Golang Data SDK is suitable for ingesting the metrics and logs into the LogicMonitor Platform.

## Overview
LogicMonitor's Push Metrics feature allows you to send metrics directly to the LogicMonitor platform via a dedicated API, removing the need to route the data through a LogicMonitor Collector. Once ingested, these metrics are presented alongside all other metrics gathered via LogicMonitor, providing a single pane of glass for metric monitoring and alerting.

Similarly, If a log integration isn’t available or you have custom logs that you want to analyze, you can send the logs directly to your LogicMonitor account via the logs ingestion API.

## Quick Start Notes:

### Set Configurations
While using LMv1 authentication set LOGICMONITOR_ACCESS_ID and LOGICMONITOR_ACCESS_KEY properties.
In case of BearerToken authentication set LOGICMONITOR_BEARER_TOKEN property. 
Company's name or Account name must be passed to LOGICMONITOR_ACCOUNT property. 
All properties can be set using environment variable.

| Environment variable |	Description |
| -------------------- |:--------------:|
|   LOGICMONITOR_ACCOUNT         |	Account name (Company Name) is your organization name |
|   LOGICMONITOR_ACCESS_ID       |	Access id while using LMv1 authentication.|
|   LOGICMONITOR_ACCESS_KEY      |	Access key while using LMv1 authentication.|
|   LOGICMONITOR_BEARER_TOKEN    |	BearerToken while using Bearer authentication.|

#### Default values

- Batching is enabled with default time interval as 10s.
- Gzip compression of payload is enabled.
- URL will be set to `https://${LOGICMONITOR_ACCOUNT}.logicmonitor.com/rest` followed by uri path (for logs: /log/ingest and for metrics: /v2/metric/ingest)
- Default http client will have timeout of 5s.

### Metrics Ingestion:
For metrics ingestion, user must create an object of `ResourceInput`, `DataSourceInput`, `InstanceInput` and `DataPointInput` by passing relevant metric and attribute values to the struct fields using package `github.com/logicmonitor/lm-data-sdk-go/model`.

Initialize metric ingest by calling `NewLMMetricIngest`. 

#### Options:

- To change batching interval of metrics, user can pass an option `WithMetricBatchingInterval(batchinterval string)`.

- To disable batching of metrics, user can pass an option `WithMetricBatchingDisabled()`.

- To pass authentication parameters, user can pass an option `WithAuthentication(authParams utils.AuthParams)`.

- To enable/disable gzip compression of metric payload, user can pass an option `WithGzipCompression(gzip bool)`.

- Number of requests for metrics per minute can be controlled using `WithRateLimit(requestCount int)`.

- To change settings of default HTTP client, user can pass an option `WithHTTPClient(client *http.Client)`

- Endpoint URL to which metrics will be exported can be changed by `WithEndpoint(endpoint string)` 

#### Example:

For exporting metrics to LM platform, call SendMetrics by passing all the attributes as input parameters.

    // fill the values
	rInput := model.ResourceInput{
		ResourceName: "example-cart-service",
		ResourceID: map[string]string{"system.displayname": "example-cart-service"},
	}

	dsInput := model.DatasourceInput{
		DataSourceName:        "TestGoSDK",
		DataSourceDisplayName: "TestGoSDK",
		DataSourceGroup:       "TestSDK",
	}

	insInput := model.InstanceInput{
		InstanceName:       "DataSDK",
		InstanceProperties: map[string]string{"test": "datasdk"},
	}

	dpInput := model.DataPointInput{
		DataPointName:            "cpuusage",
		DataPointType:            "COUNTER",
		DataPointAggregationType: "SUM",
		Value:                    map[string]string{fmt.Sprintf("%d", time.Now().Unix()): "124"},
	}

	options := []metrics.Option{
		metrics.WithMetricBatchingInterval(3 * time.Second),
	}

	lmMetric, err := metrics.NewLMMetricIngest(context.Background(), options...)
	if err != nil {
		fmt.Println("Error in initializing metric ingest :", err)
		return
	}
	lmMetric.SendMetrics(context.Background(), rInput, dsInput, insInput, dpInput)

### Logs Ingestion

Initialize log ingest by calling `NewLMLogIngest`. 

#### Options:

- To change batching interval of logs, user can pass an option `WithLogBatchingInterval(batchinterval string)`

- To disable batching of logs, user can pass an option `WithLogBatchingDisabled()`

- To pass authentication parameters, user can pass an option `WithAuthentication(authParams utils.AuthParams)`

- To enable/disable gzip compression of log payload, user can pass an option `WithGzipCompression(gzip bool)`

- Number of requests for logs per minute can be controlled using `WithRateLimit(requestCount int)`

- To change settings of default HTTP client, user can pass an option `WithHTTPClient(client *http.Client)`

- Endpoint URL to which logs will be exported, can be changed by `WithEndpoint(endpoint string)` 

#### Example:

For exporting logs to LM platform, call SendLogs by passing `log message`, `resource ID` and `metadata map` as input parameters.

```
var options []logs.Option
options = []logs.Option{
	logs.WithLogBatchingInterval(3 * time.Second),
}

lmLog, err := logs.NewLMLogIngest(context.Background(), options...)
if err != nil {
	fmt.Println("Error in initilaizing log ingest ", err)
	return
}
lmLog.SendLogs(logstr, map[string]string{"system.displayname": "device-name-test"}, map[string]string{"testkey": "testvalue"})
```

Read the Library Documentation to use Metrics/Logs ingestion API.


Copyright, 2022, LogicMonitor, Inc.

This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.