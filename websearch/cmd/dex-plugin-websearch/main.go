package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/websearch"
)

func main() {
	pluginbinding.Serve(websearch.NewPlugin())
}
