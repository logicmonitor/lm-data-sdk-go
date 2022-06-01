package metrics

import (
	"encoding/json"
	"fmt"
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
	defaultAggType   = "none"
	defaultDPType    = "GAUGE"
)

var datapointMap map[string][]model.DataPointInput
var instanceMap map[string][]model.InstanceInput
var dsMap map[string]model.DatasourceInput
var resourceMap map[string]model.ResourceInput
var (
	metricBatch      []model.MetricsInput
	metricBatchMutex sync.Mutex
)
var lastTimeSend int64

type LMMetricIngest struct {
	client   *http.Client
	url      string
	batch    bool
	interval int
}

func NewLMMetricIngest(batch bool, interval int) *LMMetricIngest {
	client := http.Client{}
	lmi := LMMetricIngest{
		client:   &client,
		url:      utils.URL(),
		batch:    batch,
		interval: interval,
	}
	if batch {
		go internal.CreateAndExportData(&lmi)
	}
	return &lmi
}

// SendMetrics validates the attributes and exports the metrics to LM Platform
func (lmi *LMMetricIngest) SendMetrics(rInput model.ResourceInput, dsInput model.DatasourceInput, instInput model.InstanceInput, dpInput model.DataPointInput) (*utils.Response, error) {
	errorMsg := validator.ValidateAttributes(rInput, dsInput, instInput, dpInput)
	if errorMsg != "" {
		return nil, fmt.Errorf("Validation failed : %s", errorMsg)
	}

	dsInput, instInput, dpInput = setDefaultValues(dsInput, instInput, dpInput)
	input := model.MetricsInput{
		Resource:   rInput,
		Datasource: dsInput,
		Instance:   instInput,
		DataPoint:  dpInput,
	}

	if lmi.batch {
		addRequest(input)
	} else {
		payload := createSingleRequestBody(input)
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("error in marshaling single metric payload: %v", err)
		}
		return lmi.ExportData(body, uri, http.MethodPost)
	}
	return nil, nil
}

func setDefaultValues(dsInput model.DatasourceInput, instInput model.InstanceInput, dpInput model.DataPointInput) (model.DatasourceInput, model.InstanceInput, model.DataPointInput) {
	if dsInput.DataSourceDisplayName == "" {
		dsInput.DataSourceDisplayName = dsInput.DataSourceName
	}
	if instInput.InstanceDisplayName == "" {
		instInput.InstanceDisplayName = instInput.InstanceName
	}
	if dpInput.DataPointDescription == "" {
		dpInput.DataPointDescription = dpInput.DataPointName
	}
	if dpInput.DataPointAggregationType == "" {
		dpInput.DataPointAggregationType = defaultAggType
	}
	if dpInput.DataPointType == "" {
		dpInput.DataPointType = defaultDPType
	}
	return dsInput, instInput, dpInput
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
	return lmi.interval
}

// addRequest adds the metric request to batching cache if batching is enabled
func addRequest(input model.MetricsInput) {
	metricBatchMutex.Lock()
	defer metricBatchMutex.Unlock()
	metricBatch = append(metricBatch, input)
}

// mergeRequest merges the requests present in batching cache at the end of every batching interval
func (lmi *LMMetricIngest) CreateRequestBody() ([]byte, error) {
	// merge the requests from map
	resourceMap = make(map[string]model.ResourceInput)
	dsMap = make(map[string]model.DatasourceInput)
	instanceMap = make(map[string][]model.InstanceInput)
	datapointMap = make(map[string][]model.DataPointInput)

	metricBatchMutex.Lock()
	defer metricBatchMutex.Unlock()
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
func (lmi *LMMetricIngest) createRestMetricsPayload() ([]byte, error) {
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
	return body, err
}

func (lmi *LMMetricIngest) ExportData(body []byte, uri, method string) (*utils.Response, error) {
	resp, err := internal.MakeRequest(lmi.client, lmi.url, body, uri, method)
	if err != nil {
		return resp, err
	}
	// flushing out the metric batch after exporting
	metricBatch = nil
	return resp, err
}

func (lmi *LMMetricIngest) UpdateResourceProperties(resIDs, resProps map[string]string, patch bool) (*utils.Response, error) {
	errorMsg := ""
	if resIDs != nil {
		errorMsg += validator.CheckResourceIDValidation(resIDs)
	}
	if resProps != nil {
		errorMsg += validator.CheckResourcePropertiesValidation(resProps)
	}

	if errorMsg != "" {
		return nil, fmt.Errorf("Validation failed: %v ", errorMsg)
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

func (lmi *LMMetricIngest) UpdateInstanceProperties(resIDs, insProps map[string]string, dsName, dsDisplayName, insName string, patch bool) (*utils.Response, error) {
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
		return nil, fmt.Errorf("Validation failed: %v", errorMsg)
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
