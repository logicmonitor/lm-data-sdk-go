package logs

import "time"

type Option func(*LMLogIngest) error

func WithLogBatchingEnabled(batchingInterval time.Duration) Option {
	return func(lmh *LMLogIngest) error {
		lmh.batch = true
		lmh.interval = batchingInterval
		return nil
	}
}
