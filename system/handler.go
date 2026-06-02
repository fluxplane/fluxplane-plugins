package system

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return pluginbinding.Define(manifestSpec(),
		pluginbinding.RegisterOperation(infoSpec(), Info),
		pluginbinding.RegisterContextProvider(contextSpec(), BuildContext),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
