package model

type LogPayload struct {
	Message    string            `json:"msg"`
	ResourceID map[string]string `json:"_lm.resourceId"`
	Timestamp  string            `json:"timestamp"`
	Metadata   map[string]string `json:"metadata"`
}
