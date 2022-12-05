package metrics

import (
	"net/http"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

type Option func(*LMMetricIngest) error

// WithMetricBatchingInterval is used for passing batch time interval.
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
func WithAuthentication(authParams utils.AuthParams) Option {
	return func(lmi *LMMetricIngest) error {
		lmi.auth = authParams
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

// WithRateLimit is used to limit the metric request count per minute
func WithRateLimit(requestCount int) Option {
	return func(lmi *LMMetricIngest) error {
		lmi.rateLimiterSetting.RequestCount = requestCount
		return nil
	}
}

// WithHTTPClient is used to set HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(lmi *LMMetricIngest) error {
		lmi.client = client
		return nil
	}
}

// WithEndpoint is used to set Endpoint URL to export logs
func WithEndpoint(endpoint string) Option {
	return func(lmi *LMMetricIngest) error {
		lmi.url = endpoint
		return nil
	}
}
