package ratelimiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewLogRateLimiter(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	logRateLimiter, err := NewLogRateLimiter(setting)
	assert.NoError(t, err)
	assert.Equal(t, uint64(setting.RequestCount), logRateLimiter.maxCount)
	assert.Equal(t, uint64(0), logRateLimiter.logRequestCount)
}

func TestNewLogRateLimiterDefaultRequestCount(t *testing.T) {
	setting := RateLimiterSetting{}
	logRateLimiter, err := NewLogRateLimiter(setting)
	assert.NoError(t, err)
	assert.Equal(t, uint64(defaultRateLimitLogs), logRateLimiter.maxCount)
	assert.Equal(t, uint64(0), logRateLimiter.logRequestCount)
}

func TestIncRequestCount_Logs(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	logRateLimiter, err := NewLogRateLimiter(setting)
	assert.NoError(t, err)
	logRequestCountBeforeInc := logRateLimiter.logRequestCount
	logRateLimiter.IncRequestCount()
	logRequestCountAfterInc := logRateLimiter.logRequestCount
	assert.Equal(t, logRequestCountBeforeInc+1, logRequestCountAfterInc)
}

func TestResetRequestCount_Logs(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	logRateLimiter, err := NewLogRateLimiter(setting)
	assert.NoError(t, err)
	logRateLimiter.ResetRequestCount()
	assert.Equal(t, uint64(0), logRateLimiter.logRequestCount)
}

func TestAcquire_Logs(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 1,
	}
	logRateLimiter, err := NewLogRateLimiter(setting)
	assert.NoError(t, err)

	// Should allow 1 request
	ok, err := logRateLimiter.Acquire()
	assert.Equal(t, true, ok)
	assert.NoError(t, err)

	// Should drop the second request as quota is exhausted
	ok, err = logRateLimiter.Acquire()
	assert.Equal(t, false, ok)
	assert.Error(t, err)
}

func TestAcquireAfterShutdown_Logs(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	logRateLimiter, err := NewLogRateLimiter(setting)
	assert.NoError(t, err)
	logRateLimiter.Shutdown(context.Background())
	ok, err := logRateLimiter.Acquire()
	assert.Equal(t, false, ok)
	assert.Error(t, err)
}

func TestRun_Logs(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	logRateLimiter, err := NewLogRateLimiter(setting)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	logRateLimiter.ticker.Reset(3 * time.Second)
	go logRateLimiter.Run(ctx)
	logRateLimiter.IncRequestCount()
	time.Sleep(5 * time.Second)
	assert.Equal(t, uint64(0), logRateLimiter.logRequestCount)
	logRateLimiter.Shutdown(ctx)
}
