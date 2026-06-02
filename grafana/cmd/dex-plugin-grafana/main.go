package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/grafana"
)

func main() {
	pluginbinding.Serve(grafana.NewPlugin())
}
