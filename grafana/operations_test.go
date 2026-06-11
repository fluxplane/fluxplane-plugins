package grafana

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

func TestManifestMarksCredentialsSensitive(t *testing.T) {
	manifest := manifestSpec()
	if len(manifest.Auth) != 1 {
		t.Fatalf("auth methods = %d, want 1", len(manifest.Auth))
	}

	fields := map[string]core.AuthField{}
	for _, field := range manifest.Auth[0].Fields {
		fields[field.Name] = field
	}
	for _, purpose := range []string{AuthPurposeAPIToken, AuthPurposePassword} {
		field, ok := fields[purpose]
		if !ok {
			t.Fatalf("missing auth field %q", purpose)
		}
		if !field.Secret || !field.Sensitive {
			t.Fatalf("auth field %q should be secret and sensitive: %#v", purpose, field)
		}
	}
	if fields[AuthPurposeUsername].Secret || fields[AuthPurposeUsername].Sensitive {
		t.Fatalf("username should stay non-secret config: %#v", fields[AuthPurposeUsername])
	}
}

func TestGrafanaTestProbesHealthAndCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			_, _ = w.Write([]byte(`{"database":"ok","version":"11.2.0"}`))
		case "/api/org":
			_, _ = w.Write([]byte(`{"id":1,"name":"Main Org."}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[TestResult](t, plugin, OperationTest, map[string]any{"endpoint_ref": "grafana-dev"}, plugintest.WithHost(newGrafanaTestHost(server.URL, "")))
	if !out.Healthy || out.Version != "11.2.0" || !out.Authenticated || out.OrgName != "Main Org." || out.Hint != "" {
		t.Fatalf("test out = %#v", out)
	}
}

func TestGrafanaTestHintsOnAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			_, _ = w.Write([]byte(`{"database":"ok","version":"11.2.0"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Unauthorized"}`))
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[TestResult](t, plugin, OperationTest, map[string]any{"endpoint_ref": "grafana-dev"}, plugintest.WithHost(newGrafanaTestHost(server.URL, "")))
	if !out.Healthy || out.Authenticated {
		t.Fatalf("test out = %#v", out)
	}
	if !strings.Contains(out.Hint, "auth connect grafana") || !strings.Contains(out.Hint, "Service accounts") {
		t.Fatalf("hint = %q, want bootstrap steps", out.Hint)
	}
}

func TestDatasourceListDerivesClusterAliases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/datasources" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"uid":"loki","name":"Loki Infra","type":"loki"},
			{"uid":"prometheus-alpha-east","name":"Prometheus Alpha East","type":"prometheus"},
			{"uid":"alertmanager-beta-west","name":"Alertmanager Beta West","type":"alertmanager"},
			{"uid":"tempo","name":"Tempo","type":"tempo"}
		]`))
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[DatasourceListResult](t, plugin, OperationDatasourceList, map[string]any{"endpoint_ref": "grafana-dev"}, plugintest.WithHost(host))
	if out.Count != 4 {
		t.Fatalf("count = %d", out.Count)
	}
	if out.Clusters["infra"]["loki"] != "loki" || out.Clusters["alpha-east"]["prometheus"] != "prometheus-alpha-east" {
		t.Fatalf("clusters = %#v", out.Clusters)
	}
	if out.Clusters["beta-west"]["alertmanager"] != "alertmanager-beta-west" {
		t.Fatalf("clusters = %#v", out.Clusters)
	}
}

func TestLokiLabelsUsesClusterUIDAndBearerAuth(t *testing.T) {
	var auth string
	var proxyPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"loki-alpha-east","name":"Loki Alpha East","type":"loki"}]`))
		case "/api/datasources/proxy/uid/loki-alpha-east/loki/api/v1/labels":
			proxyPath = r.URL.Path
			_, _ = w.Write([]byte(`{"status":"success","data":["pod","namespace","app"]}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "glsa_test")

	out := plugintest.RunOK[LabelsResult](t, plugin, OperationLokiLabels, map[string]any{"endpoint_ref": "grafana-dev", "cluster": "alpha"}, plugintest.WithHost(host))
	if out.UID != "loki-alpha-east" || len(out.Values) != 3 {
		t.Fatalf("result = %#v", out)
	}
	if proxyPath == "" {
		t.Fatalf("proxy path was not called")
	}
	if auth != "Bearer glsa_test" {
		t.Fatalf("authorization = %q", auth)
	}
}

func TestResolveUIDRejectsAmbiguousShortAlias(t *testing.T) {
	_, err := resolveUID([]Datasource{
		{UID: "loki-alpha-east", Type: "loki"},
		{UID: "loki-alpha-west", Type: "loki"},
	}, "loki", "alpha")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %v", err)
	}
}

func TestDashboardGetExtractsPanelQueries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/dashboards/uid/dash-1" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"dashboard":{"uid":"dash-1","title":"Runtime","panels":[{"id":7,"title":"Requests","type":"timeseries","datasource":{"type":"prometheus","uid":"prometheus-alpha-east"},"targets":[{"refId":"A","expr":"sum(rate(http_requests_total[5m]))"}]},{"id":8,"title":"Logs","type":"logs","datasource":{"type":"loki","uid":"loki-alpha-east"},"targets":[{"refId":"B","query":"{app=\"api\"}"}]}]}}`))
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[DashboardGetResult](t, plugin, OperationDashboardGet, map[string]any{"endpoint_ref": "grafana-dev", "uid": "dash-1"}, plugintest.WithHost(host))
	if out.Title != "Runtime" || len(out.Queries) != 2 {
		t.Fatalf("result = %#v", out)
	}
	if out.Queries[0].Expression == "" || out.Queries[1].Query == "" {
		t.Fatalf("queries = %#v", out.Queries)
	}
}

func TestLokiQueryNormalizesLogEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"loki-alpha-east","name":"Loki Alpha East","type":"loki"}]`))
		case "/api/datasources/proxy/uid/loki-alpha-east/loki/api/v1/query_range":
			_, _ = w.Write([]byte(`{"status":"success","data":{"result":[{"stream":{"namespace":"latest","app":"backend-acd","pod":"backend-acd-1"},"values":[["1710000000123456000","hello"],["1710000001123456000","world"]]}]}}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[LokiQueryResult](t, plugin, OperationLokiQuery, map[string]any{"endpoint_ref": "grafana-dev", "cluster": "alpha", "query": `{namespace="latest"}`, "limit": 2}, plugintest.WithHost(host))
	if out.Count != 2 || out.Limit != 2 || out.NormalizedQuery != `{namespace="latest"}` {
		t.Fatalf("result = %#v", out)
	}
	// full page -> truncation heuristic flags more entries likely exist
	if !out.Truncated {
		t.Fatalf("expected truncated flag on a full page: %#v", out)
	}
	if out.Entries[0].Line != "world" || out.Entries[0].Labels["pod"] != "backend-acd-1" {
		t.Fatalf("entries = %#v", out.Entries)
	}
}

func TestDatasourceHealthFallsBackForAlertmanager(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/datasources/uid/alertmanager-alpha/health":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"Plugin unavailable"}`))
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"alertmanager-alpha","name":"Alertmanager Alpha","type":"alertmanager"}]`))
		case "/api/datasources/proxy/uid/alertmanager-alpha/api/v2/status":
			_, _ = w.Write([]byte(`{"cluster":{"status":"ready"}}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[DatasourceHealthResult](t, plugin, OperationDatasourceHealth, map[string]any{"endpoint_ref": "grafana-dev", "uid": "alertmanager-alpha"}, plugintest.WithHost(host))
	if out.UID != "alertmanager-alpha" || out.Source != "alertmanager_status" || out.Status != "OK" {
		t.Fatalf("result = %#v", out)
	}
}

