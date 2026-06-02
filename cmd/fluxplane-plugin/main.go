package main

import (
	"fmt"
	"os"

	plugincli "github.com/fluxplane/fluxplane-plugin/cli"
	"github.com/fluxplane/fluxplane-plugin/management/local"
	registry "github.com/fluxplane/fluxplane-plugins"
)

func main() {
	backend, err := local.New(local.WithMarketplace(registry.Marketplace()))
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cmd := plugincli.New(plugincli.Options{Backend: backend, Out: os.Stdout, Err: os.Stderr})
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
