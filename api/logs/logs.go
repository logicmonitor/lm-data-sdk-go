package logs

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/internal"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
)

const (
	uri = "/log/ingest"
)

var (
	logBatch      []model.LogInput
	logBatchMutex sync.Mutex
)

type LMLogIngest struct {
	client   *http.Client
	url      string
	batch    bool
	interval int
}

func NewLMLogIngest(batch bool, interval int) *LMLogIngest {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false, MinVersion: tls.VersionTLS12}
	clientTransport := (http.RoundTripper)(transport)
	client := http.Client{Transport: clientTransport, Timeout: 5 * time.Second}

	lli := LMLogIngest{
		client:   &client,
		url:      utils.URL(),
		batch:    batch,
		interval: interval,
	}
	if batch {
		go internal.CreateAndExportData(&lli)
	}
	return &lli
}

func (lli *LMLogIngest) SendLogs(logMessage string, resourceidMap, metadata map[string]string) (*utils.Response, error) {
	timestamp := strconv.Itoa(int(time.Now().Unix()))
	logsV1 := model.LogInput{
		Message:    logMessage,
		ResourceID: resourceidMap,
		Metadata:   metadata,
		Timestamp:  timestamp}

	if lli.batch {
		addRequest(logsV1)
	} else {
		body := model.LogPayload{
			Message:    logMessage,
			ResourceID: resourceidMap,
			Metadata:   metadata,
			Timestamp:  timestamp,
		}
		bodyarr := append([]model.LogPayload{}, body)
		logPayloadList := internal.DataPayload{
			LogBodyList: bodyarr,
		}
		return lli.ExportData(logPayloadList, uri, http.MethodPost)
	}
	return nil, nil
}

func addRequest(logInput model.LogInput) {
	logBatchMutex.Lock()
	defer logBatchMutex.Unlock()
	logBatch = append(logBatch, logInput)
}

func (lli *LMLogIngest) BatchInterval() int {
	return lli.interval
}

func (lli *LMLogIngest) URI() string {
	return uri
}

func (lli *LMLogIngest) CreateRequestBody() internal.DataPayload {
	var logPayloadList []model.LogPayload
	logBatchMutex.Lock()
	defer logBatchMutex.Unlock()
	if len(logBatch) == 0 {
		return internal.DataPayload{}
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
	payloadList := internal.DataPayload{
		LogBodyList: logPayloadList,
	}
	// flushing out log batch
	if lli.batch {
		logBatch = nil
	}
	return payloadList
}

func (lli *LMLogIngest) ExportData(payloadList internal.DataPayload, uri, method string) (*utils.Response, error) {
	body, err := json.Marshal(payloadList.LogBodyList)
	if err != nil {
		return nil, fmt.Errorf("error in marshaling log payload: %v", err)
	}
	resp, err := internal.MakeRequest(lli.client, lli.url, body, uri, method)
	if err != nil {
		return resp, err
	}
	return resp, err
}
