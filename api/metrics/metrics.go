package metrics

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/logicmonitor/go-data-sdk/internal"
	"github.com/logicmonitor/go-data-sdk/model"
	"github.com/logicmonitor/go-data-sdk/utils"
	"github.com/logicmonitor/go-data-sdk/validator"
)

const (
	uri              = "/metric/ingest"
	updateResPropURI = "/resource_property/ingest"
	updateInsPropURI = "/instance_property/ingest"
)

var datapointMap map[string][]model.DataPointInput
var instanceMap map[string][]model.InstanceInput
var dsMap map[string]model.DatasourceInput
var resourceMap map[string]model.ResourceInput
var metricBatch []model.MetricsInput
var lastTimeSend int64

type LMMetricIngest struct {
	Client   *http.Client
	URL      string
	Batch    bool
	Interval int
}

func NewLMMetricIngest(batch bool, interval int) *LMMetricIngest {
	client := http.Client{}
	lmi := LMMetricIngest{
		Client:   &client,
		URL:      utils.URL(),
		Batch:    batch,
		Interval: interval,
	}
	if batch {
		go internal.BatchPoller(lmi)
		go internal.CheckFlag(lmi)
	}
	return &lmi
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
		go internal.BatchPoller(lmi)
		addRequest(input, &m)
	} else {
		payload := createSingleRequestBody(input)
		body, err := json.Marshal(payload)
		if err != nil {
			log.Println("error in marshaling single metric payload: ", err)
		}
		resp, err := lmi.ExportData(body, uri, http.MethodPost)
		if err != nil {
			if resp != nil {
				log.Println("error response message while exporting single metric: ", resp.Message)
			}
			return resp, fmt.Errorf("error while exporting single metric: %v ", err)
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

func (lmi LMMetricIngest) BatchInterval() int {
	return lmi.Interval
}

// addRequest adds the metric request to batching cache if batching is enabled
func addRequest(input model.MetricsInput, m *sync.Mutex) {
	m.Lock()
	defer m.Unlock()
	metricBatch = append(metricBatch, input)
}

// mergeRequest merges the requests present in batching cache at the end of every batching interval
func (lmi LMMetricIngest) CreateRequestBody() ([]byte, error) {
	// merge the requests from map
	var m sync.Mutex
	resourceMap = make(map[string]model.ResourceInput)
	dsMap = make(map[string]model.DatasourceInput)
	instanceMap = make(map[string][]model.InstanceInput)
	datapointMap = make(map[string][]model.DataPointInput)

	m.Lock()
	defer m.Unlock()
	if len(metricBatch) == 0 {
		return nil, nil
	}
	for _, singleRequest := range metricBatch {
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
	body, err := lmi.createRestMetricsPayload()
	if err != nil {
		return nil, fmt.Errorf("error in creating metric payload")
	}
	return body, nil
}

func (lmi LMMetricIngest) URI() string {
	return uri
}

// createRestMetricsBody creates metrics request body
func (lmi LMMetricIngest) createRestMetricsPayload() ([]byte, error) {
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
		return nil, fmt.Errorf("error in marshaling batched metric payload: %v", err)
	}
	// flushing out the cache after exporting
	metricBatch = nil

	return body, err
}

func (lmi LMMetricIngest) ExportData(body []byte, uri, method string) (*utils.Response, error) {
	resp, err := internal.MakeRequest(lmi.Client, lmi.URL, body, uri, method)
	if err != nil {
		return resp, err
	}
	return resp, err
}

func (lmi LMMetricIngest) UpdateResourceProperties(resIDs, resProps map[string]string, patch bool) (*utils.Response, error) {
	if resIDs != nil {
		validator.CheckResourceIDValidation(resIDs)
	}
	if resProps != nil {
		validator.CheckResourcePropertiesValidation(resProps)
	}
	updateResProp := model.MetricPayload{
		ResourceID:         resIDs,
		ResourceProperties: resProps,
	}
	method := http.MethodPut
	if patch {
		method = http.MethodPatch
	}
	body, err := json.Marshal(updateResProp)
	if err != nil {
		return nil, fmt.Errorf("error in marshaling update resource properties: %v", err)
	}
	resp, err := lmi.ExportData(body, updateResPropURI, method)
	if err != nil {
		return nil, fmt.Errorf("error in updating resource properties: %v", err)
	}
	return resp, nil
}

func (lmi LMMetricIngest) UpdateInstanceProperties(resIDs, insProps map[string]string, dsName, dsDisplayName, insName string, patch bool) (*utils.Response, error) {
	errorMsg := ""
	if resIDs != nil {
		errorMsg += validator.CheckResourceIDValidation(resIDs)
	}
	if insProps != nil {
		errorMsg += validator.CheckInstancePropertiesValidation(insProps)
	}
	if insName != "" {
		errorMsg += validator.CheckInstanceNameValidation(insName)
	}
	if dsName != "" {
		errorMsg += validator.CheckDataSourceNameValidation(dsName)
	}
	if dsDisplayName != "" {
		errorMsg += validator.CheckDSDisplayNameValidation(dsDisplayName)
	}

	if errorMsg != "" {
		return &utils.Response{Message: errorMsg, Success: false}, fmt.Errorf("Validation failed!")
	}

	method := http.MethodPut
	if patch {
		method = http.MethodPatch
	}
	updateInsProp := model.MetricPayload{
		ResourceID:            resIDs,
		DataSourceName:        dsName,
		DataSourceDisplayName: dsDisplayName,
		Instances:             append([]model.Instance{}, model.Instance{InstanceName: insName, InstanceProperties: insProps}),
	}
	body, err := json.Marshal(updateInsProp)
	if err != nil {
		return nil, fmt.Errorf("error in marshaling update instance properties: %v", err)
	}
	resp, err := lmi.ExportData(body, updateInsPropURI, method)
	if err != nil {
		return nil, fmt.Errorf("error in updating instance properties: %v", err)
	}
	return resp, nil
}
