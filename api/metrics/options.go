package metrics

import (
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
)

type Option func(*LMMetricIngest) error

// WithMetricBatchingEnabled is used for enabling batching for metrics. Pass time interval for batch as an input parameter.
func WithMetricBatchingEnabled(batchingInterval time.Duration) Option {
	return func(lmh *LMMetricIngest) error {
		lmh.batch = true
		lmh.interval = batchingInterval
		return nil
	}
}

// WithAuthentication is used for passing authentication token if not set in environment variables.
func WithAuthentication(authProvider model.AuthProvider) Option {
	return func(lmh *LMMetricIngest) error {
		lmh.auth = authProvider
		return nil
	}
}
