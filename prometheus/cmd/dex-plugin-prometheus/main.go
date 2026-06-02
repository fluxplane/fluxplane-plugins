package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/prometheus"
)

func main() {
	pluginbinding.Serve(prometheus.NewPlugin())
}
