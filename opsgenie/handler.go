package opsgenie

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
		pluginbinding.RegisterOperation(alertListSpec(), service.AlertList),
		pluginbinding.RegisterOperation(alertGetSpec(), service.AlertGet),
		pluginbinding.RegisterOperation(alertAckSpec(), service.AlertAck),
		pluginbinding.RegisterOperation(alertCloseSpec(), service.AlertClose),
		pluginbinding.RegisterOperation(alertNoteSpec(), service.AlertNote),
		pluginbinding.RegisterOperation(onCallSpec(), service.OnCall),
		pluginbinding.RegisterOperation(scheduleListSpec(), service.ScheduleList),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
