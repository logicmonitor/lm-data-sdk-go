package metrics

import (
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
)

type Option func(*LMMetricIngest) error

func WithMetricBatchingEnabled(batchingInterval time.Duration) Option {
	return func(lmh *LMMetricIngest) error {
		lmh.batch = true
		lmh.interval = batchingInterval
		return nil
	}
}
func WithAuthentication(authProvider model.AuthProvider) Option {
	return func(lmh *LMMetricIngest) error {
		lmh.auth = authProvider
		return nil
	}
}
