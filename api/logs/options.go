package logs

import (
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
)

type Option func(*LMLogIngest) error

// WithLogBatchingEnabled is used for enabling batching for logs. Pass time interval for batch as an input parameter.
func WithLogBatchingEnabled(batchingInterval time.Duration) Option {
	return func(lli *LMLogIngest) error {
		lli.batch = true
		lli.interval = batchingInterval
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
