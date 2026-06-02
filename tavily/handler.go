package tavily

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
	"github.com/fluxplane/fluxplane-plugins/websearch"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	return websearch.DefineProvider(providerSpec(), service.Search)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
