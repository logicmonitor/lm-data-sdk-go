package internal

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

type DataPayload struct {
	MetricBodyList           []model.MetricPayload
	MetricResourceCreateList []model.MetricPayload
	LogBodyList              []model.LogPayload
	UpdatePropertiesBody     model.UpdateProperties
}

type LMIngest interface {
	BatchInterval() time.Duration
	URI() string
	CreateRequestBody() DataPayload
	ExportData(body DataPayload, uri, method string) error
}

// CreateAndExportData creates and exports data (if batching is enabled) after batching interval expires
func CreateAndExportData(li LMIngest) {
	ticker := time.NewTicker(li.BatchInterval())
	for range ticker.C {
		body := li.CreateRequestBody()
		err := li.ExportData(body, li.URI(), http.MethodPost)
		if err != nil {
			log.Println(err)
		}
	}
}

// MakeRequest compresses the payload and exports it to LM Platform
func MakeRequest(client *http.Client, url string, body []byte, uri, method, token string, gzip bool) (*utils.Response, error) {
	if token == "" {
		return nil, fmt.Errorf("Missing authentication token.")
	}
	payloadBody := body
	var err error
	if gzip {
		payloadBody, err = utils.Gzip(payloadBody)
		if err != nil {
			return nil, fmt.Errorf("error while compressing body: %v", err)
		}
	}
	reqBody := bytes.NewBuffer(payloadBody)
	fullURL := url + uri

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", token)
	req.Header.Add("User-Agent", utils.BuildUserAgent())
	if gzip {
		req.Header.Add("Content-Encoding", "gzip")
	}

	httpResp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return utils.ConvertHTTPToIngestResponse(httpResp)
}