func TestDatasourceHealthReturnsAlertmanagerProxyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/datasources/uid/alertmanager-alpha/health":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"Plugin unavailable"}`))
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"alertmanager-alpha","name":"Alertmanager Alpha","type":"alertmanager"}]`))
		case "/api/datasources/proxy/uid/alertmanager-alpha/api/v2/status":
			http.NotFound(w, r)
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[DatasourceHealthResult](t, plugin, OperationDatasourceHealth, map[string]any{"endpoint_ref": "grafana-dev", "uid": "alertmanager-alpha"}, plugintest.WithHost(host))
	if out.UID != "alertmanager-alpha" || out.Status != "error" || out.Source != "alertmanager_status" || out.Error == "" {
		t.Fatalf("result = %#v", out)
	}
}

func TestGrafanaRejectsInvalidTimeRanges(t *testing.T) {
	if _, err := queryRangeValues(`{app="api"}`, "0s", "1h", 0); err == nil || !strings.Contains(err.Error(), "since must be before until") {
		t.Fatalf("queryRangeValues err = %v", err)
	}

	_, err := annotationPayload(AnnotationAddInput{
		Time:    "2024-01-02T15:04:05Z",
		TimeEnd: "2024-01-02T15:04:04Z",
		Text:    "deploy",
	})
	if err == nil || !strings.Contains(err.Error(), "time_end must be after time") {
		t.Fatalf("annotationPayload err = %v", err)
	}

	_, err = silencePayload(AlertSilenceCreateInput{
		Matchers: []AlertSilenceMatcher{{Name: "alertname", Value: "HighLatency"}},
		StartsAt: "2024-01-02T15:04:05Z",
		EndsAt:   "2024-01-02T15:04:04Z",
		Comment:  "maintenance",
	})
	if err == nil || !strings.Contains(err.Error(), "ends_at must be after starts_at") {
		t.Fatalf("silencePayload err = %v", err)
	}
}

func TestPrometheusRangeRejectsStartAfterEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/datasources" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"uid":"prometheus-alpha","name":"Prometheus Alpha","type":"prometheus"}]`))
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	err := plugintest.RunError(t, plugin, OperationPrometheusRange, map[string]any{
		"endpoint_ref": "grafana-dev",
		"cluster":      "alpha",
		"query":        "up",
		"start":        "0s",
		"end":          "1h",
	}, plugintest.WithHost(host))
	if err == nil || err.Code != "bad_input" || !strings.Contains(err.Message, "start must be before end") {
		t.Fatalf("err = %#v", err)
	}
}

