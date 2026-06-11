package grafana

import (
	"encoding/json"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// withInputExamples injects JSON Schema `examples` into an operation's input
// schema. The fluxplane-plugin CLI surfaces the first example as the runnable
// invocation in `operation describe` and treats an example-bearing op as having
// conditional (one-of) input during local `--dry-run` validation. Kept local to
// the grafana plugin rather than promoted to the SDK.
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
	PluginName        = "grafana"
	PluginVersion     = "0.20.0"
	PluginDescription = "Grafana datasource catalog and proxy operations for Loki, Prometheus, Alertmanager, and Tempo."

	EnvGrafanaAPIToken = "GRAFANA_API_TOKEN"
	EnvGrafanaUsername = "GRAFANA_USERNAME"
	EnvGrafanaPassword = "GRAFANA_PASSWORD"

	AuthPurposeAPIToken = "api_token"
	AuthPurposeUsername = "username"
	AuthPurposePassword = "password"

	OperationTest               = "grafana.test"
	OperationDatasourceList     = "grafana.datasource.list"
	OperationDatasourceHealth   = "grafana.datasource.health"
	OperationFolderList         = "grafana.folder.list"
	OperationDashboardList      = "grafana.dashboard.list"
	OperationDashboardGet       = "grafana.dashboard.get"
	OperationAnnotationList     = "grafana.annotation.list"
	OperationAnnotationAdd      = "grafana.annotation.add"
	OperationLokiLabels         = "grafana.loki.labels"
	OperationLokiQuery          = "grafana.loki.query"
	OperationLokiRecentLogs     = "grafana.loki.recent_logs"
	OperationPrometheusQuery    = "grafana.prometheus.query"
	OperationPrometheusRange    = "grafana.prometheus.range"
	OperationPrometheusRules    = "grafana.prometheus.rules"
	OperationAlertsActive       = "grafana.alerts.active"
	OperationAlertSilencesList  = "grafana.alerts.silences.list"
	OperationAlertSilenceCreate = "grafana.alerts.silences.create"
	OperationAlertSilenceDelete = "grafana.alerts.silences.delete"
	OperationTempoSearch        = "grafana.tempo.search"
	OperationTempoTraceGet      = "grafana.tempo.trace.get"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"graf", PluginName},
		Operations: []core.OperationSpec{
			testSpec(),
			datasourceListSpec(),
			datasourceHealthSpec(),
			folderListSpec(),
			dashboardListSpec(),
			dashboardGetSpec(),
			annotationListSpec(),
			annotationAddSpec(),
			lokiLabelsSpec(),
			lokiQuerySpec(),
			lokiRecentLogsSpec(),
			prometheusQuerySpec(),
			prometheusRangeSpec(),
			prometheusRulesSpec(),
			alertsActiveSpec(),
			alertSilencesListSpec(),
			alertSilenceCreateSpec(),
			alertSilenceDeleteSpec(),
			tempoSearchSpec(),
			tempoTraceGetSpec(),
		},
		Auth: []core.AuthMethod{{
			Name: "endpoint",
			Kind: "config",
			Description: "Grafana service-account token (preferred) or basic auth, resolved from the persisted secret store at call time. " +
				"Setup: 1) register the URL: fluxplane-plugin endpoint save grafana-main https://grafana.example.com --product grafana  " +
				"2) in Grafana, mint a token under Administration → Service accounts (Viewer role suffices for reads)  " +
				"3) store it: fluxplane-plugin auth connect grafana  " +
				"4) verify both reachability and credentials: fluxplane-plugin operation invoke grafana grafana.test --arg endpoint_ref=grafana-main. " +
				"Environment variables are read once during auth auto as setup hints, never at invoke time.",
			Env: []string{EnvGrafanaAPIToken, EnvGrafanaUsername, EnvGrafanaPassword},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeAPIToken, "Grafana service account token", true, true, EnvGrafanaAPIToken),
				pluginbinding.AuthField(AuthPurposeUsername, "Grafana basic auth username", false, false, EnvGrafanaUsername),
				pluginbinding.AuthField(AuthPurposePassword, "Grafana basic auth password", true, true, EnvGrafanaPassword),
			},
		}},
	}
}

func testSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[TestInput, TestResult](OperationTest,
			"Test a Grafana endpoint in two steps — reachability (/api/health, no credentials) and stored-credential validity (/api/org) — with a hint naming the missing bootstrap step on failure.",
			readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main"},
	)
}

func datasourceListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[DatasourceListInput, DatasourceListResult](OperationDatasourceList, "List Grafana datasources and derived cluster aliases.", readOptions(core.OperationIdempotent)...)
}

func datasourceHealthSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[DatasourceHealthInput, DatasourceHealthResult](OperationDatasourceHealth, "Check Grafana datasource health.", readOptions(core.OperationIdempotent)...)
}

func folderListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[FolderListInput, FolderListResult](OperationFolderList, "List Grafana folders.", readOptions(core.OperationIdempotent)...)
}

func dashboardListSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[DashboardListInput, DashboardListResult](OperationDashboardList, "Search Grafana dashboards.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main", "query": "api errors", "limit": 20},
	)
}

func dashboardGetSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[DashboardGetInput, DashboardGetResult](OperationDashboardGet, "Get a Grafana dashboard and extract panel queries.", readOptions(core.OperationIdempotent)...)
}

func annotationListSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[AnnotationListInput, AnnotationListResult](OperationAnnotationList, "List Grafana annotations.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main", "since": "24h", "tags": []string{"deploy"}},
	)
}

func annotationAddSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[AnnotationAddInput, AnnotationAddResult](OperationAnnotationAdd, "Create a Grafana annotation.", writeOptions(core.OperationNonIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main", "text": "Deployed api v1.42.0", "tags": []string{"deploy", "api"}},
	)
}

func lokiLabelsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[LokiLabelsInput, LabelsResult](OperationLokiLabels, "List Loki labels through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...)
}

func lokiQuerySpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[LokiQueryInput, LokiQueryResult](OperationLokiQuery, "Run a Loki range query through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main", "cluster": "prod", "query": `{namespace="core"} |= "error"`, "since": "30m", "limit": 200},
	)
}

func lokiRecentLogsSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[LokiRecentLogsInput, LokiQueryResult](OperationLokiRecentLogs, "Query recent Loki logs by cluster, app, and namespace.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main", "cluster": "prod", "app": "api", "namespace": "core", "contains": "timeout", "since": "30m"},
	)
}

func prometheusQuerySpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[PrometheusQueryInput, PromQueryResult](OperationPrometheusQuery, "Run an instant Prometheus query through Grafana datasource proxy. Results are parsed into samples.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main", "cluster": "prod", "query": "sum by (job) (rate(http_requests_total[5m]))"},
	)
}

func prometheusRangeSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[PrometheusRangeInput, PromQueryResult](OperationPrometheusRange, "Run a Prometheus range query through Grafana datasource proxy. Results are parsed into series of timestamped points.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main", "cluster": "prod", "query": "rate(http_requests_total[5m])", "start": "1h", "end": "0s", "step": "1m"},
	)
}

func prometheusRulesSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[PrometheusRulesInput, PromRulesResult](OperationPrometheusRules, "List Prometheus alerting and recording rules through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main", "cluster": "prod", "type": "alert"},
	)
}

func alertsActiveSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[AlertsActiveInput, AlertsActiveResult](OperationAlertsActive, "List active Alertmanager alerts through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main", "cluster": "prod", "severity": "page"},
	)
}

func alertSilencesListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AlertSilencesListInput, SilencesListResult](OperationAlertSilencesList, "List Alertmanager silences through Grafana datasource proxy.", readOptions(core.OperationIdempotent)...)
}

func alertSilenceCreateSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[AlertSilenceCreateInput, SilenceCreateResult](OperationAlertSilenceCreate, "Create an Alertmanager silence through Grafana datasource proxy.", writeOptions(core.OperationNonIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main", "cluster": "prod", "matchers": []map[string]any{{"name": "alertname", "value": "HighErrorRate"}}, "ends_at": "2h", "comment": "silencing during deploy"},
	)
}

func alertSilenceDeleteSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AlertSilenceDeleteInput, SilenceDeleteResult](OperationAlertSilenceDelete, "Delete an Alertmanager silence through Grafana datasource proxy.", writeOptions(core.OperationNonIdempotent)...)
}

func tempoSearchSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[TempoSearchInput, TempoSearchResult](OperationTempoSearch, "Search Tempo traces through Grafana datasource proxy. Results are trace summaries.", readOptions(core.OperationIdempotent)...),
		map[string]any{"endpoint_ref": "grafana-main", "query": `{resource.service.name="api"}`, "start": "1h", "limit": 20},
	)
}

func tempoTraceGetSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[TempoTraceGetInput, TempoTraceResult](OperationTempoTraceGet, "Fetch a Tempo trace by trace ID through Grafana datasource proxy, summarized to spans with service, timing, and status.", readOptions(core.OperationIdempotent)...)
}

func readOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(grafanaSecretPurposes()...),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func writeOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(grafanaSecretPurposes()...),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(idempotency),
	}
}

func grafanaSecretPurposes() []string {
	return []string{AuthPurposeAPIToken, AuthPurposeUsername, AuthPurposePassword}
}
