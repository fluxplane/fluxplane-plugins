package aws

import (
	fpcontext "github.com/fluxplane/fluxplane-context"
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

const (
	PluginName        = "aws"
	PluginVersion     = "0.1.0"
	PluginDescription = "AWS environment configuration and credential presence inspection."

	OperationInspect = "aws.environment.inspect"
	ContextName      = "aws.environment"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{PluginName},
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Operations:  []core.OperationSpec{inspectSpec()},
		Context: []core.ContextSpec{{
			Name:             ContextName,
			Description:      "Non-secret AWS profile, region, and credential presence.",
			Kinds:            []fpcontext.BlockKind{fpcontext.BlockText, fpcontext.BlockData},
			DefaultPlacement: fpcontext.PlacementSystem,
		}},
	}
}

func inspectSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InspectInput, Environment](
		OperationInspect,
		"Inspect non-secret AWS environment configuration and credential presence.",
		pluginbinding.ReadOnly(),
		pluginbinding.Compact(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	)
}
