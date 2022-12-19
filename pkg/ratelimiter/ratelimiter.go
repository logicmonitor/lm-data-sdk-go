package ratelimiter

import (
	"context"
)

// RateLimiter represents the RateLimiter operations
type RateLimiter interface {
	IncRequestCount()
	Acquire() (bool, error)
	ResetRequestCount()
	Run(context.Context)
	Shutdown(context.Context)
}

type SpanRateLimiter interface {
	SetRequestSpanCount(spanCount int)
	IncSpanCount()
	IncSpanPerRequestCount()
	ResetSpanCount()
	ResetSpanPerRequestCount()
}

// RateLimiterSetting represents the RateLimiter config
type RateLimiterSetting struct {
	RequestCount        int
	SpanCount           int
	SpanCountPerRequest int
}
