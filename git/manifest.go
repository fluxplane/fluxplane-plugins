package git

import (
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

const (
	PluginName        = "git"
	PluginVersion     = "0.1.0"
	PluginDescription = "Local Git repository inspection and write operations through the host process boundary."

	OperationStatus = "git_status"
	OperationDiff   = "git_diff"
	OperationAdd    = "git_add"
	OperationCommit = "git_commit"
	OperationTag    = "git_tag"
	OperationPush   = "git_push"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"git"},
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Operations: []core.OperationSpec{
			statusSpec(),
			diffSpec(),
			addSpec(),
			commitSpec(),
			tagSpec(),
			pushSpec(),
		},
	}
}

func statusSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[StatusInput, GitResult](
		OperationStatus,
		"Show git status for the workspace.",
		pluginbinding.ReadOnly(),
		pluginbinding.Compact(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	)
}

func diffSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[DiffInput, GitResult](
		OperationDiff,
		"Show git diff for the workspace, with optional compact stat/name views and bounded output.",
		pluginbinding.ReadOnly(),
		pluginbinding.Compact(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	)
}

func addSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AddInput, GitResult](
		OperationAdd,
		"Stage git workspace changes.",
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	)
}

func commitSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[CommitInput, GitResult](
		OperationCommit,
		"Create a git commit from staged changes, optionally staging paths first.",
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	)
}

func tagSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TagInput, GitResult](
		OperationTag,
		"Create a lightweight or annotated git tag.",
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	)
}

func pushSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PushInput, GitResult](
		OperationPush,
		"Push explicit git refspecs or tags to a configured remote.",
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectWrite, core.OperationEffectNetwork, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	)
}
