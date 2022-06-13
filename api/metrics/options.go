package metrics

import "time"

type Option func(*LMMetricIngest) error

func WithMetricBatchingEnabled(batchingInterval time.Duration) Option {
	return func(lmh *LMMetricIngest) error {
		lmh.batch = true
		lmh.interval = batchingInterval
		return nil
	}
}
