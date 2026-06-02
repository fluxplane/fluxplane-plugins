package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	slackplugin "github.com/fluxplane/fluxplane-plugins/slack"
)

func main() {
	pluginbinding.Serve(slackplugin.NewPlugin())
}
