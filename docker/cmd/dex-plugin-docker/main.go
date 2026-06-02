package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/docker"
)

func main() {
	pluginbinding.Serve(docker.NewPlugin())
}
