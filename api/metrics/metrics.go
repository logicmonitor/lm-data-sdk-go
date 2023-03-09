package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/internal/client"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	rateLimiter "github.com/logicmonitor/lm-data-sdk-go/pkg/ratelimiter"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"github.com/logicmonitor/lm-data-sdk-go/validator"
	"go.uber.org/multierr"
)

const (
	uri                      = "/v2/metric/ingest"
	createFlag               = "?create=true"
	updateResPropURI         = "/resource_property/ingest"
	updateInsPropURI         = "/instance_property/ingest"
	defaultAggType           = "none"
	defaultDPType            = "GAUGE"
	defaultBatchingInterval  = 10 * time.Second
	maxHTTPResponseReadBytes = 64 * 1024
	headerRetryAfter         = "Retry-After"
)

type LMMetricIngest struct {
	client             *http.Client
	url                string
	auth               utils.AuthParams
	gzip               bool
	rateLimiterSetting rateLimiter.MetricsRateLimiterSetting
	rateLimiter        rateLimiter.RateLimiter
	batch              *metricBatch
}

type LMMetricIngestRequest struct {
	Payload                     []model.MetricPayload
	PayloadWithResourceCreation []model.MetricPayload
	UpdatePropertiesPayload     model.UpdateProperties
}

type LMMetricIngestResponse struct {
	Success    bool                     `json:"success"`
	Message    string                   `json:"message"`
	Code       int                      `json:"code"`
	ResourceID map[string]string        `json:"resourceId"`
	Errors     []map[string]interface{} `json:"errors"`
}

type metricBatch struct {
	enabled  bool
	data     []*LMMetricIngestRequest
	interval time.Duration
	lock     *sync.Mutex
}

func NewMetricBatch() *metricBatch {
	return &metricBatch{enabled: true, interval: defaultBatchingInterval, lock: &sync.Mutex{}}
}

// NewLMMetricIngest initializes LMMetricIngest
func NewLMMetricIngest(ctx context.Context, opts ...Option) (*LMMetricIngest, error) {
	metricIngest := LMMetricIngest{
		client:             client.Client(),
		auth:               utils.AuthParams{},
		gzip:               true,
		rateLimiterSetting: rateLimiter.MetricsRateLimiterSetting{},
		batch:              NewMetricBatch(),
	}
	for _, opt := range opts {
		if err := opt(&metricIngest); err != nil {
			return nil, err
		}
	}

	var err error
	if metricIngest.url == "" {
		metricsURL, err := utils.URL()
		if err != nil {
			return nil, fmt.Errorf("error in forming Metrics URL: %v", err)
		}
		metricIngest.url = metricsURL
	}

	metricIngest.rateLimiter, err = rateLimiter.NewMetricsRateLimiter(metricIngest.rateLimiterSetting)
	if err != nil {
		return nil, err
	}
	go metricIngest.rateLimiter.Run(ctx)

	if metricIngest.batch.enabled {
		go metricIngest.processBatch(ctx)
	}
	return &metricIngest, nil
}

// SendMetrics is the entry point for receiving metric data. It also validates the attributes of metrics before creating metric payload.
func (metricIngest *LMMetricIngest) SendMetrics(ctx context.Context, rInput model.ResourceInput, dsInput model.DatasourceInput, instInput model.InstanceInput, dpInput model.DataPointInput, o ...SendMetricsOptionalParameters) (*model.IngestResponse, error) {
	errorMsg := validator.ValidateAttributes(rInput, dsInput, instInput, dpInput)
	if errorMsg != "" {
		return nil, fmt.Errorf("validation failed: %s", errorMsg)
	}
	dsInput, instInput, dpInput = setDefaultValues(dsInput, instInput, dpInput)
	input := model.MetricsInput{
		Resource:   rInput,
		Datasource: dsInput,
		Instance:   instInput,
		DataPoint:  dpInput,
	}
	req, err := buildMetricRequest(ctx, input, o...)
	if err != nil {
		return nil, err
	}

	if metricIngest.batch.enabled {
		metricIngest.batch.pushToBatch(req)
		return nil, nil
	}
	return metricIngest.export(req, uri, http.MethodPost)
}

func buildMetricRequest(ctx context.Context, body model.MetricsInput, o ...SendMetricsOptionalParameters) (*LMMetricIngestRequest, error) {
	metricIngestReq := &LMMetricIngestRequest{}

	payload := append(metricIngestReq.Payload, buildMetricPayload(body))

	if body.Resource.IsCreate {
		metricIngestReq.PayloadWithResourceCreation = payload
	} else {
		metricIngestReq.Payload = payload
	}
	return metricIngestReq, nil
}

