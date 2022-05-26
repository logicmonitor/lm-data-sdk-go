package metrics

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/logicmonitor/go-data-sdk/internal"
	"github.com/logicmonitor/go-data-sdk/model"
	"github.com/logicmonitor/go-data-sdk/utils"
	"github.com/logicmonitor/go-data-sdk/validator"
)

const (
	metricsIngestURLFmtStr = "https://%s.logicmonitor.com/rest/metric/ingest"
	uri                    = "/metric/ingest"
)

var datapointMap map[string][]model.DataPointInput
var instanceMap map[string][]model.InstanceInput
var dsMap map[string]model.DatasourceInput
var resourceMap map[string]model.ResourceInput
var batchedReq []model.MetricsInput
var lastTimeSend int64

type LMMetricIngest struct {
	Client   *http.Client
	URL      string
	Batch    bool
	Interval int
}

func NewLMMetricIngest(batch bool, interval int) *LMMetricIngest {
	client := http.Client{}
	return &LMMetricIngest{
		Client:   &client,
		URL:      fmt.Sprintf(metricsIngestURLFmtStr, os.Getenv("LM_COMPANY")),
		Batch:    batch,
		Interval: interval,
	}
}

func (lmi LMMetricIngest) Start() {
	go lmi.batchPoller()
}

// SendMetrics validates the attributes and exports the metrics to LM Platform
func (lmi LMMetricIngest) SendMetrics(rInput model.ResourceInput, dsInput model.DatasourceInput, instInput model.InstanceInput, dpInput model.DataPointInput) (*utils.Response, error) {
	errorMsg := validator.ValidateAttributes(rInput, dsInput, instInput, dpInput)
	if errorMsg != "" {
		log.Fatal(errorMsg)
	}

	input := model.MetricsInput{
		Resource:   rInput,
		Datasource: dsInput,
		Instance:   instInput,
		DataPoint:  dpInput,
	}

	var m sync.Mutex
	if lmi.Batch {
		addRequest(input, &m)
	} else {
		payload := createSingleRequestBody(input)
		body, err := json.Marshal(payload)
		if err != nil {
			log.Println(err)
		}
		resp, err := lmi.exportMetric(body)
		if err != nil {
			log.Println("Error while sending metrics ", resp.Message)
		}
		return resp, err
	}
	return nil, nil
}

func createSingleRequestBody(input model.MetricsInput) model.MetricPayload {
	dp := model.DataPoint(input.DataPoint)
	instance := model.Instance{
		InstanceName:        input.Instance.InstanceName,
		InstanceID:          input.Instance.InstanceID,
		InstanceDisplayName: input.Instance.InstanceDisplayName,
		InstanceGroup:       input.Instance.InstanceGroup,
		InstanceProperties:  input.Instance.InstanceProperties,
		DataPoints:          append([]model.DataPoint{}, dp),
	}

	body := model.MetricPayload{
		ResourceName:          input.Resource.ResourceName,
		ResourceDescription:   input.Resource.ResourceDescription,
		ResourceID:            input.Resource.ResourceID,
		ResourceProperties:    input.Resource.ResourceProperties,
		DataSourceName:        input.Datasource.DataSourceName,
		DataSourceDisplayName: input.Datasource.DataSourceDisplayName,
		DataSourceGroup:       input.Datasource.DataSourceGroup,
		DataSourceID:          input.Datasource.DataSourceID,
		Instances:             append([]model.Instance{}, instance),
	}
	return body
}

// addRequest adds the metric request to batching cache if batching is enabled
func addRequest(input model.MetricsInput, m *sync.Mutex) {
	m.Lock()
	defer m.Unlock()
	batchedReq = append(batchedReq, input)
}

