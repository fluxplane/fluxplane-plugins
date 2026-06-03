package git

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return pluginbinding.Define(manifestSpec(),
		pluginbinding.RegisterOperation(statusSpec(), Status),
		pluginbinding.RegisterOperation(diffSpec(), Diff),
		pluginbinding.RegisterOperation(addSpec(), Add),
		pluginbinding.RegisterOperation(commitSpec(), Commit),
		pluginbinding.RegisterOperation(tagSpec(), Tag),
		pluginbinding.RegisterOperation(pushSpec(), Push),
	)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
