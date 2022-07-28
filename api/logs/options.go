package logs

import (
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
)

type Option func(*LMLogIngest) error

// WithLogBatchingInterval is used for passing batching time interval for logs.
func WithLogBatchingInterval(batchingInterval time.Duration) Option {
	return func(lli *LMLogIngest) error {
		lli.interval = batchingInterval
		return nil
	}
}

// WithLogBatchingDisabled is used for disabling log batching.
func WithLogBatchingDisabled() Option {
	return func(lli *LMLogIngest) error {
		lli.batch = false
		return nil
	}
}

// WithAuthentication is used for passing authentication token if not set in environment variables.
func WithAuthentication(authProvider model.AuthProvider) Option {
	return func(lli *LMLogIngest) error {
		lli.auth = authProvider
		return nil
	}
}

// WithGzipCompression can be used to enable/disable gzip compression of log payload
// Note: By default, gzip compression is enabled.
func WithGzipCompression(gzip bool) Option {
	return func(lli *LMLogIngest) error {
		lli.gzip = gzip
		return nil
	}
}

// WithRateLimit is used to limit the log request count per minute
func WithRateLimit(requestCount int) Option {
	return func(lli *LMLogIngest) error {
		lli.rateLimiterSetting.RequestCount = requestCount
		return nil
	}
}
