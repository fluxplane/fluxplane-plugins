package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/kubernetes"
)

func main() {
	pluginbinding.Serve(kubernetes.NewPlugin())
}
