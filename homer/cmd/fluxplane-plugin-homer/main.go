package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/homer"
)

func main() {
	pluginbinding.Serve(homer.NewPlugin())
}
