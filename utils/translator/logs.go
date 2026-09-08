package translator

import (
	"github.com/logicmonitor/lm-data-sdk-go/model"
)

func ConvertToLMLogInput(logMessage interface{}, loglevel string, timestamp string, resourceID interface{}, metadata map[string]interface{}) model.LogInput {
	return model.LogInput{
		Message:    logMessage,
		LogLevel:   loglevel,
		ResourceID: resourceID,
		Metadata:   metadata,
		Timestamp:  timestamp,
	}
}
