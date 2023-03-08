package logs

import (
	"net/http"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

// LMLogIngest configuration options
type Option func(*LMLogIngest) error

// WithLogBatchingInterval is used for configuring batching time interval for logs.
func WithLogBatchingInterval(batchingInterval time.Duration) Option {
	return func(lli *LMLogIngest) error {
		lli.batch.interval = batchingInterval
		return nil
	}
}

// WithLogBatchingDisabled is used for disabling log batching.
func WithLogBatchingDisabled() Option {
	return func(lli *LMLogIngest) error {
		lli.batch.enabled = false
		return nil
	}
}

// WithAuthentication is used for passing authentication token if not set in environment variables.
func WithAuthentication(authParams utils.AuthParams) Option {
	return func(lli *LMLogIngest) error {
		lli.auth = authParams
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

// WithHTTPClient is used to set HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(lli *LMLogIngest) error {
		lli.client = client
		return nil
	}
}

// WithEndpoint is used to set Endpoint URL to export logs
func WithEndpoint(endpoint string) Option {
	return func(lli *LMLogIngest) error {
		lli.url = endpoint
		return nil
	}
}

type SendLogsOptionalParameters struct {
	Headers map[string]string
}

func NewSendLogOptionalParameters() *SendLogsOptionalParameters {
	return &SendLogsOptionalParameters{}
}

func (s *SendLogsOptionalParameters) WithHeaders(headers map[string]string) *SendLogsOptionalParameters {
	s.Headers = headers
	return s
}
