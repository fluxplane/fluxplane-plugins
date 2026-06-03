package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	sleepplugin "github.com/fluxplane/fluxplane-plugins/sleep"
)

func main() {
	pluginbinding.Serve(sleepplugin.NewPlugin())
}
