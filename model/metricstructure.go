package model

type ResourceInput struct {
	ResourceName        string
	ResourceDescription string
	ResourceID          map[string]string
	ResourceProperties  map[string]string
	IsCreate            bool
}

type DatasourceInput struct {
	DataSourceName        string
	DataSourceDisplayName string
	DataSourceGroup       string
	DataSourceID          int
}

type InstanceInput struct {
	InstanceName        string
	InstanceID          int
	InstanceDisplayName string
	InstanceGroup       string
	InstanceProperties  map[string]string
}

type DataPointInput struct {
	DataPointName            string
	DataPointType            string
	DataPointDescription     string
	DataPointAggregationType string
	Value                    map[string]string
}
