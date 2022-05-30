package logs

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/logicmonitor/go-data-sdk/internal"
	"github.com/logicmonitor/go-data-sdk/model"
	"github.com/logicmonitor/go-data-sdk/utils"
)

const (
	uri = "/log/ingest"
)

var logBatch []model.LogInput
var lastTimeSend int64

type LMLogIngest struct {
	Client   *http.Client
	URL      string
	Batch    bool
	Interval int
}

func NewLMLogIngest(batch bool, interval int) *LMLogIngest {
	client := http.Client{}
	lli := LMLogIngest{
		Client:   &client,
		URL:      utils.URL(),
		Batch:    batch,
		Interval: interval,
	}
	if batch {
		go internal.BatchPoller(lli)
		go internal.CheckFlag(lli)
	}
	return &lli
}

func (lli LMLogIngest) SendLogs(logMessage string, resourceidMap, metadata map[string]string) (*utils.Response, error) {
	timestamp := strconv.Itoa(int(time.Now().Unix()))
	logsV1 := model.LogInput{
		Message:    logMessage,
		ResourceID: resourceidMap,
		Metadata:   metadata,
		Timestamp:  timestamp}

	var m sync.Mutex
	if lli.Batch {
		addRequest(logsV1, &m)
	} else {
		body := model.LogPayload{
			Message:    logMessage,
			ResourceID: resourceidMap,
			Metadata:   metadata,
			Timestamp:  timestamp,
		}
		bodyarr := append([]model.LogPayload{}, body)
		singleReqBody, err := json.Marshal(bodyarr)
		if err != nil {
			log.Println(err)
		}
		return lli.ExportData(singleReqBody, uri, http.MethodPost)
	}
	return nil, nil
}

func addRequest(logInput model.LogInput, m *sync.Mutex) {
	m.Lock()
	defer m.Unlock()
	logBatch = append(logBatch, logInput)
}

func (lli LMLogIngest) BatchInterval() int {
	return lli.Interval
}

func (lli LMLogIngest) URI() string {
	return uri
}

func (lli LMLogIngest) CreateRequestBody() ([]byte, error) {
	var m sync.Mutex
	var logPayloadList []model.LogPayload
	m.Lock()
	defer m.Unlock()
	if len(logBatch) == 0 {
		return nil, nil
	}
	for _, logsV1 := range logBatch {
		body := model.LogPayload{
			Message:    logsV1.Message,
			ResourceID: logsV1.ResourceID,
			Metadata:   logsV1.Metadata,
			Timestamp:  logsV1.Timestamp,
		}
		logPayloadList = append(logPayloadList, body)
	}
	body, err := json.Marshal(logPayloadList)
	if err != nil {
		log.Println(err)
	}
	// flushing out log batch
	logBatch = nil

	// resp, err := lli.exportLogs(body, uri, http.MethodPost)
	// if err != nil {
	// 	return resp, err
	// }
	return body, nil
}

func (lli LMLogIngest) ExportData(body []byte, uri, method string) (*utils.Response, error) {
	resp, err := internal.MakeRequest(lli.Client, lli.URL, body, uri, method)
	if err != nil {
		return resp, err
	}
	return resp, err
}
