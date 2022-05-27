package internal

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/logicmonitor/go-data-sdk/utils"
)

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
