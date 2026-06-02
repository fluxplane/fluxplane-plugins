package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	gitlabplugin "github.com/fluxplane/fluxplane-plugins/gitlab"
)

func main() {
	pluginbinding.Serve(gitlabplugin.NewPlugin())
}
