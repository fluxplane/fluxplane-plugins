package clock

import (
	fpcontext "github.com/fluxplane/fluxplane-context"
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

const (
	PluginName          = "clock"
	PluginVersion       = "0.1.0"
	PluginDescription   = "Current wall-clock time context provider."
	ContextProviderName = "time"
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
		Context: []core.ContextSpec{{
			Name:             ContextProviderName,
			Description:      "Current wall-clock time.",
			Kinds:            []fpcontext.BlockKind{fpcontext.BlockData},
			DefaultPlacement: fpcontext.PlacementSystem,
		}},
	}
}
