# LogicMonitor Go Data SDK

LogicMonitor Go Data SDK is suitable for ingesting the metrics and logs into the LogicMonitor Platform.

## Overview
LogicMonitor's Push Metrics feature allows you to send metrics directly to the LogicMonitor platform via a dedicated API, removing the need to route the data through a LogicMonitor Collector. Once ingested, these metrics are presented alongside all other metrics gathered via LogicMonitor, providing a single pane of glass for metric monitoring and alerting.

Similarly, If a log integration isn’t available or you have custom logs that you want to analyze, you can send the logs directly to your LogicMonitor account via the logs ingestion API.

## Getting Started

### Authentication
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

### Metrics Ingestion
Here is an [example](https://github.com/logicmonitor/lm-data-sdk-go/blob/main/example/metrics/metricsingestion.go) for metrics ingestion.


#### Options

Following options can be used to create the metrics api client.

|   Option  |	Description | Default |
| -------------------- |:----------------------------------:|:--------------:| 
| `WithMetricBatchingInterval(batchinterval time.Duration)`  | Sets time interval to wait before performing next batching of metrics. | `10s` |
| `WithMetricBatchingDisabled()` | Disables batching of metrics. | `Enabled` |
| `WithGzipCompression(gzip bool)` | Enables / disables gzip compression of metric payload. | `Enabled` |
| `WithRateLimit(requestCount int)` | Sets limit on the number of requests to metrics API per minute. | `100` |
| `WithHTTPClient(client *http.Client)` | Sets custom HTTP Client | `Default http client with timeout of 5s`|
| `WithEndpoint(endpoint string)` | Sets endpoint to send the metrics to. | `https://${LOGICMONITOR_ACCOUNT}.logicmonitor.com/rest/`|
| `WithAuthentication(authParams utils.AuthParams)`         | Sets authentication parameters | `-` |

### Logs Ingestion
Here is an [example](https://github.com/logicmonitor/lm-data-sdk-go/blob/main/example/logs/logsingestion.go) for logs ingestion.


#### Options

Following options can be used to create the logs api client.


|   Option  |	Description | Default |
| -------------------- |:---------------------------------------------:|:--------------:|  
| `WithLogBatchingInterval(batchinterval time.Duration)`  | Sets time interval to wait before performing next batching of logs. | `10s` |
| `WithLogBatchingDisabled()` | Disables batching of logs. | `Enabled` |
| `WithGzipCompression(gzip bool)` | Enables / disables gzip compression of metric payload. | `Enabled` |
| `WithRateLimit(requestCount int)` | Sets limit on the number of requests to metrics API per minute. | `100` |
| `WithHTTPClient(client *http.Client)` | Sets custom HTTP Client | `Default http client will have timeout of 5s`|
| `WithEndpoint(endpoint string)` | Sets endpoint to send the metrics to. | `https://${LOGICMONITOR_ACCOUNT}.logicmonitor.com/rest/`|
| `WithResourceMappingOperation(op string)` | Sets resource mapping operation. Valid operations are `AND` & `OR` | `-` |
| `WithAuthentication(authParams utils.AuthParams)`         | Sets authentication parameters | `-` |




Copyright, 2023, LogicMonitor, Inc.

This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
