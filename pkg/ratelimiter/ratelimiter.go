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

// RateLimiterSetting represents the RateLimiter config
type RateLimiterSetting struct {
	RequestCount int
}
