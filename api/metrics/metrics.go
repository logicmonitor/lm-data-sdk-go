package metrics

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/internal"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"github.com/logicmonitor/lm-data-sdk-go/validator"
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
var dsMap map[string][]model.DatasourceInput
var resourceMap map[string]model.ResourceInput
var (
	metricBatch      []model.MetricsInput
	metricBatchMutex sync.Mutex
)

type LMMetricIngest struct {
	client   *http.Client
	url      string
	batch    bool
	interval int
}

func NewLMMetricIngest(batch bool, interval int) *LMMetricIngest {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false, MinVersion: tls.VersionTLS12}
	clientTransport := (http.RoundTripper)(transport)
	client := http.Client{Transport: clientTransport, Timeout: 5 * time.Second}

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
		singlePayloadBody := append([]model.MetricPayload{}, payload)
		payloadList := internal.DataPayload{
			MetricBodyList: singlePayloadBody,
		}
		return lmi.ExportData(payloadList, uri, http.MethodPost)
	}
	return nil, nil
}

func setDefaultValues(dsInput model.DatasourceInput, instInput model.InstanceInput, dpInput model.DataPointInput) (model.DatasourceInput, model.InstanceInput, model.DataPointInput) {
	if dsInput.DataSourceDisplayName == "" {
		dsInput.DataSourceDisplayName = dsInput.DataSourceName
	}
	if instInput.InstanceName != "" {
		instInput.InstanceName = strings.ReplaceAll(instInput.InstanceName, "/", "-")
		if instInput.InstanceDisplayName == "" {
			instInput.InstanceDisplayName = instInput.InstanceName
		}
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
func (lmi *LMMetricIngest) CreateRequestBody() internal.DataPayload {
	// merge the requests from map
	resourceMap = make(map[string]model.ResourceInput)
	dsMap = make(map[string][]model.DatasourceInput)
	instanceMap = make(map[string][]model.InstanceInput)
	datapointMap = make(map[string][]model.DataPointInput)

	metricBatchMutex.Lock()
	defer metricBatchMutex.Unlock()
	if len(metricBatch) == 0 {
		return internal.DataPayload{}
	}
	for _, singleRequest := range metricBatch {
		if _, ok := resourceMap[singleRequest.Resource.ResourceName]; !ok {
			resourceMap[singleRequest.Resource.ResourceName] = singleRequest.Resource
		}

		var dsPresent bool
		if dsArray, ok := dsMap[singleRequest.Resource.ResourceName]; !ok {
			dsMap[singleRequest.Resource.ResourceName] = append([]model.DatasourceInput{}, singleRequest.Datasource)
		} else {
			for _, ds := range dsArray {
				if ds.DataSourceName == singleRequest.Datasource.DataSourceName {
					dsPresent = true
				}
			}
			if !dsPresent {
				dsMap[singleRequest.Resource.ResourceName] = append(dsArray, singleRequest.Datasource)
			}
		}

		var instPresent bool
		if instArray, ok := instanceMap[singleRequest.Datasource.DataSourceName]; !ok {
			instanceMap[singleRequest.Datasource.DataSourceName] = append([]model.InstanceInput{}, singleRequest.Instance)
		} else {
			for _, ins := range instArray {
				if ins.InstanceName == singleRequest.Instance.InstanceName {
					instPresent = true
				}
			}
			if !instPresent {
				instanceMap[singleRequest.Datasource.DataSourceName] = append(instArray, singleRequest.Instance)
			}
		}

		var dpPresent bool
		if dpArray, ok := datapointMap[singleRequest.Instance.InstanceName]; !ok {
			datapointMap[singleRequest.Instance.InstanceName] = append([]model.DataPointInput{}, singleRequest.DataPoint)
		} else {
			for _, dp := range dpArray {
				if dp.DataPointName == singleRequest.DataPoint.DataPointName {
					dpPresent = true
				}
			}
			if !dpPresent {
				datapointMap[singleRequest.Instance.InstanceName] = append(dpArray, singleRequest.DataPoint)
			}
		}
	}

	// after merging create metric payload
	body := lmi.createRestMetricsPayload()

	return body
}

func (lmi LMMetricIngest) URI() string {
	return uri
}

// createRestMetricsBody creates metrics request body
func (lmi *LMMetricIngest) createRestMetricsPayload() internal.DataPayload {
	var payload model.MetricPayload
	var payloadList []model.MetricPayload
	for resName, resDetails := range resourceMap {
		payload.ResourceName = resDetails.ResourceName
		payload.ResourceID = resDetails.ResourceID
		payload.ResourceDescription = resDetails.ResourceDescription
		payload.ResourceProperties = resDetails.ResourceProperties

		if dsArray, dsExists := dsMap[resName]; dsExists {
			for _, ds := range dsArray {
				payload.DataSourceName = ds.DataSourceName
				payload.DataSourceID = ds.DataSourceID
				payload.DataSourceDisplayName = ds.DataSourceDisplayName
				payload.DataSourceGroup = ds.DataSourceGroup
				instArray, _ := instanceMap[ds.DataSourceName]
				for _, instance := range instArray {
					var dataPoints []model.DataPoint
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
		}
	}

	metricPayload := internal.DataPayload{
		MetricBodyList: payloadList,
	}
	// flushing out the metric batch after exporting
	if lmi.batch {
		metricBatch = nil
	}
	return metricPayload
}

func (lmi *LMMetricIngest) ExportData(payloadList internal.DataPayload, uri, method string) (*utils.Response, error) {
	for _, payload := range payloadList.MetricBodyList {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("error in marshaling batched metric payload: %v", err)
		}
		resp, err := internal.MakeRequest(lmi.client, lmi.url, body, uri, method)
		if err != nil {
			return resp, err
		}
		return resp, err
	}
	return nil, nil
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

	resPropList := append([]model.MetricPayload{}, updateResProp)
	updateResPropBody := internal.DataPayload{
		MetricBodyList: resPropList,
	}
	resp, err := lmi.ExportData(updateResPropBody, updateResPropURI, method)
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
	insPropList := append([]model.MetricPayload{}, updateInsProp)
	updateInsPropBody := internal.DataPayload{
		MetricBodyList: insPropList,
	}
	resp, err := lmi.ExportData(updateInsPropBody, updateInsPropURI, method)
	if err != nil {
		return nil, fmt.Errorf("error in updating instance properties: %v", err)
	}
	return resp, nil
}
