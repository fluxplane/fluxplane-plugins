package sql

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
		pluginbinding.RegisterOperation(databaseListSpec(), service.DatabaseList),
		pluginbinding.RegisterOperation(tableListSpec(), service.TableList),
		pluginbinding.RegisterOperation(tableShowSpec(), service.TableShow),
		pluginbinding.RegisterOperation(indexListSpec(), service.IndexList),
		pluginbinding.RegisterDatasourceSearch(queryRowsDatasourceSpec(), service.QueryRows),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