func buildMetricPayload(metricItem model.MetricsInput) model.MetricPayload {
	dp := model.DataPoint(metricItem.DataPoint)
	instance := model.Instance{
		InstanceName:        metricItem.Instance.InstanceName,
		InstanceID:          metricItem.Instance.InstanceID,
		InstanceDisplayName: metricItem.Instance.InstanceDisplayName,
		InstanceGroup:       metricItem.Instance.InstanceGroup,
		InstanceProperties:  metricItem.Instance.InstanceProperties,
		DataPoints:          append([]model.DataPoint{}, dp),
	}

	payload := model.MetricPayload{
		ResourceName:          metricItem.Resource.ResourceName,
		ResourceDescription:   metricItem.Resource.ResourceDescription,
		ResourceID:            metricItem.Resource.ResourceID,
		ResourceProperties:    metricItem.Resource.ResourceProperties,
		DataSourceName:        metricItem.Datasource.DataSourceName,
		DataSourceDisplayName: metricItem.Datasource.DataSourceDisplayName,
		DataSourceGroup:       metricItem.Datasource.DataSourceGroup,
		DataSourceID:          metricItem.Datasource.DataSourceID,
		Instances:             append([]model.Instance{}, instance),
	}
	return payload
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

// batchInterval returns the time interval for batching
func (batch *metricBatch) batchInterval() time.Duration {
	return batch.interval
}

// pushToBatch adds the metric request to batching cache if batching is enabled
func (batch *metricBatch) pushToBatch(req *LMMetricIngestRequest) {
	batch.lock.Lock()
	defer batch.lock.Unlock()
	batch.data = append(batch.data, req)
}

func (metricIngest *LMMetricIngest) processBatch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.NewTicker(metricIngest.batch.batchInterval()).C:
			req := metricIngest.batch.combineBatchedMetricsRequests()
			if req == nil {
				continue
			}
			_, err := metricIngest.export(req, metricIngest.uri(), http.MethodPost)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

// combineBatchedMetricsRequests merges the requests present in batching cache and creates metric payload at the end of every batching interval
func (batch *metricBatch) combineBatchedMetricsRequests() *LMMetricIngestRequest {
	// merge the requests from map
	resourceMap := make(map[string]model.ResourceInput)
	dsMap := make(map[string][]model.DatasourceInput)
	instanceMap := make(map[string][]model.InstanceInput)
	datapointMap := make(map[string][]model.DataPointInput)

	batch.lock.Lock()
	defer batch.lock.Unlock()

	if len(batch.data) == 0 {
		return nil
	}

	for _, metricItem := range batch.data {

		var metricPayload model.MetricPayload
		var isResCreatePayload bool

		if len(metricItem.Payload) > 0 {
			metricPayload = metricItem.Payload[0]
		} else {
			metricPayload = metricItem.PayloadWithResourceCreation[0]
			isResCreatePayload = true
		}

		if _, ok := resourceMap[metricPayload.ResourceName]; !ok {
			resourceMap[metricPayload.ResourceName] = model.ResourceInput{
				ResourceName:        metricPayload.ResourceName,
				ResourceDescription: metricPayload.ResourceDescription,
				ResourceID:          metricPayload.ResourceID,
				ResourceProperties:  metricPayload.ResourceProperties,
				IsCreate:            isResCreatePayload,
			}
		}

		var dsPresent bool
		datasource := model.DatasourceInput{
			DataSourceName:        metricPayload.DataSourceName,
			DataSourceDisplayName: metricPayload.DataSourceDisplayName,
			DataSourceGroup:       metricPayload.DataSourceGroup,
			DataSourceID:          metricPayload.DataSourceID,
		}

		if dsArray, ok := dsMap[metricPayload.ResourceName]; !ok {
			dsMap[metricPayload.ResourceName] = append([]model.DatasourceInput{}, datasource)
		} else {
			for _, ds := range dsArray {
				if ds.DataSourceName == metricPayload.DataSourceName {
					dsPresent = true
				}
			}
			if !dsPresent {
				dsMap[metricPayload.ResourceName] = append(dsArray, datasource)
			}
		}

		var instPresent bool
		instance := model.InstanceInput{
			InstanceName:        metricPayload.Instances[0].InstanceName,
			InstanceID:          metricPayload.Instances[0].InstanceID,
			InstanceDisplayName: metricPayload.Instances[0].InstanceDisplayName,
			InstanceGroup:       metricPayload.Instances[0].InstanceGroup,
			InstanceProperties:  metricPayload.Instances[0].InstanceProperties,
		}

		if instArray, ok := instanceMap[metricPayload.DataSourceName]; !ok {
			instanceMap[metricPayload.DataSourceName] = append([]model.InstanceInput{}, instance)
		} else {
			for _, ins := range instArray {
				if ins.InstanceName == metricPayload.Instances[0].InstanceName {
					instPresent = true
				}
			}
			if !instPresent {
				instanceMap[metricPayload.DataSourceName] = append(instArray, instance)
			}
		}

		var dpPresent bool
		datapoint := model.DataPointInput{
			DataPointName:            metricPayload.Instances[0].DataPoints[0].DataPointName,
			DataPointDescription:     metricPayload.Instances[0].DataPoints[0].DataPointDescription,
			DataPointType:            metricPayload.Instances[0].DataPoints[0].DataPointType,
			DataPointAggregationType: metricPayload.Instances[0].DataPoints[0].DataPointAggregationType,
			Value:                    metricPayload.Instances[0].DataPoints[0].Value,
		}

		if dpArray, ok := datapointMap[metricPayload.Instances[0].InstanceName]; !ok {
			datapointMap[metricPayload.Instances[0].InstanceName] = append([]model.DataPointInput{}, datapoint)
		} else {
			for _, dp := range dpArray {
				if dp.DataPointName == metricPayload.Instances[0].DataPoints[0].DataPointName {
					dpPresent = true
				}
			}
			if !dpPresent {
				datapointMap[metricPayload.Instances[0].InstanceName] = append(dpArray, datapoint)
			}
		}
	}
	// after merging create metric payload
	body := batch.mergeMetricPayload(resourceMap, dsMap, instanceMap, datapointMap)
	return body
}

// uri returns the endpoint/uri of metric ingest API
func (metricIngest LMMetricIngest) uri() string {
	return uri
}

// mergeMetricPayload prepares metrics payload
func (batch *metricBatch) mergeMetricPayload(resources map[string]model.ResourceInput, datasources map[string][]model.DatasourceInput, instances map[string][]model.InstanceInput, datapoints map[string][]model.DataPointInput) *LMMetricIngestRequest {

	metricsReq := &LMMetricIngestRequest{}

	for resName, resDetails := range resources {

		var payload model.MetricPayload

		payload.ResourceName = resDetails.ResourceName
		payload.ResourceID = resDetails.ResourceID
		payload.ResourceDescription = resDetails.ResourceDescription
		payload.ResourceProperties = resDetails.ResourceProperties

		if dsArray, dsExists := datasources[resName]; dsExists {

			for _, ds := range dsArray {
				payload.DataSourceName = ds.DataSourceName
				payload.DataSourceID = ds.DataSourceID
				payload.DataSourceDisplayName = ds.DataSourceDisplayName
				payload.DataSourceGroup = ds.DataSourceGroup

				instArray, _ := instances[ds.DataSourceName]
				var instances []model.Instance

				for _, instance := range instArray {
					var dataPoints []model.DataPoint
					if dpArray, exists := datapoints[instance.InstanceName]; exists {
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
						metricsReq.PayloadWithResourceCreation = append(metricsReq.PayloadWithResourceCreation, payload)
					} else {
						metricsReq.Payload = append(metricsReq.Payload, payload)
					}
				}
			}
		}
	}

	// flushing out the metric batch after exporting
	if batch.enabled {
		batch.data = nil
	}
	return metricsReq
}

// export exports metrics to the LM platform
func (metricIngest *LMMetricIngest) export(req *LMMetricIngestRequest, uri, method string) (*model.IngestResponse, error) {
	ctx := context.Background()
	var errs []error
	var respErrs []error

	cfg := client.RequestConfig{
		Client:      metricIngest.client,
		Url:         metricIngest.url,
		Uri:         uri,
		Method:      method,
		Gzip:        metricIngest.gzip,
		RateLimiter: metricIngest.rateLimiter,
	}

	apiCallResponse := &model.IngestResponse{StatusCode: http.StatusMultiStatus}

	if method == http.MethodPatch || method == http.MethodPut {
		payloadBody, err := json.Marshal(req.UpdatePropertiesPayload)
		if err != nil {
			errs = append(errs, fmt.Errorf("error in marshaling update property metric payload: %w ", err))
		}

		cfg.Token = metricIngest.auth.GetCredentials(method, uri, payloadBody)
		cfg.Body = payloadBody

		resp, err := client.DoRequest(ctx, cfg, handleMetricsExportResponse)
		if err != nil {
			errs = append(errs, fmt.Errorf("error in updating properties: %w", err))
		} else if resp != nil && (resp.StatusCode >= 400 && resp.StatusCode <= 599) {

			if resp.StatusCode == http.StatusMultiStatus {
				apiCallResponse.MultiStatus = append(apiCallResponse.MultiStatus, resp.MultiStatus...)
			} else {
				apiCallResponse.MultiStatus = append(apiCallResponse.MultiStatus, struct {
					Code  float64 `json:"code"`
					Error string  `json:"error"`
				}{
					Code:  float64(resp.StatusCode),
					Error: resp.Error.Error(),
				})
			}
			respErrs = append(respErrs, errors.New(resp.Message))
		}
	}

	if len(req.Payload) > 0 {
		payloadBody, err := json.Marshal(req.Payload)
		if err != nil {
			errs = append(errs, fmt.Errorf("error in marshaling metric payload: %w", err))
		}
		cfg.Token = metricIngest.auth.GetCredentials(method, uri, payloadBody)
		cfg.Body = payloadBody

		resp, err := client.DoRequest(ctx, cfg, handleMetricsExportResponse)
		if err != nil {
			errs = append(errs, fmt.Errorf("error while exporting metrics: %w", err))
		} else if resp != nil && (resp.StatusCode >= 400 && resp.StatusCode <= 599) {

			if resp.StatusCode == http.StatusMultiStatus {
				apiCallResponse.MultiStatus = append(apiCallResponse.MultiStatus, resp.MultiStatus...)
			} else {
				apiCallResponse.MultiStatus = append(apiCallResponse.MultiStatus, struct {
					Code  float64 `json:"code"`
					Error string  `json:"error"`
				}{
					Code:  float64(resp.StatusCode),
					Error: resp.Error.Error(),
				})
			}
			respErrs = append(respErrs, errors.New(resp.Message))
		}
	}

	if len(req.PayloadWithResourceCreation) > 0 {
		metricURI := uri + createFlag
		payloadBody, err := json.Marshal(req.PayloadWithResourceCreation)
		if err != nil {
			errs = append(errs, fmt.Errorf("error in marshaling metric payload with create flag: %w", err))
		}
		cfg.Token = metricIngest.auth.GetCredentials(method, uri, payloadBody)
		cfg.Body = payloadBody
		cfg.Uri = metricURI

		resp, err := client.DoRequest(ctx, cfg, handleMetricsExportResponse)
		if err != nil {
			errs = append(errs, fmt.Errorf("error while exporting metrics with create flag: %w", err))
		} else if resp != nil && (resp.StatusCode >= 400 && resp.StatusCode <= 599) {

			if resp.StatusCode == http.StatusMultiStatus {
				apiCallResponse.MultiStatus = append(apiCallResponse.MultiStatus, resp.MultiStatus...)
			} else {
				apiCallResponse.MultiStatus = append(apiCallResponse.MultiStatus, struct {
					Code  float64 `json:"code"`
					Error string  `json:"error"`
				}{
					Code:  float64(resp.StatusCode),
					Error: resp.Error.Error(),
				})
			}
			respErrs = append(respErrs, errors.New(resp.Message))
		}
	}

	apiCallResponse.Error = multierr.Combine(respErrs...)
	return apiCallResponse, multierr.Combine(errs...)
}

func (metricIngest *LMMetricIngest) UpdateResourceProperties(resName string, resIDs, resProps map[string]string, patch bool) (*model.IngestResponse, error) {
	if resName == "" || resIDs == nil || resProps == nil {
		return nil, fmt.Errorf("one of the fields: resource name, resource ids or resource properties, is missing")
	}
	errorMsg := ""
	errorMsg += validator.CheckResourceNameValidation(false, resName)
	errorMsg += validator.CheckResourceIDValidation(resIDs)
	errorMsg += validator.CheckResourcePropertiesValidation(resProps)

	if errorMsg != "" {
		return nil, fmt.Errorf("validation failed: %s", errorMsg)
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

	req := &LMMetricIngestRequest{UpdatePropertiesPayload: updateResProp}

	return metricIngest.export(req, updateResPropURI, method)
}

func (metricIngest *LMMetricIngest) UpdateInstanceProperties(resIDs, insProps map[string]string, dsName, dsDisplayName, insName string, patch bool) (*model.IngestResponse, error) {
	if resIDs == nil || insProps == nil || dsName == "" || insName == "" {
		return nil, fmt.Errorf("one of the fields: instance name, datasource name, resource ids, or instance properties, is missing")
	}
	errorMsg := ""
	errorMsg += validator.CheckResourceIDValidation(resIDs)
	errorMsg += validator.CheckInstancePropertiesValidation(insProps)
	errorMsg += validator.CheckInstanceNameValidation(insName)
	errorMsg += validator.CheckDataSourceNameValidation(dsName)
	errorMsg += validator.CheckDSDisplayNameValidation(dsDisplayName)

	if errorMsg != "" {
		return nil, fmt.Errorf("validation failed: %s", errorMsg)
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

	req := &LMMetricIngestRequest{UpdatePropertiesPayload: updateInsProp}
	return metricIngest.export(req, updateInsPropURI, method)
}

// handleLogsExportResponse handles the http response returned by LM platform
func handleMetricsExportResponse(ctx context.Context, resp *http.Response) (*model.IngestResponse, error) {
	defer func() {
		// Discard any remaining response body when we are done reading.
		io.CopyN(io.Discard, resp.Body, maxHTTPResponseReadBytes) // nolint:errcheck
		resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		return &model.IngestResponse{
			StatusCode: resp.StatusCode,
			Success:    true,
		}, nil
	}

	parsedResponse := readResponse(resp)

	apiCallResponse := &model.IngestResponse{StatusCode: resp.StatusCode, Success: false}

	// Format the error message. Use the status if it is present in the response.
	var formattedErr error
	if parsedResponse != nil {
		var err error

		if resp.StatusCode == http.StatusMultiStatus {
			errs := []error{}
			apiCallResponse.Message = parsedResponse.Message

			for _, responseError := range parsedResponse.Errors {
				if responseError["error"] != nil {
					apiCallResponse.MultiStatus = append(apiCallResponse.MultiStatus, struct {
						Code  float64 `json:"code"`
						Error string  `json:"error"`
					}{
						Code:  responseError["code"].(float64),
						Error: responseError["error"].(string),
					})
					errs = append(errs, fmt.Errorf("error code: [%d], error message: %s", int(responseError["code"].(float64)), responseError["error"].(string)))
				}
			}
			err = multierr.Combine(errs...)
		} else {
			err = errors.New(parsedResponse.Message)
		}

		formattedErr = fmt.Errorf(
			"error exporting items, request to %s responded with HTTP Status Code %d, Message: %s, Details: %s",
			resp.Request.URL, resp.StatusCode, parsedResponse.Message, err.Error())
	} else {
		formattedErr = fmt.Errorf(
			"error exporting items, request to %s responded with HTTP Status Code %d",
			resp.Request.URL, resp.StatusCode)
	}
	apiCallResponse.Error = formattedErr

	retryAfter := 0
	if val := resp.Header.Get(headerRetryAfter); val != "" {
		if seconds, err2 := strconv.Atoi(val); err2 == nil {
			retryAfter = seconds
		}
	}
	apiCallResponse.RetryAfter = retryAfter

	return apiCallResponse, nil
}

// Read the response and decode the status.Status from the body.
// Returns nil if the response is empty or cannot be decoded.
func readResponse(resp *http.Response) *LMMetricIngestResponse {
	var lmMetricIngestResponse *LMMetricIngestResponse
	if resp.StatusCode == 207 || resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		// Request failed. Read the body.
		maxRead := resp.ContentLength
		if maxRead == -1 || maxRead > maxHTTPResponseReadBytes {
			maxRead = maxHTTPResponseReadBytes
		}
		respBytes := make([]byte, maxRead)
		n, err := io.ReadFull(resp.Body, respBytes)
		if err == nil && n > 0 {
			lmMetricIngestResponse = &LMMetricIngestResponse{}
			err = json.Unmarshal(respBytes, lmMetricIngestResponse)
			if err != nil {
				lmMetricIngestResponse = nil
			}
		}
	}
	return lmMetricIngestResponse
}
