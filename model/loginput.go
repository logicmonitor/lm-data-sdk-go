package model

type LogInput struct {
	Message    string
	ResourceID map[string]string
	Metadata   map[string]string
	Timestamp  string
}
