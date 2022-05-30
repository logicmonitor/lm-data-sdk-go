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
		return ingestResponse, fmt.Errorf("Invalid Response! , Status Code: %d , Body: %s", resp.StatusCode, string(body[:]))
	}
	if !(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted) {
		ingestResponse.Success = false
		return ingestResponse, fmt.Errorf("Error caught... , Status Code: %d , Body: %s", resp.StatusCode, ingestResponse.Message)
	}
	ingestResponse.RequestID, _ = uuid.Parse(resp.Header.Get("x-request-id"))
	return ingestResponse, nil
}
