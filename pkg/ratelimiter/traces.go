package ratelimiter

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

const (
	defaultRateLimitTraces = 1000
)

// TraceRateLimiter represents the RateLimiter config for traces
type TraceRateLimiter struct {
	traceRequestCount uint64
	maxCount          uint64
	ticker            *time.Ticker
	shutdownCh        chan struct{}
}

// NewTraceRateLimiter creates RateLimiter implementation for traces using RateLimiterSetting
func NewTraceRateLimiter(setting RateLimiterSetting) (*TraceRateLimiter, error) {
	if setting.RequestCount == 0 {
		setting.RequestCount = defaultRateLimitTraces
	}
	return &TraceRateLimiter{
		traceRequestCount: 0,
		maxCount:          uint64(setting.RequestCount),
		ticker:            time.NewTicker(time.Duration(1 * time.Minute)),
		shutdownCh:        make(chan struct{}, 1),
	}, nil
}

// IncRequestCount increaments the request count associated with traces by 1
func (rateLimiter *TraceRateLimiter) IncRequestCount() {
	atomic.AddUint64(&rateLimiter.traceRequestCount, 1)
}

// ResetRequestCount resets the request count associated with traces to 0
func (rateLimiter *TraceRateLimiter) ResetRequestCount() {
	atomic.StoreUint64(&rateLimiter.traceRequestCount, 0)
}

// Acquire checks if the requests count for traces is reached to maximum allocated quota per minute.
func (rateLimiter *TraceRateLimiter) Acquire() (bool, error) {
	for {
		select {
		case <-rateLimiter.shutdownCh:
			return false, fmt.Errorf("shutdown is called")
		default:
			if rateLimiter.traceRequestCount < rateLimiter.maxCount {
				rateLimiter.IncRequestCount()
				return true, nil
			}
			return false, fmt.Errorf("request quota of (%d) requests per min for the traces is exhausted for the interval", rateLimiter.maxCount)
		}
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
		}
	}
}

// Shutdown triggers the shutdown of the LogRateLimiter
func (rateLimiter *TraceRateLimiter) Shutdown(_ context.Context) {
	rateLimiter.shutdownCh <- struct{}{}
}
