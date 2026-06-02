package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	duckduckgoplugin "github.com/fluxplane/fluxplane-plugins/duckduckgo"
)

func main() {
	pluginbinding.Serve(duckduckgoplugin.NewPlugin())
}
