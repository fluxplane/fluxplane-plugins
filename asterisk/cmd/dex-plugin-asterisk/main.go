package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/asterisk"
)

func main() {
	pluginbinding.Serve(asterisk.NewPlugin())
}
