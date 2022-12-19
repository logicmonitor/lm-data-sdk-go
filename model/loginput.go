package model

type LogInput struct {
	Message    string
	ResourceID map[string]interface{}
	Metadata   map[string]interface{}
	Timestamp  string
}
