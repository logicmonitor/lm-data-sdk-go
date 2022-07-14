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
	uri                     = "/v2/metric/ingest"
	createFlag              = "?create=true"
	updateResPropURI        = "/resource_property/ingest"
	updateInsPropURI        = "/instance_property/ingest"
	defaultAggType          = "none"
	defaultDPType           = "GAUGE"
	defaultBatchingInterval = 10 * time.Second
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
	gzip     bool
}

// NewLMMetricIngest initializes LMMetricIngest
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
		client:   &client,
		url:      metricsURL,
		batch:    true,
		interval: defaultBatchingInterval,
		auth:     model.DefaultAuthenticator{},
		gzip:     true,
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

// SendMetrics is the entry point for receiving metric data. It also validates the attributes of metrics before creating metric payload.
func (lmi *LMMetricIngest) SendMetrics(ctx context.Context, rInput model.ResourceInput, dsInput model.DatasourceInput, instInput model.InstanceInput, dpInput model.DataPointInput) error {
	errorMsg := validator.ValidateAttributes(rInput, dsInput, instInput, dpInput)
	if errorMsg != "" {
		return fmt.Errorf("Validation failed : %s", errorMsg)
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
		var payloadList internal.DataPayload
		if input.Resource.IsCreate {
			payloadList = internal.DataPayload{
				MetricResourceCreateList: singlePayloadBody,
			}
		} else {
			payloadList = internal.DataPayload{
				MetricBodyList: singlePayloadBody,
			}
		}
		return lmi.ExportData(payloadList, uri, http.MethodPost)
	}
	return nil
}

// setDefaultValues sets default values to missing or empty attribute fields
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

// createSingleRequestBody prepares metric payload for single request when batching is disabled
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

// BatchInterval returns the time interval for batching
func (lmi LMMetricIngest) BatchInterval() time.Duration {
	return lmi.interval
}

// addRequest adds the metric request to batching cache if batching is enabled
func addRequest(input model.MetricsInput) {
	metricBatchMutex.Lock()
	defer metricBatchMutex.Unlock()
	metricBatch = append(metricBatch, input)
}

// CreateRequestBody merges the requests present in batching cache and creates metric payload at the end of every batching interval
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

// URI returns the endpoint/uri of metric ingest API
func (lmi LMMetricIngest) URI() string {
	return uri
}

// createRestMetricsPayload prepares metrics payload
func (lmi *LMMetricIngest) createRestMetricsPayload() internal.DataPayload {
	var payloadList []model.MetricPayload
	var payloadListCreateFlag []model.MetricPayload
	for resName, resDetails := range resourceMap {
		var payload model.MetricPayload
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
					payload.Instances = instances
					if resDetails.IsCreate {
						payloadListCreateFlag = append(payloadListCreateFlag, payload)
					} else {
						payloadList = append(payloadList, payload)
					}
				}
			}
		}
	}

	metricPayload := internal.DataPayload{
		MetricBodyList:           payloadList,
		MetricResourceCreateList: payloadListCreateFlag,
	}
	// flushing out the metric batch after exporting
	if lmi.batch {
		metricBatch = nil
	}
	return metricPayload
}

// ExportData exports metrics to the LM platform
func (lmi *LMMetricIngest) ExportData(payloadList internal.DataPayload, uri, method string) error {
	var errStrings []string
	if method == http.MethodPatch || method == http.MethodPut {
		payloadBody, err := json.Marshal(payloadList.UpdatePropertiesBody)
		if err != nil {
			errStrings = append(errStrings, "error in marshaling update property metric payload: "+err.Error())
		}
		token := lmi.auth.GetCredentials(method, uri, payloadBody)
		_, err = internal.MakeRequest(lmi.client, lmi.url, payloadBody, uri, method, token, lmi.gzip)
		if err != nil {
			errStrings = append(errStrings, "error in updating properties: "+err.Error())
		}
	}
	if len(payloadList.MetricBodyList) > 0 {
		payloadBody, err := json.Marshal(payloadList.MetricBodyList)
		if err != nil {
			errStrings = append(errStrings, "error in marshaling metric payload: "+err.Error())
		}
		token := lmi.auth.GetCredentials(method, uri, payloadBody)
		_, err = internal.MakeRequest(lmi.client, lmi.url, payloadBody, uri, method, token, lmi.gzip)
		if err != nil {
			errStrings = append(errStrings, "error while exporting metrics: "+err.Error())
		}
	}
	if len(payloadList.MetricResourceCreateList) > 0 {
		metricURI := uri + createFlag
		payloadBody, err := json.Marshal(payloadList.MetricResourceCreateList)
		if err != nil {
			errStrings = append(errStrings, "error in marshaling metric payload with create flag: "+err.Error())
		}
		token := lmi.auth.GetCredentials(method, uri, payloadBody)
		_, err = internal.MakeRequest(lmi.client, lmi.url, payloadBody, metricURI, method, token, lmi.gzip)
		if err != nil {
			errStrings = append(errStrings, "error while exporting metrics with create flag : "+err.Error())
		}
	}
	if len(errStrings) > 0 {
		return fmt.Errorf(strings.Join(errStrings, "\n"))
	}
	return nil
}

func (lmi *LMMetricIngest) UpdateResourceProperties(resName string, resIDs, resProps map[string]string, patch bool) error {
	if resName == "" || resIDs == nil || resProps == nil {
		return fmt.Errorf("One of the fields: resource name, resource ids or resource properties, is missing.")
	}
	errorMsg := ""
	errorMsg += validator.CheckResourceNameValidation(false, resName)
	errorMsg += validator.CheckResourceIDValidation(resIDs)
	errorMsg += validator.CheckResourcePropertiesValidation(resProps)

	if errorMsg != "" {
		return fmt.Errorf("Validation failed: %v ", errorMsg)
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
	return lmi.ExportData(updateResPropBody, updateResPropURI, method)
}

func (lmi *LMMetricIngest) UpdateInstanceProperties(resIDs, insProps map[string]string, dsName, dsDisplayName, insName string, patch bool) error {
	if resIDs == nil || insProps == nil || dsName == "" || insName == "" {
		return fmt.Errorf("One of the fields: instance name, datasource name, resource ids, or instance properties, is missing.")
	}
	errorMsg := ""
	errorMsg += validator.CheckResourceIDValidation(resIDs)
	errorMsg += validator.CheckInstancePropertiesValidation(insProps)
	errorMsg += validator.CheckInstanceNameValidation(insName)
	errorMsg += validator.CheckDataSourceNameValidation(dsName)
	errorMsg += validator.CheckDSDisplayNameValidation(dsDisplayName)

	if errorMsg != "" {
		return fmt.Errorf("Validation failed: %v", errorMsg)
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
	return lmi.ExportData(updateInsPropBody, updateInsPropURI, method)

}
