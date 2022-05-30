package internal

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/logicmonitor/go-data-sdk/utils"
)

var lastTimeSend int64
var flag bool

type LMIngest interface {
	BatchInterval() int
	URI() string
	CreateRequestBody() ([]byte, error)
	ExportData(body []byte, uri, method string) (*utils.Response, error)
}

// batchPoller checks for the batching interval
// if current time exceeds the interval, then it creates request body by merging the requests present in buffer
func BatchPoller(li LMIngest) {
	for {
		currentTime := time.Now().Unix()
		if currentTime > (lastTimeSend + int64(li.BatchInterval())) {
			fmt.Println("Time exceeded........")
			flag = true
			lastTimeSend = currentTime
		}
	}
}

func CheckFlag(li LMIngest) {
	for {
		if flag {
			flag = false
			fmt.Println("Flag:: flag is true")
			body, err := li.CreateRequestBody()
			if err != nil {
				log.Println("error while creating request body: ", err)
			}
			if body != nil {
				resp, err := li.ExportData(body, li.URI(), http.MethodPost)
				if err != nil {
					log.Println("error while exporting data..", err)
				}
				log.Println("Response: ", resp)
			}
		}
	}
}

// MakeRequest compresses the payload and exports it to LM Platform
func MakeRequest(client *http.Client, url string, body []byte, uri, method string) (*utils.Response, error) {
	token := utils.GetToken(method, body, uri)
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
