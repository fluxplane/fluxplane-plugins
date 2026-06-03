package sleep

import (
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

const (
	PluginName        = "sleep"
	PluginVersion     = "0.1.0"
	PluginDescription = "Interruptible local wait operation."

	OperationSleep = "sleep"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Operations:  []core.OperationSpec{sleepSpec()},
	}
}

func sleepSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[Input, Output](
		OperationSleep,
		"Sleep for a duration in seconds without spawning a shell process. The wait is interruptible by cancellation.",
		pluginbinding.ReadOnly(),
		pluginbinding.Compact(),
		pluginbinding.Effects(core.OperationEffect("none")),
		pluginbinding.Access(core.OperationAccessNone),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	)
}
