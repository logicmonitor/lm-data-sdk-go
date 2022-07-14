package metrics

import (
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
)

type Option func(*LMMetricIngest) error

// WithMetricBatchingEnabled is used for passing batch time interval.
func WithMetricBatchingInterval(batchingInterval time.Duration) Option {
	return func(lmi *LMMetricIngest) error {
		lmi.interval = batchingInterval
		return nil
	}
}

// WithMetricBatchingDisabled is used for disabling metric batching.
func WithMetricBatchingDisabled() Option {
	return func(lmi *LMMetricIngest) error {
		lmi.batch = false
		return nil
	}
}

// WithAuthentication is used for passing authentication token if not set in environment variables.
func WithAuthentication(authProvider model.AuthProvider) Option {
	return func(lmi *LMMetricIngest) error {
		lmi.auth = authProvider
		return nil
	}
}

// WithGzipCompression can be used to enable/disable gzip compression of metric payload
// Note: By default, gzip compression is enabled.
func WithGzipCompression(gzip bool) Option {
	return func(lmi *LMMetricIngest) error {
		lmi.gzip = gzip
		return nil
	}
}
