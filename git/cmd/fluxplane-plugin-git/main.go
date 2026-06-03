package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	gitplugin "github.com/fluxplane/fluxplane-plugins/git"
)

func main() {
	pluginbinding.Serve(gitplugin.NewPlugin())
}
