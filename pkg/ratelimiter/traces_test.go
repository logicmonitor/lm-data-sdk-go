package ratelimiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTraceRateLimiter(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount:        100,
		SpanCount:           100,
		SpanCountPerRequest: 100,
	}
	traceRateLimiter, err := NewTraceRateLimiter(setting)
	assert.NoError(t, err)
	assert.Equal(t, uint64(setting.RequestCount), traceRateLimiter.maxRequestCount)
	assert.Equal(t, uint64(setting.SpanCount), traceRateLimiter.maxSpanCount)
	assert.Equal(t, uint64(setting.SpanCountPerRequest), traceRateLimiter.maxSpanCountPerRequest)
	assert.Equal(t, uint64(0), traceRateLimiter.requestCount)
	assert.Equal(t, uint64(0), traceRateLimiter.spanCount)
	assert.Equal(t, uint64(0), traceRateLimiter.requestSpanCount)
}

func TestIncRequestCount_Traces(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	traceRateLimiter, err := NewTraceRateLimiter(setting)
	assert.NoError(t, err)
	traceRequestCountBeforeInc := traceRateLimiter.requestCount
	traceRateLimiter.IncRequestCount()
	traceRequestCountAfterInc := traceRateLimiter.requestCount
	assert.Equal(t, traceRequestCountBeforeInc+1, traceRequestCountAfterInc)
}

func TestIncSpanCount_Traces(t *testing.T) {
	setting := RateLimiterSetting{
		SpanCount: 100,
	}
	traceRateLimiter, err := NewTraceRateLimiter(setting)
	assert.NoError(t, err)
	traceSpanCountBeforeInc := traceRateLimiter.spanCount
	traceRateLimiter.SetRequestSpanCount(10)
	traceRateLimiter.IncSpanCount()
	traceSpanCountAfterInc := traceRateLimiter.spanCount
	assert.Equal(t, traceSpanCountBeforeInc+10, traceSpanCountAfterInc)
}

func TestResetRequestCount_Traces(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	traceRateLimiter, err := NewTraceRateLimiter(setting)
	assert.NoError(t, err)
	traceRateLimiter.ResetRequestCount()
	assert.Equal(t, uint64(0), traceRateLimiter.requestCount)
}

func TestResetSpanCount_Traces(t *testing.T) {
	setting := RateLimiterSetting{
		SpanCount: 100,
	}
	traceRateLimiter, err := NewTraceRateLimiter(setting)
	assert.NoError(t, err)
	traceRateLimiter.ResetSpanCount()
	assert.Equal(t, uint64(0), traceRateLimiter.spanCount)
}

func TestAcquireRequest_Traces(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount:        1,
		SpanCount:           2,
		SpanCountPerRequest: 2,
	}
	traceRateLimiter, err := NewTraceRateLimiter(setting)
	assert.NoError(t, err)

	traceRateLimiter.SetRequestSpanCount(2)
	ok, err := traceRateLimiter.Acquire()
	assert.Equal(t, true, ok)
	assert.NoError(t, err)

	// Should drop the second request as quota is exhausted
	ok, err = traceRateLimiter.Acquire()
	assert.Equal(t, false, ok)
	assert.Error(t, err)
}

func TestAcquireSpan_Traces(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount:        3,
		SpanCount:           2,
		SpanCountPerRequest: 2,
	}
	traceRateLimiter, err := NewTraceRateLimiter(setting)
	assert.NoError(t, err)

	traceRateLimiter.SetRequestSpanCount(2)
	ok, err := traceRateLimiter.Acquire()
	assert.Equal(t, true, ok)
	assert.NoError(t, err)

	// Should drop the second request as quota is exhausted
	ok, err = traceRateLimiter.Acquire()
	assert.Equal(t, false, ok)
	assert.Error(t, err)
}

func TestAcquireSpanRequest_Traces(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount:        3,
		SpanCount:           10,
		SpanCountPerRequest: 2,
	}
	traceRateLimiter, err := NewTraceRateLimiter(setting)
	assert.NoError(t, err)

	traceRateLimiter.SetRequestSpanCount(4)
	ok, err := traceRateLimiter.Acquire()
	assert.Equal(t, false, ok)
	assert.Error(t, err)
}

func TestAcquireAfterShutdown_Traces(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	traceRateLimiter, err := NewTraceRateLimiter(setting)
	assert.NoError(t, err)
	traceRateLimiter.Shutdown(context.Background())
	ok, err := traceRateLimiter.Acquire()
	assert.Equal(t, false, ok)
	assert.Error(t, err)
}

func TestRun_Traces(t *testing.T) {
	setting := RateLimiterSetting{
		RequestCount: 100,
	}
	traceRateLimiter, err := NewTraceRateLimiter(setting)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	traceRateLimiter.ticker.Reset(3 * time.Second)
	go traceRateLimiter.Run(ctx)
	traceRateLimiter.IncRequestCount()
	time.Sleep(5 * time.Second)
	assert.Equal(t, uint64(0), traceRateLimiter.requestCount)
	traceRateLimiter.Shutdown(ctx)
}
