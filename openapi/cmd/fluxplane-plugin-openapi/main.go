package main

import (
	"github.com/fluxplane/fluxplane-plugin/protocol"
	openapiplugin "github.com/fluxplane/fluxplane-plugins/openapi"
)

func main() {
	protocol.Serve(func(req protocol.Request, _ protocol.HostCaller) protocol.Response {
		return openapiplugin.Handle(req)
	})
}