func TestPrometheusQueryParsesVectorThroughProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"prometheus-alpha","name":"Prometheus Alpha","type":"prometheus"}]`))
		case "/api/datasources/proxy/uid/prometheus-alpha/api/v1/query":
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"job":"api"},"value":[1700000000,"NaN"]}]}}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[PromQueryResult](t, plugin, OperationPrometheusQuery, map[string]any{"endpoint_ref": "grafana-dev", "cluster": "alpha", "query": "up"}, plugintest.WithHost(host))
	if out.ResultType != "vector" || out.Count != 1 || out.Samples[0].Metric["job"] != "api" || out.Samples[0].Value != "NaN" {
		t.Fatalf("result = %#v", out)
	}
}

func TestPrometheusRulesParsesGroupsThroughProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"prometheus-alpha","name":"Prometheus Alpha","type":"prometheus"}]`))
		case "/api/datasources/proxy/uid/prometheus-alpha/api/v1/rules":
			_, _ = w.Write([]byte(`{"status":"success","data":{"groups":[{"name":"g","interval":60,"rules":[{"name":"HighErrorRate","type":"alerting","query":"x>1","state":"pending","duration":120,"alerts":[{}]}]}]}}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[PromRulesResult](t, plugin, OperationPrometheusRules, map[string]any{"endpoint_ref": "grafana-dev", "cluster": "alpha"}, plugintest.WithHost(host))
	if out.GroupCount != 1 || out.RuleCount != 1 {
		t.Fatalf("rules = %#v", out)
	}
	rule := out.Groups[0].Rules[0]
	if rule.State != "pending" || rule.For != "2m0s" || rule.ActiveCount != 1 {
		t.Fatalf("rule = %#v", rule)
	}
}

func TestAlertsActiveParsesAndFiltersTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"alertmanager-alpha","name":"AM Alpha","type":"alertmanager"}]`))
		case "/api/datasources/proxy/uid/alertmanager-alpha/api/v2/alerts":
			_, _ = w.Write([]byte(`[
				{"labels":{"alertname":"HighErrorRate","severity":"page","namespace":"core"},"annotations":{"summary":"errors"},"startsAt":"2026-06-10T00:00:00Z","fingerprint":"abc","status":{"state":"active","silencedBy":["s1"]}},
				{"labels":{"alertname":"DiskFull","severity":"warn","namespace":"infra"},"status":{"state":"active"}}
			]`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	out := plugintest.RunOK[AlertsActiveResult](t, plugin, OperationAlertsActive, map[string]any{"endpoint_ref": "grafana-dev", "cluster": "alpha", "severity": "page"}, plugintest.WithHost(host))
	if out.Count != 1 || len(out.Alerts) != 1 {
		t.Fatalf("alerts = %#v", out)
	}
	alert := out.Alerts[0]
	if alert.Name != "HighErrorRate" || alert.State != "active" || alert.SilencedBy[0] != "s1" || alert.Fingerprint != "abc" {
		t.Fatalf("alert = %#v", alert)
	}
}

func TestSilencesListAndCreateTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"alertmanager-alpha","name":"AM Alpha","type":"alertmanager"}]`))
		case r.URL.Path == "/api/datasources/proxy/uid/alertmanager-alpha/api/v2/silences" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[{"id":"sil-1","status":{"state":"active"},"matchers":[{"name":"alertname","value":"HighErrorRate","isRegex":false}],"startsAt":"2026-06-10T00:00:00Z","endsAt":"2026-06-10T02:00:00Z","createdBy":"fluxplane","comment":"deploy"}]`))
		case r.URL.Path == "/api/datasources/proxy/uid/alertmanager-alpha/api/v2/silences" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"silenceID":"sil-2"}`))
		default:
			t.Fatalf("path = %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	listed := plugintest.RunOK[SilencesListResult](t, plugin, OperationAlertSilencesList, map[string]any{"endpoint_ref": "grafana-dev", "cluster": "alpha"}, plugintest.WithHost(host))
	if listed.Count != 1 || listed.Silences[0].ID != "sil-1" || listed.Silences[0].State != "active" || listed.Silences[0].Matchers[0].Name != "alertname" {
		t.Fatalf("silences = %#v", listed)
	}
	created := plugintest.RunOK[SilenceCreateResult](t, plugin, OperationAlertSilenceCreate, map[string]any{
		"endpoint_ref": "grafana-dev", "cluster": "alpha",
		"matchers": []map[string]any{{"name": "alertname", "value": "HighErrorRate"}},
		"ends_at":  "2h", "comment": "deploy window",
	}, plugintest.WithHost(host))
	if created.SilenceID != "sil-2" {
		t.Fatalf("created = %#v", created)
	}
}

func TestAnnotationListAndAddTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/annotations" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[{"id":7,"time":1770000000000,"timeEnd":1770000600000,"text":"Deployed api","tags":["deploy"],"dashboardUID":"dash-1","panelId":3}]`))
		case r.URL.Path == "/api/annotations" && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"message":"Annotation added","id":8}`))
		default:
			t.Fatalf("path = %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	listed := plugintest.RunOK[AnnotationListResult](t, plugin, OperationAnnotationList, map[string]any{"endpoint_ref": "grafana-dev", "since": "24h"}, plugintest.WithHost(host))
	if listed.Count != 1 {
		t.Fatalf("annotations = %#v", listed)
	}
	annotation := listed.Annotations[0]
	if annotation.ID != 7 || annotation.Text != "Deployed api" || annotation.Time == "" || annotation.TimeEnd == "" || annotation.DashboardUID != "dash-1" {
		t.Fatalf("annotation = %#v", annotation)
	}
	added := plugintest.RunOK[AnnotationAddResult](t, plugin, OperationAnnotationAdd, map[string]any{"endpoint_ref": "grafana-dev", "text": "Deployed api v2"}, plugintest.WithHost(host))
	if added.ID != 8 || added.Message != "Annotation added" {
		t.Fatalf("added = %#v", added)
	}
}

func TestTempoSearchAndTraceGetSummarize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/datasources":
			_, _ = w.Write([]byte(`[{"uid":"tempo","name":"Tempo","type":"tempo"}]`))
		case "/api/datasources/proxy/uid/tempo/api/search":
			_, _ = w.Write([]byte(`{"traces":[{"traceID":"abc123","rootServiceName":"api","rootTraceName":"GET /orders","startTimeUnixNano":"1770000000000000000","durationMs":250}]}`))
		case "/api/datasources/proxy/uid/tempo/api/traces/abc123":
			_, _ = w.Write([]byte(`{"batches":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"api"}}]},"scopeSpans":[{"spans":[
				{"spanId":"root1","name":"GET /orders","startTimeUnixNano":"1770000000000000000","endTimeUnixNano":"1770000000250000000","status":{"code":2}},
				{"spanId":"child1","parentSpanId":"root1","name":"db.query","startTimeUnixNano":"1770000000010000000","endTimeUnixNano":"1770000000110000000","status":{"code":1}}
			]}]}]}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newGrafanaTestHost(server.URL, "")

	search := plugintest.RunOK[TempoSearchResult](t, plugin, OperationTempoSearch, map[string]any{"endpoint_ref": "grafana-dev", "query": `{}`}, plugintest.WithHost(host))
	if search.Count != 1 || search.Traces[0].TraceID != "abc123" || search.Traces[0].RootServiceName != "api" || search.Traces[0].DurationMS != 250 {
		t.Fatalf("search = %#v", search)
	}
	trace := plugintest.RunOK[TempoTraceResult](t, plugin, OperationTempoTraceGet, map[string]any{"endpoint_ref": "grafana-dev", "trace_id": "abc123"}, plugintest.WithHost(host))
	if trace.SpanCount != 2 || trace.RootSpan != "GET /orders" || trace.DurationMS != 250 || trace.Services[0] != "api" {
		t.Fatalf("trace = %#v", trace)
	}
	if trace.Spans[0].StatusCode != "error" || trace.Spans[1].StatusCode != "ok" || trace.Spans[1].DurationMS != 100 || trace.Spans[1].ParentSpanID != "root1" {
		t.Fatalf("spans = %#v", trace.Spans)
	}
}

