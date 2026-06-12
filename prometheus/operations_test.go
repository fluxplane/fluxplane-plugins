package prometheus

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

func TestQueryUsesPrometheusAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" || r.URL.Query().Get("query") != "up" {
			t.Fatalf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   map[string]any{"resultType": "vector", "result": []map[string]any{{"metric": map[string]string{"job": "api"}, "value": []any{1, "1"}}}},
		})
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newPrometheusTestHost(server.URL)

	out := plugintest.RunOK[QueryResult](t, plugin, OperationQuery, map[string]any{"endpoint_ref": "prometheus-dev", "query": "up"}, plugintest.WithHost(host))
	if out.URL != "prometheus-dev" || out.ResultType != "vector" || out.Count != 1 {
		t.Fatalf("query output = %#v", out)
	}
	if out.Samples[0].Metric["job"] != "api" || out.Samples[0].Value != "1" || out.Samples[0].Timestamp == "" {
		t.Fatalf("sample = %#v", out.Samples[0])
	}
}

func TestQueryParsesScalarAndSpecialValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		switch {
		case query == "42":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"resultType": "scalar", "result": []any{1700000000, "42"}},
			})
		default:
			// NaN / +Inf values survive as strings instead of breaking encoding.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{"resultType": "vector", "result": []map[string]any{
					{"metric": map[string]string{"job": "a"}, "value": []any{1700000000, "NaN"}},
					{"metric": map[string]string{"job": "b"}, "value": []any{1700000000, "+Inf"}},
				}},
			})
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newPrometheusTestHost(server.URL)

	scalar := plugintest.RunOK[QueryResult](t, plugin, OperationQuery, map[string]any{"endpoint_ref": "prometheus-dev", "query": "42"}, plugintest.WithHost(host))
	if scalar.Count != 1 || scalar.Samples[0].Value != "42" || len(scalar.Samples[0].Metric) != 0 {
		t.Fatalf("scalar = %#v", scalar)
	}
	special := plugintest.RunOK[QueryResult](t, plugin, OperationQuery, map[string]any{"endpoint_ref": "prometheus-dev", "query": "x"}, plugintest.WithHost(host))
	if special.Samples[0].Value != "NaN" || special.Samples[1].Value != "+Inf" {
		t.Fatalf("special values = %#v", special.Samples)
	}
}

func TestQueryRangeParsesMatrixAndTruncates(t *testing.T) {
	points := make([][]any, 0, maxPointsPerSeries+10)
	for i := 0; i < maxPointsPerSeries+10; i++ {
		points = append(points, []any{1700000000 + i, "1"})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query_range" || r.URL.Query().Get("step") != "1m" {
			t.Fatalf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{"resultType": "matrix", "result": []map[string]any{
				{"metric": map[string]string{"job": "api"}, "values": points},
			}},
		})
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newPrometheusTestHost(server.URL)

	out := plugintest.RunOK[QueryResult](t, plugin, OperationQueryRange, map[string]any{"endpoint_ref": "prometheus-dev", "query": "up", "since": "1h", "step": "1m"}, plugintest.WithHost(host))
	if out.Count != 1 || len(out.Series) != 1 {
		t.Fatalf("range output = %#v", out)
	}
	series := out.Series[0]
	if !out.Truncated || !series.Truncated || len(series.Points) != maxPointsPerSeries || series.PointCount != maxPointsPerSeries+10 {
		t.Fatalf("truncation: points=%d count=%d truncated=%v", len(series.Points), series.PointCount, series.Truncated)
	}
	// newest points kept: last point is the last served timestamp
	last := series.Points[len(series.Points)-1]
	if last.Timestamp == "" || series.Points[0].Timestamp == last.Timestamp {
		t.Fatalf("points window = first %s last %s", series.Points[0].Timestamp, last.Timestamp)
	}
}

func TestTargetsParsesActiveAndDropped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"activeTargets": []map[string]any{{
					"health": "down", "scrapePool": "api", "scrapeUrl": "http://api:9090/metrics",
					"lastError": "connection refused",
					"labels":    map[string]string{"job": "api", "instance": "api:9090"},
				}},
				"droppedTargets": []map[string]any{{
					"discoveredLabels": map[string]string{"job": "old", "__address__": "old:9090"},
				}},
			},
		})
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newPrometheusTestHost(server.URL)

	out := plugintest.RunOK[TargetsResult](t, plugin, OperationTargets, map[string]any{"endpoint_ref": "prometheus-dev"}, plugintest.WithHost(host))
	if out.ActiveCount != 1 || out.DroppedCount != 1 || len(out.Targets) != 2 {
		t.Fatalf("targets = %#v", out)
	}
	if out.Targets[0].Job != "api" || out.Targets[0].Health != "down" || out.Targets[0].LastError != "connection refused" {
		t.Fatalf("active target = %#v", out.Targets[0])
	}
	if !out.Targets[1].Dropped || out.Targets[1].Instance != "old:9090" {
		t.Fatalf("dropped target = %#v", out.Targets[1])
	}
}

