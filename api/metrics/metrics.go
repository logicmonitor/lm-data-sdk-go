package metrics

import (
	"context"
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
	uri              = "/v2/metric/ingest"
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
	interval time.Duration
	auth     model.AuthProvider
}

func NewLMMetricIngest(ctx context.Context, opts ...Option) (*LMMetricIngest, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false, MinVersion: tls.VersionTLS12}
	clientTransport := (http.RoundTripper)(transport)
	client := http.Client{Transport: clientTransport, Timeout: 5 * time.Second}

	metricsURL, err := utils.URL()
	if err != nil {
		return nil, fmt.Errorf("Error in forming Metrics URL: %v", err)
	}

	lmi := LMMetricIngest{
		client: &client,
		url:    metricsURL,
		auth:   model.DefaultAuthenticator{},
	}
	for _, opt := range opts {
		if err := opt(&lmi); err != nil {
			return nil, err
		}
	}
	if lmi.batch {
		go internal.CreateAndExportData(&lmi)
	}
	return &lmi, nil
}

// SendMetrics validates the attributes and exports the metrics to LM Platform
func (lmi *LMMetricIngest) SendMetrics(ctx context.Context, rInput model.ResourceInput, dsInput model.DatasourceInput, instInput model.InstanceInput, dpInput model.DataPointInput) (*utils.Response, error) {
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
		if instInput.InstanceDisplayName == "" {
			instInput.InstanceDisplayName = instInput.InstanceName
		}
		instInput.InstanceName = strings.ReplaceAll(instInput.InstanceName, "/", "-")
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

func (lmi LMMetricIngest) BatchInterval() time.Duration {
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

// createRestMetricsPayload creates metrics request payload
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
				var instances []model.Instance
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
					instances = append(instances, model.Instance{InstanceName: instance.InstanceName, InstanceID: instance.InstanceID, InstanceDisplayName: instance.InstanceDisplayName, InstanceGroup: instance.InstanceGroup, InstanceProperties: instance.InstanceProperties, DataPoints: dataPoints})
				}
				payload.Instances = instances
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
	var payloadBody []byte
	var err error
	if method == http.MethodPatch || method == http.MethodPut {
		payloadBody, err = json.Marshal(payloadList.UpdatePropertiesBody)
		if err != nil {
			return nil, fmt.Errorf("error in marshaling update property payload: %v", err)
		}
	} else {
		if len(payloadList.MetricBodyList) > 0 {
			payloadBody, err = json.Marshal(payloadList.MetricBodyList)
			if err != nil {
				return nil, fmt.Errorf("error in marshaling metric payload: %v", err)
			}
		}
	}
	token := lmi.auth.GetCredentials(method, uri, payloadBody)
	resp, err := internal.MakeRequest(lmi.client, lmi.url, payloadBody, uri, method, token)
	if err != nil {
		return resp, fmt.Errorf("error while exporting metrics : %v", err)
	}
	return resp, err
}

func (lmi *LMMetricIngest) UpdateResourceProperties(resName string, resIDs, resProps map[string]string, patch bool) (*utils.Response, error) {
	if resName == "" || resIDs == nil || resProps == nil {
		return nil, fmt.Errorf("One of the fields: resource name, resource ids or resource properties, is missing.")
	}
	errorMsg := ""
	errorMsg += validator.CheckResourceNameValidation(false, resName)
	errorMsg += validator.CheckResourceIDValidation(resIDs)
	errorMsg += validator.CheckResourcePropertiesValidation(resProps)

	if errorMsg != "" {
		return nil, fmt.Errorf("Validation failed: %v ", errorMsg)
	}
	updateResProp := model.UpdateProperties{
		ResourceName:       resName,
		ResourceID:         resIDs,
		ResourceProperties: resProps,
	}
	method := http.MethodPut
	if patch {
		method = http.MethodPatch
	}

	updateResPropBody := internal.DataPayload{
		UpdatePropertiesBody: updateResProp,
	}
	resp, err := lmi.ExportData(updateResPropBody, updateResPropURI, method)
	if err != nil {
		return nil, fmt.Errorf("error in updating resource properties: %v", err)
	}
	return resp, nil
}

func (lmi *LMMetricIngest) UpdateInstanceProperties(resIDs, insProps map[string]string, dsName, dsDisplayName, insName string, patch bool) (*utils.Response, error) {
	if resIDs == nil || insProps == nil || dsName == "" || insName == "" {
		return nil, fmt.Errorf("One of the fields: instance name, datasource name, resource ids, or instance properties, is missing.")
	}
	errorMsg := ""
	errorMsg += validator.CheckResourceIDValidation(resIDs)
	errorMsg += validator.CheckInstancePropertiesValidation(insProps)
	errorMsg += validator.CheckInstanceNameValidation(insName)
	errorMsg += validator.CheckDataSourceNameValidation(dsName)
	errorMsg += validator.CheckDSDisplayNameValidation(dsDisplayName)

	if errorMsg != "" {
		return nil, fmt.Errorf("Validation failed: %v", errorMsg)
	}

	method := http.MethodPut
	if patch {
		method = http.MethodPatch
	}
	updateInsProp := model.UpdateProperties{
		ResourceID:            resIDs,
		DataSourceName:        dsName,
		DataSourceDisplayName: dsDisplayName,
		InstanceName:          insName,
		InstanceProperties:    insProps,
	}
	updateInsPropBody := internal.DataPayload{
		UpdatePropertiesBody: updateInsProp,
	}
	resp, err := lmi.ExportData(updateInsPropBody, updateInsPropURI, method)
	if err != nil {
		return nil, fmt.Errorf("error in updating instance properties: %v", err)
	}
	return resp, nil
}
