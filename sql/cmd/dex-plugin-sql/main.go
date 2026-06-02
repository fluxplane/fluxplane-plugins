package main

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/sql"
)

func main() {
	pluginbinding.Serve(sql.NewPlugin())
}
