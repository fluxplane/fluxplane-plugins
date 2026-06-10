package homer

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
		pluginbinding.RegisterOperation(searchSpec(), service.Search),
		pluginbinding.RegisterOperation(callListSpec(), service.CallList),
		pluginbinding.RegisterOperation(callShowSpec(), service.CallShow),
		pluginbinding.RegisterOperation(callQoSSpec(), service.CallQoS),
		pluginbinding.RegisterOperation(callAnalyzeSpec(), service.CallAnalyze),
		pluginbinding.RegisterOperation(pcapExportSpec(), service.PCAPExport),
		pluginbinding.RegisterOperation(aliasListSpec(), service.AliasList),
		pluginbinding.RegisterDatasourceSearch(callsDatasourceSpec(), service.CallsDatasource),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
