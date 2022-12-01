package traces

import (
	"net/http"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
)

type Option func(*LMTraceIngest) error

// WithTraceBatchingInterval is used for passing batch time interval.
func WithTraceBatchingInterval(batchingInterval time.Duration) Option {
	return func(lti *LMTraceIngest) error {
		lti.interval = batchingInterval
		return nil
	}
}

// WithTraceBatchingDisabled is used for disabling Trace batching.
func WithTraceBatchingDisabled() Option {
	return func(lti *LMTraceIngest) error {
		lti.batch = false
		return nil
	}
}

// WithAuthentication is used for passing authentication token if not set in environment variables.
func WithAuthentication(authProvider model.AuthProvider) Option {
	return func(lti *LMTraceIngest) error {
		lti.auth = authProvider
		return nil
	}
}

// WithGzipCompression can be used to enable/disable gzip compression of Trace payload
// Note: By default, gzip compression is enabled.
func WithGzipCompression(gzip bool) Option {
	return func(lti *LMTraceIngest) error {
		lti.gzip = gzip
		return nil
	}
}

// WithRateLimit is used to limit the Trace request count per minute
func WithRateLimit(requestCount int) Option {
	return func(lti *LMTraceIngest) error {
		lti.rateLimiterSetting.RequestCount = requestCount
		return nil
	}
}

// WithHTTPClient is used to set HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(lti *LMTraceIngest) error {
		lti.client = client
		return nil
	}
}

// WithEndpoint is used to set Endpoint URL to export traces
func WithEndpoint(endpoint string) Option {
	return func(lti *LMTraceIngest) error {
		lti.url = endpoint
		return nil
	}
}
