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
	PluginVersion     = "0.19.0"
	PluginDescription = "Loki endpoint discovery, health checks, LogQL queries, recent logs, and labels."

	EnvLokiTenantID     = "LOKI_TENANT_ID"
	AuthPurposeTenantID = "tenant_id"

	OperationTest       = "loki.test"
	OperationQuery      = "loki.query"
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
			labelsSpec(),
			recentLogsSpec(),
		},
		Datasources: []core.DatasourceSpec{
			logEntriesDatasourceSpec(),
			labelsDatasourceSpec(),
		},
		Auth: []core.AuthMethod{{
			Name:        "endpoint",
			Kind:        "config",
			Description: "Optional Loki tenant ID used by host-resolved endpoint refs.",
			Env:         []string{EnvLokiTenantID},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeTenantID, "Loki tenant ID", false, false, EnvLokiTenantID),
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
		pluginbinding.DatasourceSecretPurposes(AuthPurposeTenantID),
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
		pluginbinding.DatasourceSecretPurposes(AuthPurposeTenantID),
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
		pluginbinding.SecretPurposes(AuthPurposeTenantID),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}
