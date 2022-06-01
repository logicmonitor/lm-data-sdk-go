package internal

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/logicmonitor/go-data-sdk/utils"
)

type LMIngest interface {
	BatchInterval() int
	URI() string
	CreateRequestBody() ([]byte, error)
	ExportData(body []byte, uri, method string) (*utils.Response, error)
}

func CreateAndExportData(li LMIngest) {
	ticker := time.NewTicker(time.Duration(li.BatchInterval()) * time.Second)
	for range ticker.C {
		body, err := li.CreateRequestBody()
		if err != nil {
			log.Println("error while creating request body: ", err)
		}
		if body != nil {
			resp, err := li.ExportData(body, li.URI(), http.MethodPost)
			if err != nil {
				log.Println("error while exporting data..", err)
			}
			log.Println("Response Message: ", resp.Message)
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
