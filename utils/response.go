package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/google/uuid"
)

//Response will contain variable for responses from logingest
type Response struct {
	Success   bool                     `json:"success"`
	Message   string                   `json:"message"`
	Errors    []map[string]interface{} `json:"errors"`
	RequestID uuid.UUID                `json:"requestId"`
}

func ConvertHTTPToIngestResponse(resp *http.Response) (*Response, error) {
	ingestResponse := &Response{}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	_ = resp.Body.Close()

	err = json.Unmarshal(body, ingestResponse)
	if err != nil {
		ingestResponse.Success = false
		return ingestResponse, fmt.Errorf("Invalid Response! , Status Code: %d , Error Message: %s", resp.StatusCode, string(body[:]))
	}
	if !(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted) {
		errMsg := ""
		if resp.StatusCode == http.StatusMultiStatus {
			for _, responseError := range ingestResponse.Errors {
				if responseError["message"] != nil {
					errMsg += responseError["message"].(string)
				} else if responseError["error"] != nil {
					errMsg += responseError["error"].(string)
				}
			}
		} else {
			errMsg = ingestResponse.Message
		}
		ingestResponse.Success = false
		return ingestResponse, fmt.Errorf("Status Code: %d , Error Message: %s", resp.StatusCode, errMsg)
	}
	ingestResponse.RequestID, _ = uuid.Parse(resp.Header.Get("x-request-id"))
	return ingestResponse, nil
}
