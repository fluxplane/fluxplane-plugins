package prometheus

import (
	"encoding/json"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// withInputExamples injects JSON Schema `examples` into an operation's input
// schema. The fluxplane-plugin CLI surfaces the first example as the runnable
// invocation in `operation describe` and treats an example-bearing op as having
// conditional (one-of) input during local `--dry-run` validation. Kept local to
// the prometheus plugin rather than promoted to the SDK.
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
	PluginName        = "prometheus"
	PluginVersion     = "0.20.0"
	PluginDescription = "Prometheus endpoint discovery, health checks, PromQL queries, labels, series, targets, rules, and alerts."

	OperationTest       = "prometheus.test"
	OperationQuery      = "prometheus.query"
	OperationQueryRange = "prometheus.query_range"
	OperationLabels     = "prometheus.labels"
	OperationSeries     = "prometheus.series"
	OperationTargets    = "prometheus.targets"
	OperationRules      = "prometheus.rules"
	OperationAlerts     = "prometheus.alerts"

	DatasourceQueryResults = "prometheus.query_results"
	DatasourceLabels       = "prometheus.labels"
	DatasourceTargets      = "prometheus.targets"
	DatasourceAlerts       = "prometheus.alerts"

	EntityQueryResult = "prometheus.query_result"
	EntityLabel       = "prometheus.label"
	EntityTarget      = "prometheus.target"
	EntityAlert       = "prometheus.alert"

	EndpointPrometheus = "prometheus.endpoints"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"prom", PluginName},
		Operations: []core.OperationSpec{
			testSpec(),
			querySpec(),
			queryRangeSpec(),
			labelsSpec(),
			seriesSpec(),
			targetsSpec(),
			rulesSpec(),
			alertsSpec(),
		},
		Datasources: []core.DatasourceSpec{
			queryResultsDatasourceSpec(),
			labelsDatasourceSpec(),
			targetsDatasourceSpec(),
			alertsDatasourceSpec(),
		},
	}
}

func queryResultsDatasourceSpec() core.DatasourceSpec {
	return datasourceSpec[QueryInput, QueryDatasourceResult, QueryRecord](DatasourceQueryResults, EntityQueryResult, "Prometheus instant query result records.", "query", "result_type", "endpoint_url")
}

func labelsDatasourceSpec() core.DatasourceSpec {
	return datasourceSpec[LabelsInput, LabelDatasourceResult, LabelRecord](DatasourceLabels, EntityLabel, "Prometheus label names or values.", "name", "label", "endpoint_url")
}

func targetsDatasourceSpec() core.DatasourceSpec {
	return datasourceSpec[TargetsInput, TargetDatasourceResult, TargetRecord](DatasourceTargets, EntityTarget, "Prometheus scrape targets.", "state", "job", "endpoint", "endpoint_url")
}

func alertsDatasourceSpec() core.DatasourceSpec {
	return datasourceSpec[TestInput, AlertDatasourceResult, AlertRecord](DatasourceAlerts, EntityAlert, "Prometheus alerts.", "name", "state", "severity", "endpoint_url")
}

func datasourceSpec[I any, O any, R any](name, entity, description string, completions ...string) core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[I, O](
		name,
		entity,
		description,
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.EntitySchemaFor[R](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "id", TitleField: "title"}),
		pluginbinding.Completion(description, completions...),
	)
}

func testSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TestInput, TestResult](OperationTest, "Test Prometheus readiness.", readOptions(core.OperationIdempotent)...)
}

func querySpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[QueryInput, QueryResult](OperationQuery, "Run an instant PromQL query. Results are parsed into samples (vector/scalar/string).", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "prometheus-dev", "query": "sum by (job) (rate(http_requests_total[5m]))"},
	)
}

func queryRangeSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[QueryRangeInput, QueryRangeResult](OperationQueryRange, "Run a range PromQL query. Results are parsed into series of timestamped points.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "prometheus-dev", "query": "rate(http_requests_total[5m])", "start": "1h", "end": "0s", "step": "1m"},
	)
}

func labelsSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[LabelsInput, LabelsResult](OperationLabels, "List Prometheus label names or values.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "prometheus-dev", "label": "job", "match": []string{"up"}},
	)
}

func seriesSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[SeriesInput, SeriesResult](OperationSeries, "List series label sets matching PromQL selectors.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "prometheus-dev", "match": []string{`up{job="api"}`}},
	)
}

func targetsSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[TargetsInput, TargetsResult](OperationTargets, "List Prometheus scrape targets with health and last error.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "prometheus-dev", "state": "active"},
	)
}

func rulesSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[RulesInput, RulesResult](OperationRules, "List alerting and recording rules with state and health.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "prometheus-dev", "type": "alert"},
	)
}

func alertsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TestInput, AlertsResult](OperationAlerts, "List Prometheus alerts with state, severity, labels, and annotations.", readOptions(core.OperationIdempotent)...)
}

func readOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}
