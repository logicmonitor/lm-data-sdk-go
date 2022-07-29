package ratelimiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMetricsRateLimiter(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	metricsRateLimiter, err := NewMetricsRateLimiter(setting)
	assert.NoError(t, err)
	assert.Equal(t, uint64(setting.RequestCount), metricsRateLimiter.maxCount)
	assert.Equal(t, uint64(0), metricsRateLimiter.metricRequestCount)
}

func TestNewMetricsRateLimiterDefaultRequestCount(t *testing.T) {
	setting := RateLimiterSetting{}
	metricsRateLimiter, err := NewMetricsRateLimiter(setting)
	assert.NoError(t, err)
	assert.Equal(t, uint64(defaultRateLimitLogs), metricsRateLimiter.maxCount)
	assert.Equal(t, uint64(0), metricsRateLimiter.metricRequestCount)
}

func TestIncRequestCount_Metrics(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	metricsRateLimiter, err := NewMetricsRateLimiter(setting)
	assert.NoError(t, err)
	metricsRequestCountBeforeInc := metricsRateLimiter.metricRequestCount
	metricsRateLimiter.IncRequestCount()
	metricsRequestCountAfterInc := metricsRateLimiter.metricRequestCount
	assert.Equal(t, metricsRequestCountBeforeInc+1, metricsRequestCountAfterInc)
}

func TestResetRequestCount_Metrics(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	metricsRateLimiter, err := NewMetricsRateLimiter(setting)
	assert.NoError(t, err)
	metricsRateLimiter.ResetRequestCount()
	assert.Equal(t, uint64(0), metricsRateLimiter.metricRequestCount)
}

func TestAcquire_Metrics(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 1,
	}
	metricsRateLimiter, err := NewMetricsRateLimiter(setting)
	assert.NoError(t, err)

	// Should allow 1 request
	ok, err := metricsRateLimiter.Acquire()
	assert.Equal(t, true, ok)
	assert.NoError(t, err)

	// Should drop the second request as quota is exhausted
	ok, err = metricsRateLimiter.Acquire()
	assert.Equal(t, false, ok)
	assert.Error(t, err)
}

func TestAcquireAfterShutdown_Metrics(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	metricsRateLimiter, err := NewMetricsRateLimiter(setting)
	assert.NoError(t, err)
	metricsRateLimiter.Shutdown(context.Background())
	ok, err := metricsRateLimiter.Acquire()
	assert.Equal(t, false, ok)
	assert.Error(t, err)
}

func TestRun_Metrics(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	metricsRateLimiter, err := NewMetricsRateLimiter(setting)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	metricsRateLimiter.ticker.Reset(3 * time.Second)
	go metricsRateLimiter.Run(ctx)
	metricsRateLimiter.IncRequestCount()
	time.Sleep(5 * time.Second)
	assert.Equal(t, uint64(0), metricsRateLimiter.metricRequestCount)
	metricsRateLimiter.Shutdown(ctx)
}