// mergeRequest merges the requests present in batching cache at the end of every batching interval
func (lmi LMMetricIngest) mergeAndCreateRequestBody() error {
	resourceMap = make(map[string]model.ResourceInput)
	dsMap = make(map[string]model.DatasourceInput)
	instanceMap = make(map[string][]model.InstanceInput)
	datapointMap = make(map[string][]model.DataPointInput)

	for _, singleRequest := range batchedReq {
		if _, ok := resourceMap[singleRequest.Resource.ResourceName]; !ok {
			resourceMap[singleRequest.Resource.ResourceName] = singleRequest.Resource
		}
		if _, ok := dsMap[singleRequest.Resource.ResourceName]; !ok {
			dsMap[singleRequest.Resource.ResourceName] = singleRequest.Datasource
		}

		if instArray, ok := instanceMap[singleRequest.Datasource.DataSourceName]; !ok {
			instanceMap[singleRequest.Datasource.DataSourceName] = append([]model.InstanceInput{}, singleRequest.Instance)
		} else {
			for _, ins := range instArray {
				if ins.InstanceName != singleRequest.Instance.InstanceName {
					instanceMap[singleRequest.Datasource.DataSourceName] = append(instArray, singleRequest.Instance)
				}
			}
		}

		if dpArray, ok := datapointMap[singleRequest.Instance.InstanceName]; !ok {
			datapointMap[singleRequest.Instance.InstanceName] = append([]model.DataPointInput{}, singleRequest.DataPoint)
		} else {
			datapointMap[singleRequest.Instance.InstanceName] = append(dpArray, singleRequest.DataPoint)
		}
	}

	// after merging create metric payload
	body, err := lmi.createRestMetricsBody()
	if err != nil {
		log.Println("error")
		return err
	}
	resp, err := lmi.exportMetric(body)
	if err != nil {
		if resp != nil {
			log.Println("Response message: ", resp.Message)
		}
		return err
	}
	return nil
}

// batchPoller checks for the batching interval
// if current time exceeds the interval, then it merges the request and create request body
func (lmi *LMMetricIngest) batchPoller() {
	for {
		if len(batchedReq) > 0 {
			currentTime := time.Now().Unix()
			if currentTime > (lastTimeSend + int64(lmi.Interval)) {
				lmi.mergeAndCreateRequestBody()
				lastTimeSend = currentTime
			}
		}
	}
}

// createRestMetricsBody creates metrics request body
func (lmi *LMMetricIngest) createRestMetricsBody() ([]byte, error) {
	var payload model.MetricPayload
	var payloadList []model.MetricPayload
	var dataPoints []model.DataPoint
	for resName, resDetails := range resourceMap {
		payload.ResourceName = resDetails.ResourceName
		payload.ResourceID = resDetails.ResourceID
		payload.ResourceDescription = resDetails.ResourceDescription
		payload.ResourceProperties = resDetails.ResourceProperties

		ds, dsExists := dsMap[resName]
		if dsExists {
			payload.DataSourceName = ds.DataSourceName
			payload.DataSourceID = ds.DataSourceID
			payload.DataSourceDisplayName = ds.DataSourceDisplayName
			payload.DataSourceGroup = ds.DataSourceGroup
		}

		instArray, _ := instanceMap[ds.DataSourceName]
		for _, instance := range instArray {
			if dpArray, exists := datapointMap[instance.InstanceName]; exists {
				for _, dp := range dpArray {
					dataPoint := model.DataPoint{
						DataPointName:            dp.DataPointName,
						DataPointType:            dp.DataPointType,
						DataPointAggregationType: dp.DataPointAggregationType,
						DataPointDescription:     dp.DataPointDescription,
						Value:                    dp.Value,
					}
					dataPoints = append(dataPoints, dataPoint)
				}
			}
			payload.Instances = append(payload.Instances, model.Instance{InstanceName: instance.InstanceName, InstanceID: instance.InstanceID, InstanceDisplayName: instance.InstanceDisplayName, InstanceGroup: instance.InstanceGroup, InstanceProperties: instance.InstanceProperties, DataPoints: dataPoints})
		}
		payloadList = append(payloadList, payload)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Println(err)
	}
	// flushing out the cache after exporting
	batchedReq = nil

	return body, err
}

func (lmi *LMMetricIngest) exportMetric(body []byte) (*utils.Response, error) {
	resp, err := internal.MakeRequest(lmi.Client, lmi.URL, body, uri)
	if err != nil {
		log.Println("Error while sending logs.. ", resp.Message)
	}
	return resp, err
}
