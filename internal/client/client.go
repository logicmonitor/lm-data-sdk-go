package client

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type RequestConfig struct {
	Client      *http.Client
	RateLimiter rateLimiter.RateLimiter
	Url         string
	Body        []byte
	Uri         string
	Method      string
	Token       string
	Gzip        bool
	Headers     map[string]string
}

// MakeRequest compresses the payload and exports it to LM Platform
func MakeRequest(_ context.Context, reqConfig RequestConfig) (*utils.Response, error) {
	if reqConfig.Token == "" {
		return nil, fmt.Errorf("missing authentication token")
	}
	payloadBody := reqConfig.Body
	var err error
	if reqConfig.Gzip {
		payloadBody, err = utils.Gzip(payloadBody)
		if err != nil {
			return nil, fmt.Errorf("error while compressing body: %v", err)
		}
	}
	reqBody := bytes.NewBuffer(payloadBody)
	fullURL := reqConfig.Url + reqConfig.Uri

	req, err := http.NewRequest(reqConfig.Method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", reqConfig.Token)
	req.Header.Add("User-Agent", utils.BuildUserAgent())

	if reqConfig.Gzip {
		req.Header.Add("Content-Encoding", "gzip")
	}

	for key, value := range reqConfig.Headers {
		req.Header.Set(key, value)
	}

	spanlimiter := reqConfig.RateLimiter.(rateLimiter.SpanRateLimiter)
	count := getSpanCount(reqConfig.Body)
	spanlimiter.SetRequestSpanCount(count)
	if acquire, err := reqConfig.RateLimiter.Acquire(); !acquire {
		return nil, err
	}

	httpResp, err := reqConfig.Client.Do(req)
	if err != nil {
		return nil, err
	}
	spanlimiter.ResetSpanPerRequestCount()

	return utils.ConvertHTTPToIngestResponse(httpResp)
}

func getSpanCount(body []byte) int {
	tracesUnmarshaler := ptrace.NewProtoUnmarshaler()
	traceData, err := tracesUnmarshaler.UnmarshalTraces(body)
	if err != nil {
		return 0
	}
	return traceData.SpanCount()
}
