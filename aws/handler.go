package aws

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	spec := manifestSpec()
	return pluginbinding.Define(spec,
		pluginbinding.WithAuthTestOperation(OperationTest),
		pluginbinding.RegisterOperation(testSpec(), service.Test),
		pluginbinding.RegisterOperation(inspectSpec(), Inspect),
		pluginbinding.RegisterOperation(ec2InstancesSpec(), service.EC2Instances),
		pluginbinding.RegisterOperation(eksClustersSpec(), service.EKSClusters),
		pluginbinding.RegisterOperation(rdsInstancesSpec(), service.RDSInstances),
		pluginbinding.RegisterOperation(s3BucketsSpec(), service.S3Buckets),
		pluginbinding.RegisterOperation(s3ObjectsSpec(), service.S3Objects),
		pluginbinding.RegisterOperation(logsGroupsSpec(), service.LogsGroups),
		pluginbinding.RegisterOperation(logsTailSpec(), service.LogsTail),
		pluginbinding.RegisterOperation(logsQuerySpec(), service.LogsQuery),
		pluginbinding.RegisterOperation(cloudWatchMetricsSpec(), service.CloudWatchMetrics),
		pluginbinding.RegisterDatasourceSearch(ec2DatasourceSpec(), service.EC2Search),
		pluginbinding.RegisterContextProvider(spec.Context[0], BuildContext),
		pluginbinding.RegisterEvidenceObserver(environmentObserverSpec(), Observe),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
