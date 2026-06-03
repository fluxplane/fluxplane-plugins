package aws

import (
	fpcontext "github.com/fluxplane/fluxplane-context"
	evidence "github.com/fluxplane/fluxplane-evidence"
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

const (
	PluginName        = "aws"
	PluginVersion     = "0.2.0"
	PluginDescription = "AWS environment configuration and credential presence inspection."

	OperationInspect                 = "aws.environment.inspect"
	ContextName                      = "aws.environment"
	ObserverEnvironment              = "aws.environment"
	ObservationEnvironmentConfigured = "aws.environment.configured"
	ObservationEnvironmentAvailable  = "aws.environment.available"
	AssertionConfigured              = "integration.configured"
	AssertionAvailable               = "integration.available"
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
		Observers:         []core.ObserverSpec{environmentObserverSpec()},
		AssertionDerivers: []core.AssertionDeriverSpec{configuredAssertionDeriverSpec(), availableAssertionDeriverSpec()},
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

func environmentObserverSpec() core.ObserverSpec {
	return core.ObserverSpec{
		Name:        ObserverEnvironment,
		Description: "Observes non-secret AWS environment configuration and credential presence.",
		Environment: evidence.Ref{
			Name: evidence.Name(PluginName),
		},
		Phase: core.ObservationPhaseTurn,
		ObservableKinds: []string{
			ObservationEnvironmentConfigured,
			ObservationEnvironmentAvailable,
		},
		Dynamic: true,
	}
}

func configuredAssertionDeriverSpec() core.AssertionDeriverSpec {
	return core.AssertionDeriverSpec{
		Name:             "aws.environment.configured",
		Description:      "Derives AWS integration configuration from non-secret environment evidence.",
		ObservationKinds: []string{ObservationEnvironmentConfigured},
		Assertions: []core.AssertionTemplate{{
			Kind:    AssertionConfigured,
			Target:  PluginName,
			Subject: evidence.Subject{Kind: evidence.SubjectIntegration, Name: PluginName},
		}},
	}
}

func availableAssertionDeriverSpec() core.AssertionDeriverSpec {
	return core.AssertionDeriverSpec{
		Name:             "aws.environment.available",
		Description:      "Derives AWS integration availability from non-secret environment evidence.",
		ObservationKinds: []string{ObservationEnvironmentAvailable},
		Assertions: []core.AssertionTemplate{{
			Kind:    AssertionAvailable,
			Target:  PluginName,
			Subject: evidence.Subject{Kind: evidence.SubjectIntegration, Name: PluginName},
		}},
	}
}
