package git

import (
	"encoding/json"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

const (
	PluginName        = "git"
	PluginVersion     = "0.2.0"
	PluginDescription = "Local Git repository inspection and write operations through the host process boundary."

	OperationStatus = "git_status"
	OperationDiff   = "git_diff"
	OperationAdd    = "git_add"
	OperationCommit = "git_commit"
	OperationTag    = "git_tag"
	OperationPush   = "git_push"
)

// withInputExamples injects JSON Schema `examples` into an operation's input
// schema. The fluxplane-plugin CLI surfaces the first example as the runnable
// invocation in `operation describe`. Kept local to the git plugin rather than
// promoted to the SDK.
func withInputExamples(spec core.OperationSpec, examples ...map[string]any) core.OperationSpec {
	if len(examples) == 0 || len(spec.Input) == 0 {
		return spec
	}
	var schema map[string]any
	if err := json.Unmarshal(spec.Input, &schema); err != nil {
		return spec
	}
	arr := make([]any, 0, len(examples))
	for _, example := range examples {
		arr = append(arr, example)
	}
	schema["examples"] = arr
	if raw, err := json.Marshal(schema); err == nil {
		spec.Input = raw
	}
	return spec
}

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
	return withInputExamples(pluginbinding.TypedOperationSpec[StatusInput, GitResult](
		OperationStatus,
		"Show git status for the workspace.",
		pluginbinding.ReadOnly(),
		pluginbinding.Compact(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	), map[string]any{})
}

func diffSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[DiffInput, GitResult](
		OperationDiff,
		"Show git diff for the workspace, with optional compact stat/name views and bounded output.",
		pluginbinding.ReadOnly(),
		pluginbinding.Compact(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	), map[string]any{"staged": true, "stat_only": true, "paths": []any{"src/"}})
}

func addSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[AddInput, GitResult](
		OperationAdd,
		"Stage git workspace changes.",
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	), map[string]any{"paths": []any{"README.md"}})
}

func commitSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[CommitInput, GitResult](
		OperationCommit,
		"Create a git commit from staged changes, optionally staging paths first.",
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	), map[string]any{"message": "fix: handle empty refspec", "stage": true, "paths": []any{"operations.go"}})
}

func tagSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[TagInput, GitResult](
		OperationTag,
		"Create a lightweight or annotated git tag.",
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	), map[string]any{"name": "v0.3.0", "message": "release v0.3.0"})
}

func pushSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[PushInput, GitResult](
		OperationPush,
		"Push explicit git refspecs or tags to a configured remote.",
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectWrite, core.OperationEffectNetwork, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	), map[string]any{"refspecs": []any{"HEAD:refs/heads/main"}, "set_upstream": true, "dry_run": true})
}
