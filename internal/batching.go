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
	MetricBodyList       []model.MetricPayload
	LogBodyList          []model.LogPayload
	UpdatePropertiesBody model.UpdateProperties
}

type LMIngest interface {
	BatchInterval() int
	URI() string
	CreateRequestBody() DataPayload
	ExportData(body DataPayload, uri, method string) (*utils.Response, error)
}

func CreateAndExportData(li LMIngest) {
	ticker := time.NewTicker(time.Duration(li.BatchInterval()) * time.Second)
	for range ticker.C {
		body := li.CreateRequestBody()
		_, err := li.ExportData(body, li.URI(), http.MethodPost)
		if err != nil {
			log.Println("error while exporting data..", err)
		}
	}
}

// MakeRequest compresses the payload and exports it to LM Platform
func MakeRequest(client *http.Client, url string, body []byte, uri, method string) (*utils.Response, error) {
	token, err := utils.GetToken(method, body, uri)
	if err != nil {
		return nil, err
	}
	compressedBody, err := utils.Gzip(body)
	if err != nil {
		return nil, fmt.Errorf("error while compressing body: %v", err)
	}
	reqBody := bytes.NewBuffer(compressedBody)
	fullURL := url + uri

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", token)
	req.Header.Add("User-Agent", utils.BuildUserAgent())
	req.Header.Add("Content-Encoding", "gzip")

	httpResp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return utils.ConvertHTTPToIngestResponse(httpResp)
}