func TestManifestIncludesExpectedOperations(t *testing.T) {
	manifest := Manifest()
	operations := map[string]bool{}
	for _, operation := range manifest.Operations {
		operations[operation.Name] = true
	}
	for _, name := range []string{OperationDatasourceList, OperationDatasourceHealth, OperationDashboardList, OperationDashboardGet, OperationAnnotationList, OperationAnnotationAdd, OperationLokiLabels, OperationLokiQuery, OperationPrometheusQuery, OperationPrometheusRules, OperationAlertsActive, OperationAlertSilencesList, OperationAlertSilenceCreate, OperationAlertSilenceDelete, OperationTempoSearch, OperationTempoTraceGet} {
		if !operations[name] {
			t.Fatalf("manifest missing operation %s", name)
		}
	}
	for _, operation := range manifest.Operations {
		if !containsString(operation.SecretPurposes, AuthPurposeAPIToken) || !containsString(operation.SecretPurposes, AuthPurposeUsername) || !containsString(operation.SecretPurposes, AuthPurposePassword) {
			t.Fatalf("%s secret purposes = %#v", operation.Name, operation.SecretPurposes)
		}
	}
	var raw string
	for _, operation := range manifest.Operations {
		if operation.Name == OperationLokiLabels {
			raw = operationInputSchema(t, operation.Input)
			break
		}
	}
	if !strings.Contains(raw, "endpoint_ref") || !strings.Contains(raw, "cluster") {
		t.Fatalf("input schema = %s", raw)
	}
	if strings.Contains(raw, "token") || strings.Contains(raw, "password") || strings.Contains(raw, "\"url\"") || strings.Contains(raw, "credential_ref") {
		t.Fatalf("input schema exposes endpoint secret fields = %s", raw)
	}
}

type grafanaTestHost struct {
	pluginbinding.HostClient

	baseURL string
	token   string
}

func newGrafanaTestHost(baseURL, token string) *grafanaTestHost {
	return &grafanaTestHost{baseURL: strings.TrimRight(baseURL, "/"), token: token}
}

func (h *grafanaTestHost) Secret(purpose string) (pluginbinding.SecretMaterial, error) {
	if purpose == AuthPurposeAPIToken && h.token != "" {
		return pluginbinding.SecretMaterial{Purpose: purpose, Value: h.token}, nil
	}
	return pluginbinding.SecretMaterial{Purpose: purpose}, nil
}

func (h *grafanaTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h *grafanaTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h *grafanaTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h *grafanaTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *grafanaTestHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	endpoint := h.baseURL + "/" + strings.TrimLeft(input.Path, "/")
	method := input.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(input.Body))
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	for key, value := range input.Headers {
		req.Header.Set(key, value)
	}
	if input.Auth != nil && input.Auth.BearerTokenPurpose == AuthPurposeAPIToken && h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	if len(input.Query) > 0 {
		query := req.URL.Query()
		for key, vals := range input.Query {
			for _, value := range vals {
				query.Add(key, value)
			}
		}
		req.URL.RawQuery = query.Encode()
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	return pluginbinding.HTTPResponse{URL: endpoint, FinalURL: resp.Request.URL.String(), Status: resp.Status, StatusCode: resp.StatusCode, Headers: resp.Header, Body: body}, nil
}

func (h *grafanaTestHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h *grafanaTestHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *grafanaTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *grafanaTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h *grafanaTestHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

var _ pluginbinding.HostClient = (*grafanaTestHost)(nil)

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func operationInputSchema(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, raw); err != nil {
		t.Fatal(err)
	}
	return compacted.String()
}
