// Package openapi generates SDK plugin operations and documentation datasources
// from OpenAPI 3.x specifications.
package openapi

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	coredatasource "github.com/fluxplane/fluxplane-datasource"
	"github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

const (
	PluginName        = Name
	PluginVersion     = "0.1.0"
	PluginDescription = "OpenAPI-generated operations and documentation datasources."
)

type Options struct {
	Root     string
	Instance string
}

func NewPlugin(ctx context.Context, cfg Config, opts Options) (*pluginbinding.Plugin, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg = normalizeConfig(cfg)
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	loaded, errs := loadSpecs(ctx, abs, cfg)
	if len(errs) > 0 {
		return nil, errs[0]
	}
	generated, err := generateAll(strings.TrimSpace(opts.Instance), loaded)
	if err != nil {
		return nil, err
	}
	plugin := pluginbinding.Define(pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Operations:  generated.Operations,
		Auth:        generated.AuthMethods,
		Datasources: generated.Datasources,
	})
	for _, def := range generated.Executable {
		def := def
		pluginbinding.Operation[map[string]any, OperationResult](plugin, def.Spec, func(ctx pluginbinding.Context, input map[string]any) (OperationResult, error) {
			return runOperation(ctx, def, input)
		})
	}
	for _, datasource := range generated.Datasources {
		datasourceName := datasource.Name
		for _, entity := range datasourceEntities(datasource) {
			spec := datasource
			spec.Entity = string(entity)
			spec.Capabilities = []string{pluginbinding.CapabilitySearch, pluginbinding.CapabilityList, pluginbinding.CapabilityGet}
			pluginbinding.DatasourceHandlerFor[pluginbinding.DatasourceSearchInput, pluginbinding.DatasourceSearchResult[DocRecord]](plugin, spec, pluginbinding.CapabilitySearch, func(_ pluginbinding.Context, input pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[DocRecord], error) {
				records, err := searchDocs(generated.Docs, input)
				if err != nil {
					return pluginbinding.DatasourceSearchResult[DocRecord]{}, err
				}
				return pluginbinding.NewDatasourceSearchResult(input.Datasource, input.Query, records), nil
			})
			pluginbinding.DatasourceHandlerFor[pluginbinding.DatasourceListInput, pluginbinding.DatasourceListResult](plugin, spec, pluginbinding.CapabilityList, func(_ pluginbinding.Context, input pluginbinding.DatasourceListInput) (pluginbinding.DatasourceListResult, error) {
				records, err := listDocs(generated.Docs, input)
				if err != nil {
					return pluginbinding.DatasourceListResult{}, err
				}
				return pluginbinding.DatasourceListResult{Datasource: coredatasource.Name(datasourceName), Entity: coredatasource.EntityType(input.Entity), Records: docRecordsToAny(records), Total: len(records), Complete: true}, nil
			})
			pluginbinding.DatasourceHandlerFor[pluginbinding.DatasourceGetInput, pluginbinding.DatasourceGetResult[DocRecord]](plugin, spec, pluginbinding.CapabilityGet, func(_ pluginbinding.Context, input pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[DocRecord], error) {
				record, err := getDoc(generated.Docs, input)
				if err != nil {
					return pluginbinding.DatasourceGetResult[DocRecord]{}, err
				}
				return pluginbinding.NewDatasourceGetResult(input.Datasource, record), nil
			})
		}
	}
	return plugin, nil
}

func MustNewPlugin(ctx context.Context, cfg Config, opts Options) *pluginbinding.Plugin {
	plugin, err := NewPlugin(ctx, cfg, opts)
	if err != nil {
		panic(err)
	}
	return plugin
}

func ManifestFor(ctx context.Context, cfg Config, opts Options) (manifest.PluginManifest, error) {
	plugin, err := NewPlugin(ctx, cfg, opts)
	if err != nil {
		return manifest.PluginManifest{}, err
	}
	return plugin.Manifest(), nil
}

func Handle(req protocol.Request) protocol.Response {
	return protocol.Fail("configuration_required", fmt.Sprintf("%s plugin requires host application configuration", PluginName))
}
