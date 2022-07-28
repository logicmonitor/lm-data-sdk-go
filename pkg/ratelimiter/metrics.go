package ratelimiter

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

const (
	defaultRateLimitMetrics = 100
)

// MetricsRateLimiter represents the RateLimiter config for metrics
type MetricsRateLimiter struct {
	metricRequestCount uint64
	maxCount           uint64
	ticker             *time.Ticker
	shutdownCh         chan struct{}
}

// NewMetricsRateLimiter creates RateLimiter implementation for metrics using RateLimiterSetting
func NewMetricsRateLimiter(setting RateLimiterSetting) (*MetricsRateLimiter, error) {
	if setting.RequestCount == 0 {
		setting.RequestCount = defaultRateLimitMetrics
	}
	return &MetricsRateLimiter{
		metricRequestCount: 0,
		maxCount:           uint64(setting.RequestCount),
		ticker:             time.NewTicker(time.Duration(1 * time.Minute)),
		shutdownCh:         make(chan struct{}, 1),
	}, nil
}

// IncRequestCount increaments the request count associated with metrics by 1
func (rateLimiter *MetricsRateLimiter) IncRequestCount() {
	atomic.AddUint64(&rateLimiter.metricRequestCount, 1)
}

// ResetRequestCount resets the request count associated with metrics to 0
func (rateLimiter *MetricsRateLimiter) ResetRequestCount() {
	atomic.StoreUint64(&rateLimiter.metricRequestCount, 0)
}

// Acquire checks if the requests count for metrics is reached to maximum allocated quota per minute.
func (rateLimiter *MetricsRateLimiter) Acquire() (bool, error) {
	for {
		select {
		case <-rateLimiter.shutdownCh:
			return false, fmt.Errorf("shutdown is called")
		default:
			if rateLimiter.metricRequestCount < rateLimiter.maxCount {
				rateLimiter.IncRequestCount()
				return true, nil
			}
			return false, fmt.Errorf("request quota of (%d) requests per min for the metrics is exhausted for the interval", rateLimiter.maxCount)
		}
	}
}

// Run starts the timer for reseting the metrics request counter
func (rateLimiter *MetricsRateLimiter) Run(ctx context.Context) {
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

// Shutdown triggers the shutdown of the MetricsRateLimiter
func (rateLimiter *MetricsRateLimiter) Shutdown(_ context.Context) {
	rateLimiter.shutdownCh <- struct{}{}
}
