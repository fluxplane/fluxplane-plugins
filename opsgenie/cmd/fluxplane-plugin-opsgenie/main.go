package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/opsgenie"
)

func main() {
	pluginbinding.Serve(opsgenie.NewPlugin())
}
