package aws

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	spec := manifestSpec()
	return pluginbinding.Define(spec,
		pluginbinding.RegisterOperation(inspectSpec(), Inspect),
		pluginbinding.RegisterContextProvider(spec.Context[0], BuildContext),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
