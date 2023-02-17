package translator

import (
	"github.com/logicmonitor/lm-data-sdk-go/model"
)

func ConvertToLMLogInput(logMessage interface{}, timestamp string, resourceidMap, metadata map[string]interface{}) model.LogInput {
	return model.LogInput{
		Message:    logMessage,
		ResourceID: resourceidMap,
		Metadata:   metadata,
		Timestamp:  timestamp,
	}
}
