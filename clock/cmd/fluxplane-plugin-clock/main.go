package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	clockplugin "github.com/fluxplane/fluxplane-plugins/clock"
)

func main() {
	pluginbinding.Serve(clockplugin.NewPlugin())
}
