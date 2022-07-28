package model

import "time"

type DataPayload struct {
	MetricBodyList           []MetricPayload
	MetricResourceCreateList []MetricPayload
	LogBodyList              []LogPayload
	UpdatePropertiesBody     UpdateProperties
}

type LMIngest interface {
	BatchInterval() time.Duration
	URI() string
	CreateRequestBody() DataPayload
	ExportData(body DataPayload, uri, method string) error
}
