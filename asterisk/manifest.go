package asterisk

import (
	"encoding/json"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

// withInputExamples injects JSON Schema `examples` into an operation's input
// schema. The fluxplane-plugin CLI surfaces the first example as the runnable
// invocation in `operation describe` and treats an example-bearing op as having
// conditional (one-of) input during local `--dry-run` validation. Kept local to
// the asterisk plugin rather than promoted to the SDK.
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
	PluginName        = "asterisk"
	PluginVersion     = "0.19.1"
	PluginDescription = "Asterisk endpoint discovery and AMI telephony operations: channels, peers, queues, device states, CLI commands, originate, and hangup."

	EnvAsteriskAMIUsername = "ASTERISK_AMI_USERNAME"
	EnvAsteriskAMISecret   = "ASTERISK_AMI_SECRET"
	EnvAsteriskAMIPassword = "ASTERISK_AMI_PASSWORD"

	AuthMethodAMI       = "ami"
	AuthPurposeUsername = "username"
	AuthPurposeSecret   = "secret"
	AuthPurposePassword = "password"

	OperationAMIPing         = "asterisk.ami.ping"
	OperationChannelList     = "asterisk.channel.list"
	OperationChannelHangup   = "asterisk.channel.hangup"
	OperationPeerList        = "asterisk.peer.list"
	OperationQueueStatus     = "asterisk.queue.status"
	OperationDeviceStateList = "asterisk.devicestate.list"
	OperationCommand         = "asterisk.command"
	OperationOriginate       = "asterisk.call.originate"

	EndpointAsterisk = "asterisk.endpoints"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"pbx", PluginName},
		Operations: []core.OperationSpec{
			amiPingSpec(),
			channelListSpec(),
			channelHangupSpec(),
			peerListSpec(),
			queueStatusSpec(),
			deviceStateListSpec(),
			commandSpec(),
			originateSpec(),
		},
		Auth: []core.AuthMethod{{
			Name:        AuthMethodAMI,
			Kind:        "username_secret",
			Description: "Asterisk Manager Interface username and secret.",
			Env:         []string{EnvAsteriskAMIUsername, EnvAsteriskAMISecret, EnvAsteriskAMIPassword},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeUsername, "AMI username", true, false, EnvAsteriskAMIUsername),
				pluginbinding.AuthField(AuthPurposeSecret, "AMI secret", true, true, EnvAsteriskAMISecret),
				pluginbinding.AuthField(AuthPurposePassword, "AMI password alias", false, true, EnvAsteriskAMIPassword),
			},
		}},
		Endpoints: []core.EndpointSpec{
			pluginbinding.Endpoint(EndpointAsterisk, "Asterisk AMI endpoints discovered from Kubernetes services, secrets, and config maps.", "asterisk", "ami", "asterisk-ami"),
		},
		Metadata: map[string]string{
			pluginbinding.ManifestProtocolKey: protocol.Version,
		},
	}
}

func amiPingSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AMIPingInput, AMIPingResult](
		OperationAMIPing,
		"Ping an Asterisk Manager Interface endpoint.",
		amiReadOptions()...,
	)
}

func channelListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ChannelListInput, ChannelListResult](
		OperationChannelList,
		"List active Asterisk channels (live calls) with state, caller ID, dialplan position, application, and duration.",
		amiReadOptions()...,
	)
}

func channelHangupSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[HangupInput, HangupResult](
			OperationChannelHangup,
			"Hang up one active Asterisk channel by exact name (terminates a live call).",
			amiWriteOptions(core.OperationRiskDestructive, core.OperationNonIdempotent)...,
		),
		map[string]any{"endpoint_ref": "asterisk-ami", "channel": "PJSIP/agent-7-00000123", "cause": 16},
	)
}

func peerListSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[PeerListInput, PeerListResult](
			OperationPeerList,
			"List Asterisk peers/endpoints (pjsip default, sip, or iax) with registration address and device status.",
			amiReadOptions()...,
		),
		map[string]any{"endpoint_ref": "asterisk-ami", "technology": "pjsip"},
	)
}

func queueStatusSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[QueueStatusInput, QueueStatusResult](
		OperationQueueStatus,
		"Show Asterisk call queues: stats (calls, hold/talk time, abandoned), members with status/pause, and waiting callers.",
		amiReadOptions()...,
	)
}

func deviceStateListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[DeviceStateListInput, DeviceStateListResult](
		OperationDeviceStateList,
		"List Asterisk device states (NOT_INUSE, INUSE, RINGING, ...), filterable by device-name substring.",
		amiReadOptions()...,
	)
}

func commandSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[CommandInput, CommandResult](
			OperationCommand,
			"Run an Asterisk CLI command over AMI and return its output. Powerful — CLI commands can mutate the PBX.",
			amiWriteOptions(core.OperationRiskHigh, core.OperationNonIdempotent)...,
		),
		map[string]any{"endpoint_ref": "asterisk-ami", "command": "core show uptime"},
	)
}

func originateSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[OriginateInput, OriginateResult](
			OperationOriginate,
			"Originate a call: dial channel first, then connect it to exten+context or run application. Places a real call.",
			amiWriteOptions(core.OperationRiskHigh, core.OperationNonIdempotent)...,
		),
		map[string]any{"endpoint_ref": "asterisk-ami", "channel": "PJSIP/agent-7", "exten": "100", "context": "from-internal", "caller_id": "Fluxplane <7000>", "timeout_ms": 30000},
	)
}

func amiReadOptions() []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(AuthPurposeUsername, AuthPurposeSecret, AuthPurposePassword),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessProvider, core.OperationAccessAuth, core.OperationAccessSecret),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	}
}

func amiWriteOptions(risk core.OperationRisk, idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(AuthPurposeUsername, AuthPurposeSecret, AuthPurposePassword),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessProvider, core.OperationAccessAuth, core.OperationAccessSecret),
		pluginbinding.Risk(risk),
		pluginbinding.Idempotency(idempotency),
	}
}
