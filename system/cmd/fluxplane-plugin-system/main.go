package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	systemplugin "github.com/fluxplane/fluxplane-plugins/system"
)

func main() {
	pluginbinding.Serve(systemplugin.NewPlugin())
}
