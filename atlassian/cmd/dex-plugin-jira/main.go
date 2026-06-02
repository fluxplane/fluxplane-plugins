package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	jiraplugin "github.com/fluxplane/fluxplane-plugins/atlassian/jira"
)

func main() {
	pluginbinding.Serve(jiraplugin.NewPlugin())
}
