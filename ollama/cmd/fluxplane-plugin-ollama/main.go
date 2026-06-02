package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	ollamaplugin "github.com/fluxplane/fluxplane-plugins/ollama"
)

func main() {
	pluginbinding.Serve(ollamaplugin.NewPlugin())
}
