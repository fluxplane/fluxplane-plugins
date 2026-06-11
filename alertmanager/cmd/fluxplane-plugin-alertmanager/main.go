package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/alertmanager"
)

func main() {
	pluginbinding.Serve(alertmanager.NewPlugin())
}
