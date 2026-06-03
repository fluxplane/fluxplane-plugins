module github.com/fluxplane/fluxplane-plugins/clock

go 1.26.1

require (
	github.com/fluxplane/fluxplane-context v0.0.0
	github.com/fluxplane/fluxplane-plugin v0.0.0
)

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.2.0 // indirect
	github.com/fluxplane/fluxplane-auth v0.3.0 // indirect
	github.com/fluxplane/fluxplane-datasource v0.1.0 // indirect
	github.com/fluxplane/fluxplane-endpoint v0.2.0 // indirect
	github.com/fluxplane/fluxplane-event v0.2.0 // indirect
	github.com/fluxplane/fluxplane-operation v0.1.0 // indirect
	github.com/fluxplane/fluxplane-policy v0.1.1 // indirect
	github.com/fluxplane/fluxplane-secret v0.2.0 // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.4 // indirect
)

replace github.com/fluxplane/fluxplane-auth v0.3.0 => ../../fluxplane-auth

replace github.com/fluxplane/fluxplane-context v0.0.0 => ../../fluxplane-context

replace github.com/fluxplane/fluxplane-datasource v0.1.0 => ../../fluxplane-datasource

replace github.com/fluxplane/fluxplane-endpoint v0.2.0 => ../../fluxplane-endpoint

replace github.com/fluxplane/fluxplane-event v0.2.0 => ../../fluxplane-event

replace github.com/fluxplane/fluxplane-operation v0.1.0 => ../../fluxplane-operation

replace github.com/fluxplane/fluxplane-plugin v0.0.0 => ../../fluxplane-plugin

replace github.com/fluxplane/fluxplane-policy v0.1.1 => ../../fluxplane-policy

replace github.com/fluxplane/fluxplane-secret v0.2.0 => ../../fluxplane-secret
