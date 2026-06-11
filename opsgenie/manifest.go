package opsgenie

import (
	"encoding/json"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

// withInputExamples injects JSON Schema `examples` into an operation's input
// schema. Kept local to the opsgenie plugin rather than promoted to the SDK.
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
	PluginName        = "opsgenie"
	PluginVersion     = "0.1.0"
	PluginDescription = "Opsgenie paging visibility and ack loop: alerts, acknowledge/close/note, schedules, and who is on call."

	EnvOpsgenieAPIKey = "OPSGENIE_API_KEY"
	EnvOpsgenieAPIURL = "OPSGENIE_API_URL"

	AuthPurposeAPIKey = "api_key"

	// DefaultAPIURL is the EU region host; register an endpoint or set
	// api_url for other regions.
	DefaultAPIURL = "https://api.eu.opsgenie.com"

	OperationTest         = "opsgenie.test"
	OperationAlertList    = "opsgenie.alert.list"
	OperationAlertGet     = "opsgenie.alert.get"
	OperationAlertAck     = "opsgenie.alert.ack"
	OperationAlertClose   = "opsgenie.alert.close"
	OperationAlertNote    = "opsgenie.alert.note"
	OperationOnCall       = "opsgenie.oncall"
	OperationScheduleList = "opsgenie.schedule.list"

	EndpointOpsgenie = "opsgenie.endpoints"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"og", PluginName},
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Operations: []core.OperationSpec{
			testSpec(),
			alertListSpec(),
			alertGetSpec(),
			alertAckSpec(),
			alertCloseSpec(),
			alertNoteSpec(),
			onCallSpec(),
			scheduleListSpec(),
		},
		Auth: []core.AuthMethod{{
			Name: "api_key",
			Kind: "bearer_token",
			Description: "Opsgenie API key (GenieKey) resolved from the persisted secret store at call time. " +
				"Setup: 1) create an API key in Opsgenie (Settings → API key management, or a team integration key with read+ack rights)  " +
				"2) store it: fluxplane-plugin auth connect opsgenie --field api_key=<key>  " +
				"3) verify: fluxplane-plugin operation invoke opsgenie opsgenie.test. " +
				"The EU API host (" + DefaultAPIURL + ") is the default; register an endpoint with product opsgenie to override. " +
				"Environment variables are read once during auth auto as setup hints, never at invoke time.",
			Env: []string{EnvOpsgenieAPIKey},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeAPIKey, "Opsgenie API key (GenieKey)", true, true, EnvOpsgenieAPIKey),
			},
		}},
		Endpoints: []core.EndpointSpec{{
			Name:        EndpointOpsgenie,
			Description: "Opsgenie API endpoint (defaults to the EU region host).",
			Products:    []string{PluginName},
			Env:         []string{EnvOpsgenieAPIURL},
		}},
	}
}

func testSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TestInput, TestResult](OperationTest,
		"Validate the stored Opsgenie API key and report the account it belongs to.",
		readOptions(core.OperationIdempotent)...)
}

func alertListSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[AlertListInput, AlertListResult](OperationAlertList,
			"List Opsgenie alerts (newest first) with the Opsgenie query language — status, priority, tags, time ranges.",
			readOptions(core.OperationIdempotent)...),
		map[string]any{"query": "status: open", "limit": 20},
	)
}

func alertGetSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[AlertGetInput, AlertGetResult](OperationAlertGet,
			"Show one Opsgenie alert by id, alias, or tiny id — full details, status, owner, acknowledgement state."),
		map[string]any{"id": "3", "identifier_type": "tiny"},
	)
}

func alertAckSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[AlertActionInput, AlertActionResult](OperationAlertAck,
			"Acknowledge an Opsgenie alert (stops escalation), optionally with a note.",
			writeOptions()...),
		map[string]any{"id": "3", "identifier_type": "tiny", "note": "investigating via fluxplane"},
	)
}

func alertCloseSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[AlertActionInput, AlertActionResult](OperationAlertClose,
			"Close an Opsgenie alert, optionally with a note.",
			writeOptions()...),
		map[string]any{"id": "3", "identifier_type": "tiny", "note": "resolved: rollback deployed"},
	)
}

func alertNoteSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[AlertNoteInput, AlertActionResult](OperationAlertNote,
			"Add a note to an Opsgenie alert.",
			writeOptions()...),
		map[string]any{"id": "3", "identifier_type": "tiny", "note": "root cause: media gateway OOM"},
	)
}

func onCallSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[OnCallInput, OnCallResult](OperationOnCall,
		"Who is on call right now: every schedule with its current on-call participants (optionally filtered by schedule name).",
		readOptions(core.OperationIdempotent)...)
}

func scheduleListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ScheduleListInput, ScheduleListResult](OperationScheduleList,
		"List Opsgenie schedules (id, name, timezone, enabled).",
		readOptions(core.OperationIdempotent)...)
}

func readOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(AuthPurposeAPIKey),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func writeOptions() []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(AuthPurposeAPIKey),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(core.OperationNonIdempotent),
	}
}
