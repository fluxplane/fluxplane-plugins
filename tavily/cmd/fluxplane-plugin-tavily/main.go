package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	tavilyplugin "github.com/fluxplane/fluxplane-plugins/tavily"
)

func main() {
	pluginbinding.Serve(tavilyplugin.NewPlugin())
}
