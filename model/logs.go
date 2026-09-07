package model

type LogInput struct {
	Message    interface{}
	LogLevel   string
	ResourceID interface{}
	Metadata   map[string]interface{}
	Timestamp  string
}

type LogPayload map[string]interface{}
