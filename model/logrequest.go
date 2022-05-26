package model

type LogPayload struct {
	Message    string            `json:"msg"`
	ResourceID map[string]string `json:"_lm.resourceId"`
	Timestamp  string
	Metadata   map[string]string
}
