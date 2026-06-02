package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/loki"
)

func main() {
	pluginbinding.Serve(loki.NewPlugin())
}
