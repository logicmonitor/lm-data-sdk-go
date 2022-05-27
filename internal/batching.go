package internal

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/logicmonitor/go-data-sdk/utils"
)

var ingestURL = "https://%s.logicmonitor.com/rest"

// exportMetric compresses the payload and exports it to LM Platform
func MakeRequest(client *http.Client, url string, body []byte, uri string) (*utils.Response, error) {

	fmt.Println("Payload Body::", string(body))

	token := utils.GetToken(http.MethodPost, body, uri)

	//url := fmt.Sprintf(ingestURL, os.Getenv("LM_COMPANY")) + uri

	compressedBody, err := utils.Gzip(body)
	if err != nil {
		fmt.Println("compression error::", err.Error())
		return nil, err
	}
	reqBody := bytes.NewBuffer(compressedBody)
	req, err := http.NewRequest(http.MethodPost, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", token)
	req.Header.Add("User-Agent", utils.BuildUserAgent())
	req.Header.Add("Content-Encoding", "gzip")

	fmt.Println("Request..", req)

	httpResp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return utils.ConvertHTTPToIngestResponse(httpResp)
}
