package ratelimiter

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

const (
	defaultRequestsPerMinuteLimit = 2000
	defaultSpansPerRequestLimit   = 3000
	defaultSpansPerMinuteLimit    = 139000
)

// TraceRateLimiter represents the RateLimiter config for traces
type TraceRateLimiter struct {
	inputSpanCount         uint64
	spanCount              uint64
	requestCount           uint64
	spanCountPerRequest    uint64
	maxRequestCount        uint64
	maxSpanCount           uint64
	maxSpanCountPerRequest uint64
	ticker                 *time.Ticker
	shutdownCh             chan struct{}
}

// NewTraceRateLimiter creates RateLimiter implementation for traces using RateLimiterSetting
func NewTraceRateLimiter(setting RateLimiterSetting) (*TraceRateLimiter, error) {
	if setting.RequestCount == 0 {
		setting.RequestCount = defaultRequestsPerMinuteLimit
	}
	if setting.SpanCount == 0 {
		setting.SpanCount = defaultSpansPerMinuteLimit
	}
	if setting.SpanCountPerRequest == 0 {
		setting.SpanCountPerRequest = defaultSpansPerRequestLimit
	}
	return &TraceRateLimiter{
		requestCount:           0,
		spanCount:              0,
		spanCountPerRequest:    0,
		maxRequestCount:        uint64(setting.RequestCount),
		maxSpanCount:           uint64(setting.SpanCount),
		maxSpanCountPerRequest: uint64(setting.SpanCountPerRequest),
		ticker:                 time.NewTicker(time.Duration(1 * time.Minute)),
		shutdownCh:             make(chan struct{}, 1),
	}, nil
}

// IncRequestCount increments the request count associated with traces by 1
func (rateLimiter *TraceRateLimiter) IncRequestCount() {
	atomic.AddUint64(&rateLimiter.requestCount, 1)
}

// IncSpanCount increments the span count associated with traces by no. of spans
func (rateLimiter *TraceRateLimiter) IncSpanCount() {
	atomic.AddUint64(&rateLimiter.spanCount, rateLimiter.inputSpanCount)
}

// IncSpanPerRequestCount increments the span count associated with traces by no of spans
func (rateLimiter *TraceRateLimiter) IncSpanPerRequestCount() {
	atomic.AddUint64(&rateLimiter.spanCountPerRequest, rateLimiter.inputSpanCount)
}

// ResetRequestCount resets the request count associated with traces to 0
func (rateLimiter *TraceRateLimiter) ResetRequestCount() {
	atomic.StoreUint64(&rateLimiter.requestCount, 0)
}

// ResetSpanCount resets the request count associated with traces to 0
func (rateLimiter *TraceRateLimiter) ResetSpanCount() {
	atomic.StoreUint64(&rateLimiter.spanCount, 0)
}

// ResetSpanPerRequestCount resets the request count associated with traces to 0
func (rateLimiter *TraceRateLimiter) ResetSpanPerRequestCount() {
	atomic.StoreUint64(&rateLimiter.spanCountPerRequest, 0)
}

// ResetSpanPerRequestCount resets the request count associated with traces to 0
func (rateLimiter *TraceRateLimiter) SetRequestSpanCount(count int) {
	atomic.StoreUint64(&rateLimiter.inputSpanCount, uint64(count))
}

// 139000 spans per minute
// 2000 requests per minute
// 3000 spans per request
// Acquire checks if the requests count for traces is reached to maximum allocated quota per minute.
func (rateLimiter *TraceRateLimiter) Acquire() (bool, error) {
	select {
	case <-rateLimiter.shutdownCh:
		return false, fmt.Errorf("shutdown is called")
	default:
		if rateLimiter.requestCount < rateLimiter.maxRequestCount {
			rateLimiter.IncRequestCount()
			return true, nil
		}
		if rateLimiter.spanCount < rateLimiter.maxSpanCount {
			rateLimiter.IncSpanCount()
			return true, nil
		}
		if rateLimiter.spanCountPerRequest < rateLimiter.maxSpanCountPerRequest {
			rateLimiter.IncSpanPerRequestCount()
			return true, nil
		}
		return false, fmt.Errorf("request quota of requests per min for the traces is exhausted for the interval")
	}
}

// Run starts the timer for reseting the traces request counter
func (rateLimiter *TraceRateLimiter) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			rateLimiter.Shutdown(ctx)
			return
		case <-rateLimiter.ticker.C:
			rateLimiter.ResetRequestCount()
			rateLimiter.ResetSpanCount()
		}
	}
}

// Shutdown triggers the shutdown of the LogRateLimiter
func (rateLimiter *TraceRateLimiter) Shutdown(_ context.Context) {
	rateLimiter.shutdownCh <- struct{}{}
}
