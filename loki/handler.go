package loki

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	return pluginbinding.Define(manifestSpec(),
		pluginbinding.RegisterOperation(testSpec(), service.Test),
		pluginbinding.RegisterOperation(querySpec(), service.Query),
		pluginbinding.RegisterOperation(metricSpec(), service.Metric),
		pluginbinding.RegisterOperation(labelsSpec(), service.Labels),
		pluginbinding.RegisterOperation(recentLogsSpec(), service.RecentLogs),
		pluginbinding.RegisterDatasourceSearch(logEntriesDatasourceSpec(), service.LogEntriesDatasource),
		pluginbinding.RegisterDatasourceSearch(labelsDatasourceSpec(), service.LabelsDatasource),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
