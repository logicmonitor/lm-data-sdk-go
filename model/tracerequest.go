package model

import "go.opentelemetry.io/collector/pdata/ptrace"

type TracesRequest struct {
	TraceData ptrace.Traces
	SpanCount int
}
