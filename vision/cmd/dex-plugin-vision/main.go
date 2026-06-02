package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/vision"
)

func main() {
	pluginbinding.Serve(vision.NewPlugin())
}
