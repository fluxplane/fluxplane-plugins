package alertmanager

import (
	"encoding/json"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

// withInputExamples injects JSON Schema `examples` into an operation's input
// schema. The fluxplane-plugin CLI surfaces the first example as the runnable
// invocation in `operation describe`. Kept local to the alertmanager plugin
// rather than promoted to the SDK.
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

const (
	PluginName        = "alertmanager"
	PluginVersion     = "0.1.0"
	PluginDescription = "Alertmanager active alerts and silence management (list, create, delete) against registered endpoints."

	EnvAlertmanagerUsername = "ALERTMANAGER_USERNAME"
	EnvAlertmanagerPassword = "ALERTMANAGER_PASSWORD"

	AuthPurposeBasicUsername = "basic_username"
	AuthPurposeBasicPassword = "basic_password"

	OperationTest          = "alertmanager.test"
	OperationAlerts        = "alertmanager.alerts"
	OperationSilenceList   = "alertmanager.silence.list"
	OperationSilenceCreate = "alertmanager.silence.create"
	OperationSilenceDelete = "alertmanager.silence.delete"

	EndpointAlertmanager = "alertmanager.endpoints"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"am", PluginName},
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Operations: []core.OperationSpec{
			testSpec(),
			alertsSpec(),
			silenceListSpec(),
			silenceCreateSpec(),
			silenceDeleteSpec(),
		},
		Auth: []core.AuthMethod{{
			Name: "endpoint",
			Kind: "config",
			Description: "All fields are optional and resolve from the persisted secret store at call time. " +
				"Setup: 1) register the Alertmanager URL: fluxplane-plugin endpoint save alertmanager-prod http://127.0.0.1:19093 --product alertmanager " +
				"(in-cluster instances: kubernetes.portforward.start first)  " +
				"2) store basic-auth credentials only if the instance needs them: fluxplane-plugin auth connect alertmanager  " +
				"3) verify: fluxplane-plugin operation invoke alertmanager alertmanager.test --arg endpoint_ref=alertmanager-prod. " +
				"Environment variables are read once during auth auto as setup hints, never at invoke time.",
			Env: []string{EnvAlertmanagerUsername, EnvAlertmanagerPassword},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeBasicUsername, "HTTP basic auth username", false, false, EnvAlertmanagerUsername),
				pluginbinding.AuthField(AuthPurposeBasicPassword, "HTTP basic auth password", false, true, EnvAlertmanagerPassword),
			},
		}},
		Endpoints: []core.EndpointSpec{{
			Name:        EndpointAlertmanager,
			Description: "Configured Alertmanager API endpoint.",
			Products:    []string{PluginName},
		}},
	}
}

func testSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[TestInput, TestResult](OperationTest,
			"Test Alertmanager readiness and report version/cluster status.",
			readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "alertmanager-prod"},
	)
}

func alertsSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[AlertsInput, AlertsResult](OperationAlerts,
			"List alerts currently known to Alertmanager with state filters (active/silenced/inhibited) and label matchers — what is firing right now, after routing.",
			readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "alertmanager-prod", "filter": []string{`severity="critical"`}, "silenced": false},
	)
}

func silenceListSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[SilenceListInput, SilenceListResult](OperationSilenceList,
			"List silences with their matchers, state (active/pending/expired), creator, and comment.",
			readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "alertmanager-prod", "state": "active"},
	)
}

func silenceCreateSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[SilenceCreateInput, SilenceCreateResult](OperationSilenceCreate,
			"Create a silence: label matchers, duration, creator, and comment. The pager-storm tool — silence the noisy alert, keep triaging.",
			writeOptions()...),
		map[string]any{"endpoint_ref": "alertmanager-dev", "matchers": []map[string]any{{"name": "alertname", "value": "HighErrorRate"}}, "duration": "2h", "comment": "silenced during incident triage", "created_by": "fluxplane"},
	)
}

func silenceDeleteSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[SilenceDeleteInput, SilenceDeleteResult](OperationSilenceDelete,
			"Expire (delete) a silence by id.",
			writeOptions()...),
		map[string]any{"endpoint_ref": "alertmanager-dev", "id": "9b7f1c..."},
	)
}

func readOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(AuthPurposeBasicUsername, AuthPurposeBasicPassword),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func writeOptions() []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(AuthPurposeBasicUsername, AuthPurposeBasicPassword),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	}
}
