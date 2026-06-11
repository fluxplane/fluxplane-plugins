package alertmanager

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
		pluginbinding.RegisterOperation(alertsSpec(), service.Alerts),
		pluginbinding.RegisterOperation(silenceListSpec(), service.SilenceList),
		pluginbinding.RegisterOperation(silenceCreateSpec(), service.SilenceCreate),
		pluginbinding.RegisterOperation(silenceDeleteSpec(), service.SilenceDelete),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
