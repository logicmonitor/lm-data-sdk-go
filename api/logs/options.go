package logs

import (
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
)

type Option func(*LMLogIngest) error

func WithLogBatchingEnabled(batchingInterval time.Duration) Option {
	return func(lmh *LMLogIngest) error {
		lmh.batch = true
		lmh.interval = batchingInterval
		return nil
	}
}

func WithAuthentication(authProvider model.AuthProvider) Option {
	return func(lmh *LMLogIngest) error {
		lmh.auth = authProvider
		return nil
	}
}
