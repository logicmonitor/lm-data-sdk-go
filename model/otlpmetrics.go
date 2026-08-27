package model

import "go.opentelemetry.io/collector/pdata/pmetric"

type OTLPMetricsPayload struct {
	MetricData pmetric.Metrics
}
