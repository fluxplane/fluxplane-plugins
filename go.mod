module github.com/fluxplane/fluxplane-plugins

go 1.26.1

require github.com/fluxplane/fluxplane-plugin v0.0.0

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.2.0 // indirect
	github.com/fluxplane/fluxplane-auth v0.3.0 // indirect
	github.com/fluxplane/fluxplane-context v0.0.0 // indirect
	github.com/fluxplane/fluxplane-datasource v0.1.0 // indirect
	github.com/fluxplane/fluxplane-endpoint v0.2.0 // indirect
	github.com/fluxplane/fluxplane-event v0.2.0 // indirect
	github.com/fluxplane/fluxplane-operation v0.1.0 // indirect
	github.com/fluxplane/fluxplane-policy v0.1.1 // indirect
	github.com/fluxplane/fluxplane-secret v0.2.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.4 // indirect
)

replace github.com/fluxplane/fluxplane-plugin v0.0.0 => ../fluxplane-plugin

replace github.com/fluxplane/fluxplane-context v0.0.0 => ../fluxplane-context

replace github.com/fluxplane/fluxplane-operation v0.1.0 => ../fluxplane-operation
