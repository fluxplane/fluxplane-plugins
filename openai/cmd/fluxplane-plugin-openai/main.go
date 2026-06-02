package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	openaiplugin "github.com/fluxplane/fluxplane-plugins/openai"
)

func main() {
	pluginbinding.Serve(openaiplugin.NewPlugin())
}
