package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/aws"
)

func main() {
	pluginbinding.Serve(aws.NewPlugin())
}
