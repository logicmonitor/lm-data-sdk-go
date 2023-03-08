package model

import (
	"github.com/google/uuid"
)

type IngestResponse struct {
	StatusCode  int    `json:"statusCode"`
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	Error       error  `json:"error"`
	MultiStatus []struct {
		Code  int    `json:"code"`
		Error string `json:"error"`
	} `json:"multiStatus"`
	RequestID  uuid.UUID `json:"requestId"`
	RetryAfter int       `json:"retryAfter"`
}
