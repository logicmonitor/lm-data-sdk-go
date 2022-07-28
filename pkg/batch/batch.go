package batch

import (
	"log"
	"net/http"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/model"
)

// CreateAndExportData creates and exports data (if batching is enabled) after batching interval expires
func CreateAndExportData(li model.LMIngest) {
	ticker := time.NewTicker(li.BatchInterval())
	for range ticker.C {
		body := li.CreateRequestBody()
		err := li.ExportData(body, li.URI(), http.MethodPost)
		if err != nil {
			log.Println(err)
		}
	}
}
