package otlpmetrics

import (
	"net/http"
	"time"

	ratelimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

type Option func(*LMOTLPMetricIngest) error

// WithOTLPMetricBatchingInterval sets the interval between OTLP metric batches.
func WithOTLPMetricBatchingInterval(batchingInterval time.Duration) Option {
	return func(ingest *LMOTLPMetricIngest) error {
		ingest.batch.interval = batchingInterval
		return nil
	}
}

// WithOTLPMetricBatchingDisabled disables SDK-side batching of OTLP metrics.
func WithOTLPMetricBatchingDisabled() Option {
	return func(ingest *LMOTLPMetricIngest) error {
		ingest.batch.enabled = false
		return nil
	}
}

// WithAuthentication sets authentication parameters if not taken from the environment.
func WithAuthentication(authProvider utils.AuthParams) Option {
	return func(ingest *LMOTLPMetricIngest) error {
		ingest.auth = authProvider
		return nil
	}
}

// WithGzipCompression enables or disables gzip compression of the OTLP payload.
// Gzip is enabled by default.
func WithGzipCompression(gzip bool) Option {
	return func(ingest *LMOTLPMetricIngest) error {
		ingest.gzip = gzip
		return nil
	}
}

// WithRateLimit limits OTLP metric requests per minute.
func WithRateLimit(requestCount int) Option {
	return func(ingest *LMOTLPMetricIngest) error {
		ingest.rateLimiterSetting.RequestCount = requestCount
		return nil
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(ingest *LMOTLPMetricIngest) error {
		ingest.client = client
		return nil
	}
}

// WithEndpoint sets the base REST URL (SDK appends /api/v1/metrics).
func WithEndpoint(endpoint string) Option {
	return func(ingest *LMOTLPMetricIngest) error {
		ingest.url = endpoint
		return nil
	}
}

// WithCollectorID sets Collector-ID on the request header.
func WithCollectorID(collectorID string) Option {
	return func(ingest *LMOTLPMetricIngest) error {
		ingest.collectorID = collectorID
		return nil
	}
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(userAgent string) Option {
	return func(ingest *LMOTLPMetricIngest) error {
		ingest.userAgent = userAgent
		return nil
	}
}

func WithRateLimiterDisabled() Option {
	return func(ingest *LMOTLPMetricIngest) error {
		ingest.rateLimiter = &ratelimiter.NoopRateLimiter{}
		return nil
	}
}

type SendOTLPMetricsOptionalParameters struct{}

func NewSendOTLPMetricsOptionalParameters() *SendOTLPMetricsOptionalParameters {
	return &SendOTLPMetricsOptionalParameters{}
}
