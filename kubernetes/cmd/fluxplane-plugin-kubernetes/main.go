package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/kubernetes"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == kubernetes.PortForwardHelperCommand() {
		if err := kubernetes.RunKubePortForwardCommand(context.Background(), os.Args[2:], os.Stdout, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	pluginbinding.Serve(kubernetes.NewPlugin())
}
