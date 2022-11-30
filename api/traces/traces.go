package traces

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/internal/client"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	"github.com/logicmonitor/lm-data-sdk-go/pkg/batch"
	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

const uri = "/api/v1/traces"
const defaultBatchingInterval = 10 * time.Second

type LMTraceIngest struct {
	client             *http.Client
	url                string
	batch              bool
	interval           time.Duration
	auth               model.AuthProvider
	gzip               bool
	rateLimiterSetting rateLimiter.RateLimiterSetting
	rateLimiter        rateLimiter.RateLimiter
}

// NewLMLogIngest initializes LMLogIngest
func NewLMTraceIngest(ctx context.Context, opts ...Option) (*LMTraceIngest, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false, MinVersion: tls.VersionTLS12}
	clientTransport := (http.RoundTripper)(transport)
	client := http.Client{Transport: clientTransport, Timeout: 5 * time.Second}

	tracesURL, err := utils.URL()
	if err != nil {
		return nil, fmt.Errorf("error in forming Traces URL: %v", err)
	}
	lti := LMTraceIngest{
		client:             &client,
		url:                tracesURL,
		batch:              true,
		interval:           defaultBatchingInterval,
		auth:               model.DefaultAuthenticator{},
		gzip:               true,
		rateLimiterSetting: rateLimiter.RateLimiterSetting{},
	}

	for _, opt := range opts {
		if err := opt(&lti); err != nil {
			return nil, err
		}
	}
	if lti.batch {
		go batch.CreateAndExportData(&lti)
	}
	return &lti, nil
}

// URI returns the endpoint/uri of trace ingest API
func (lti *LMTraceIngest) URI() string {
	return uri
}

// BatchInterval returns the time interval for batching
func (lti LMTraceIngest) BatchInterval() time.Duration {
	return lti.interval
}

// SendTraces is the entry point for receiving trace data
func (lti *LMTraceIngest) SendTraces(ctx context.Context, traceData []byte) error {
	headers := make(map[string]string)
	headers["Content-Type"] = "application/x-protobuf"
	method := http.MethodPost
	token := lti.auth.GetCredentials(method, lti.URI(), traceData)
	cfg := client.RequestConfig{
		Client:      lti.client,
		RateLimiter: lti.rateLimiter,
		Url:         lti.url,
		Body:        traceData,
		Uri:         lti.URI(),
		Method:      method,
		Token:       token,
		Gzip:        lti.gzip,
		Headers:     headers,
	}

	_, err := client.MakeRequest(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("error while exporting traces : %v", err)
	}
	return err
}

func (lti *LMTraceIngest) CreateRequestBody() model.DataPayload {
	return model.DataPayload{}
}

// ExportData exports logs to the LM platform
func (lti *LMTraceIngest) ExportData(payloadList model.DataPayload, uri, method string) error {
	return nil
}