func TestRulesParsesGroupsAndAlertingState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/rules" || r.URL.Query().Get("type") != "alert" {
			t.Fatalf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{"groups": []map[string]any{{
				"name": "availability", "file": "rules.yml", "interval": 30,
				"rules": []map[string]any{{
					"name": "HighErrorRate", "type": "alerting", "query": "rate(errors[5m]) > 0.1",
					"state": "firing", "duration": 300, "health": "ok",
					"labels":      map[string]string{"severity": "page"},
					"annotations": map[string]string{"summary": "errors"},
					"alerts":      []map[string]any{{"state": "firing"}},
				}},
			}}},
		})
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newPrometheusTestHost(server.URL)

	out := plugintest.RunOK[RulesResult](t, plugin, OperationRules, map[string]any{"endpoint_ref": "prometheus-dev", "type": "alert"}, plugintest.WithHost(host))
	if out.GroupCount != 1 || out.RuleCount != 1 {
		t.Fatalf("rules = %#v", out)
	}
	rule := out.Groups[0].Rules[0]
	if rule.Name != "HighErrorRate" || rule.State != "firing" || rule.For != "5m0s" || rule.ActiveCount != 1 || rule.Labels["severity"] != "page" {
		t.Fatalf("rule = %#v", rule)
	}
	if out.Groups[0].Interval != "30s" {
		t.Fatalf("interval = %q", out.Groups[0].Interval)
	}
}

func TestSeriesRequiresMatchAndCaps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/series" || len(r.URL.Query()["match[]"]) != 1 {
			t.Fatalf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": []map[string]string{
				{"__name__": "up", "job": "api"},
				{"__name__": "up", "job": "db"},
				{"__name__": "up", "job": "web"},
			},
		})
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newPrometheusTestHost(server.URL)

	if perr := plugintest.RunError(t, plugin, OperationSeries, map[string]any{"endpoint_ref": "prometheus-dev"}, plugintest.WithHost(host)); perr.Code != "bad_input" {
		t.Fatalf("expected bad_input error, got %#v", perr)
	}
	out := plugintest.RunOK[SeriesResult](t, plugin, OperationSeries, map[string]any{"endpoint_ref": "prometheus-dev", "match": []string{"up"}, "limit": 2}, plugintest.WithHost(host))
	if out.Count != 2 || !out.Truncated || out.Series[0]["job"] != "api" {
		t.Fatalf("series = %#v", out)
	}
}

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestDatasourceHandlersUsePrometheusAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/query":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"resultType": "vector", "result": []map[string]any{{"metric": map[string]string{"job": "api"}, "value": []any{1, "1"}}}},
			})
		case "/api/v1/labels":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"job", "instance"}})
		case "/api/v1/targets":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"activeTargets": []map[string]any{{"health": "up", "labels": map[string]string{"job": "api", "instance": "api:9090"}}}}})
		case "/api/v1/alerts":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"alerts": []map[string]any{{"state": "firing", "labels": map[string]string{"alertname": "HighErrorRate", "severity": "page"}}}}})
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newPrometheusTestHost(server.URL)

	query := plugintest.DatasourceSearchOK[QueryDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceQueryResults, "endpoint_ref": "prometheus-dev", "query": "up"}, plugintest.WithHost(host))
	if query.Count != 1 || query.Records[0].Query != "up" || query.Records[0].EndpointURL != "prometheus-dev" {
		t.Fatalf("query datasource = %#v", query)
	}
	labels := plugintest.DatasourceSearchOK[LabelDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceLabels, "endpoint_ref": "prometheus-dev"}, plugintest.WithHost(host))
	if labels.Count != 2 || labels.Records[0].Name != "job" {
		t.Fatalf("labels datasource = %#v", labels)
	}
	targets := plugintest.DatasourceSearchOK[TargetDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceTargets, "endpoint_ref": "prometheus-dev"}, plugintest.WithHost(host))
	if targets.Count != 1 || targets.Records[0].Job != "api" {
		t.Fatalf("targets datasource = %#v", targets)
	}
	alerts := plugintest.DatasourceSearchOK[AlertDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceAlerts, "endpoint_ref": "prometheus-dev"}, plugintest.WithHost(host))
	if alerts.Count != 1 || alerts.Records[0].Name != "HighErrorRate" || alerts.Records[0].Severity != "page" {
		t.Fatalf("alerts datasource = %#v", alerts)
	}
}

type prometheusTestHost struct {
	pluginbinding.HostClient

	baseURL string
}

func newPrometheusTestHost(baseURL string) *prometheusTestHost {
	return &prometheusTestHost{baseURL: strings.TrimRight(baseURL, "/")}
}

func (h *prometheusTestHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (h *prometheusTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h *prometheusTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h *prometheusTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h *prometheusTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *prometheusTestHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	method := input.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequest(method, h.baseURL+"/"+strings.TrimLeft(input.Path, "/"), bytes.NewReader(input.Body))
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	query := req.URL.Query()
	for key, values := range input.Query {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	req.URL.RawQuery = query.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	return pluginbinding.HTTPResponse{URL: req.URL.String(), FinalURL: resp.Request.URL.String(), Status: resp.Status, StatusCode: resp.StatusCode, Headers: resp.Header, Body: body}, nil
}

func (h *prometheusTestHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h *prometheusTestHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *prometheusTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *prometheusTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h *prometheusTestHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

var _ pluginbinding.HostClient = (*prometheusTestHost)(nil)
