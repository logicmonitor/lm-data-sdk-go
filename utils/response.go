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
		ingestResponse.Message = fmt.Sprintf("Invalid Response! , Status Code: %d , Body: %s", resp.StatusCode, string(body[:]))
		return ingestResponse, err
	}
	ingestResponse.RequestID, _ = uuid.Parse(resp.Header.Get("x-request-id"))
	return ingestResponse, nil
}
