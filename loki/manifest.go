package loki

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
// the loki plugin rather than promoted to the SDK.
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
	PluginName        = "loki"
	PluginVersion     = "0.20.0"
	PluginDescription = "Loki endpoint discovery, health checks, LogQL stream and metric queries, recent logs, and labels."

	EnvLokiTenantID = "LOKI_TENANT_ID"
	EnvLokiUsername = "LOKI_USERNAME"
	EnvLokiPassword = "LOKI_PASSWORD"

	AuthPurposeTenantID      = "tenant_id"
	AuthPurposeBasicUsername = "basic_username"
	AuthPurposeBasicPassword = "basic_password"

	OperationTest       = "loki.test"
	OperationQuery      = "loki.query"
	OperationMetric     = "loki.metric"
	OperationLabels     = "loki.labels"
	OperationRecentLogs = "loki.recent_logs"

	DatasourceLogEntries = "loki.log_entries"
	DatasourceLabels     = "loki.labels"

	EntityLogEntry = "loki.log_entry"
	EntityLabel    = "loki.label"

	EndpointLoki = "loki.endpoints"
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
		Operations: []core.OperationSpec{
			testSpec(),
			querySpec(),
			metricSpec(),
			labelsSpec(),
			recentLogsSpec(),
		},
		Datasources: []core.DatasourceSpec{
			logEntriesDatasourceSpec(),
			labelsDatasourceSpec(),
		},
		Auth: []core.AuthMethod{{
			Name: "endpoint",
			Kind: "config",
			Description: "All fields are optional and resolve from the persisted secret store at call time. " +
				"Setup: 1) register the Loki URL: fluxplane-plugin endpoint save loki-main https://loki.example.com --product loki  " +
				"2) store credentials if the instance needs them: fluxplane-plugin auth connect loki (tenant ID for multi-tenant X-Scope-OrgID, username+password for HTTP basic auth)  " +
				"3) verify: fluxplane-plugin operation invoke loki loki.test --arg endpoint_ref=loki-main. " +
				"Environment variables are read once during auth auto as setup hints, never at invoke time.",
			Env: []string{EnvLokiTenantID, EnvLokiUsername, EnvLokiPassword},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeTenantID, "Loki tenant ID (X-Scope-OrgID header)", false, false, EnvLokiTenantID),
				pluginbinding.AuthField(AuthPurposeBasicUsername, "HTTP basic auth username", false, false, EnvLokiUsername),
				pluginbinding.AuthField(AuthPurposeBasicPassword, "HTTP basic auth password", false, true, EnvLokiPassword),
			},
		}},
	}
}

func logEntriesDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LogEntriesInput, LogEntriesDatasourceResult](
		DatasourceLogEntries,
		EntityLogEntry,
		"Loki log entries.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.DatasourceAccess(core.OperationAccessNetwork),
		pluginbinding.DatasourceSecretPurposes(AuthPurposeTenantID, AuthPurposeBasicUsername, AuthPurposeBasicPassword),
		pluginbinding.EntitySchemaFor[LogEntryRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "id", TitleField: "title"}),
		pluginbinding.Completion("Loki log entry fields.", "app", "namespace", "pod", "container", "endpoint_url"),
	)
}

func labelsDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LabelsInput, LabelDatasourceResult](
		DatasourceLabels,
		EntityLabel,
		"Loki label names or values.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.DatasourceAccess(core.OperationAccessNetwork),
		pluginbinding.DatasourceSecretPurposes(AuthPurposeTenantID, AuthPurposeBasicUsername, AuthPurposeBasicPassword),
		pluginbinding.EntitySchemaFor[LabelRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "id", TitleField: "title"}),
		pluginbinding.Completion("Loki label fields.", "name", "label", "endpoint_url"),
	)
}

func testSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TestInput, TestResult](OperationTest, "Test Loki readiness.", readOptions(core.OperationIdempotent)...)
}

func querySpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[QueryInput, QueryResult](OperationQuery, "Run a LogQL query.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "loki-main", "query": `{namespace="core"} |= "error"`, "since": "30m", "limit": 200},
	)
}

func metricSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[MetricInput, MetricResult](OperationMetric,
			"Run a LogQL metric query over a window (query_range, matrix result) — one call for rate/count questions like \"when did this error start and how many per day\" instead of paging raw streams.",
			readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "loki-main", "query": `sum(count_over_time({namespace="core"} |= "error" [1d]))`, "since": "720h", "step": "1d"},
	)
}

func labelsSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[LabelsInput, LabelsResult](OperationLabels, "List Loki label names or values.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "loki-main", "label": "app"},
	)
}

func recentLogsSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[RecentLogsInput, QueryResult](OperationRecentLogs, "Query recent logs by app, pod, container, or text filter.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "loki-main", "app": "api", "namespace": "core", "contains": "timeout", "since": "30m"},
	)
}

func readOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(AuthPurposeTenantID, AuthPurposeBasicUsername, AuthPurposeBasicPassword),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}
