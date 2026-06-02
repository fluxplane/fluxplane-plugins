package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	confluenceplugin "github.com/fluxplane/fluxplane-plugins/atlassian/confluence"
)

func main() {
	pluginbinding.Serve(confluenceplugin.NewPlugin())
}
