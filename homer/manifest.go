package homer

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
// the homer plugin rather than promoted to the SDK.
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
	PluginName        = "homer"
	PluginVersion     = "0.4.1"
	PluginDescription = "Homer 7 SIP capture: call search, message flows, QoS/MOS metrics, multi-leg analysis, PCAP export, and aliases."

	EnvHomerURL      = "HOMER_URL"
	EnvHomerUsername = "HOMER_USERNAME"
	EnvHomerPassword = "HOMER_PASSWORD"

	SecretPurposeUsername = "username"
	SecretPurposePassword = "password"

	OperationTest        = "homer.test"
	OperationSearch      = "homer.search"
	OperationCallList    = "homer.call.list"
	OperationCallShow    = "homer.call.show"
	OperationCallQoS     = "homer.call.qos"
	OperationCallAnalyze = "homer.call.analyze"
	OperationPCAPExport  = "homer.pcap.export"
	OperationAliasList   = "homer.alias.list"

	DatasourceCalls = "homer.calls"
	EntityCall      = "homer.call"

	EndpointHomer = "homer.endpoints"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{PluginName, "sip"},
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Operations: []core.OperationSpec{
			testSpec(),
			searchSpec(),
			callListSpec(),
			callShowSpec(),
			callQoSSpec(),
			callAnalyzeSpec(),
			pcapExportSpec(),
			aliasListSpec(),
		},
		Datasources: []core.DatasourceSpec{
			callsDatasourceSpec(),
		},
		Auth: []core.AuthMethod{{
			Name:        "endpoint",
			Kind:        "config",
			Description: "Homer web UI credentials used to obtain the API JWT. Persist once via auth connect/auto; operations never read environment variables.",
			Env:         []string{EnvHomerUsername, EnvHomerPassword},
			Fields: []core.AuthField{
				pluginbinding.AuthField(SecretPurposeUsername, "Homer username", true, false, EnvHomerUsername),
				pluginbinding.AuthField(SecretPurposePassword, "Homer password", true, true, EnvHomerPassword),
			},
		}},
		Endpoints: []core.EndpointSpec{{
			Name:        EndpointHomer,
			Description: "Homer webapp endpoints. Discover in-cluster instances via kubernetes.endpoint.discover (product homer) plus kubernetes.portforward.start.",
			Products:    []string{"homer"},
			Env:         []string{EnvHomerURL},
		}},
	}
}

func callsDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[CallListInput, CallsDatasourceResult](
		DatasourceCalls,
		EntityCall,
		"Homer SIP calls grouped by Call-ID.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.DatasourceAccess(core.OperationAccessNetwork),
		pluginbinding.DatasourceSecretPurposes(SecretPurposeUsername, SecretPurposePassword),
		pluginbinding.EntitySchemaFor[CallDatasourceRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "id", TitleField: "title"}),
		pluginbinding.Completion("Homer call fields.", "caller", "callee", "status", "route"),
	)
}

func testSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TestInput, TestResult](OperationTest, "Test Homer reachability and authentication.", readOptions(core.OperationIdempotent)...)
}

func searchSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[SearchInput, SearchResultOutput](OperationSearch, "Search SIP messages by number, caller, callee, user agent, method, Call-ID, or query DSL.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "homer-main", "number": "4930123456", "since": "1h"},
		map[string]any{"endpoint_ref": "homer-main", "query": "from_user = '4930%' AND method = 'INVITE'", "since": "30m", "limit": 100},
	)
}

func callListSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[CallListInput, CallListResult](OperationCallList, "List calls (messages grouped by Call-ID) with caller, callee, status, duration, and route.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "homer-main", "number": "4930123456", "since": "2h", "limit": 20},
	)
}

func callShowSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[CallShowInput, CallShowResult](OperationCallShow, "Show the SIP message flow of one or more calls: ordered events with SDP annotations, a plain-text ladder, and optionally raw messages.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "homer-main", "call_ids": []string{"abc123@10.0.0.1"}, "since": "24h"},
	)
}

func callQoSSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[CallQoSInput, CallQoSResult](OperationCallQoS, "Per-stream call quality from RTCP reports: packet loss, jitter, and an E-model MOS estimate.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "homer-main", "call_ids": []string{"abc123@10.0.0.1"}, "since": "24h", "clock_rate": 8000},
	)
}

func callAnalyzeSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[CallAnalyzeInput, CallAnalyzeResult](OperationCallAnalyze, "Multi-leg call analysis: find the legs of one logical call via a shared correlation header, with per-leg status/route and the merged message flow.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "homer-main", "call_id": "abc123@10.0.0.1", "correlation_header": "X-CID", "since": "6h"},
	)
}

func pcapExportSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[PCAPExportInput, PCAPExportResult](OperationPCAPExport, "Export the messages of one or more calls as a PCAP blob.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "homer-main", "call_ids": []string{"abc123@10.0.0.1"}, "since": "24h"},
	)
}

func aliasListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AliasListInput, AliasListResult](OperationAliasList, "List Homer IP/port aliases.", readOptions(core.OperationIdempotent)...)
}

func readOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(SecretPurposeUsername, SecretPurposePassword),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}
