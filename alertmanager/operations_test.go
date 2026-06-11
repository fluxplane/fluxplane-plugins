package alertmanager

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func TestManifestQuality(t *testing.T) {
	manifest := Manifest()
	if manifest.Metadata[pluginbinding.ManifestProtocolKey] != protocol.Version {
		t.Fatalf("protocol metadata = %#v", manifest.Metadata)
	}
	plugintest.AssertManifestQuality(t, manifest)
}

func TestTestReportsStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/status" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"cluster":{"status":"ready","peers":[{"name":"a"},{"name":"b"}]},"versionInfo":{"version":"0.27.0"}}`))
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[TestResult](t, plugin, OperationTest, map[string]any{"endpoint_ref": "am-dev"}, plugintest.WithHost(newAMTestHost(server.URL)))
	if !out.Ready || out.Version != "0.27.0" || out.Cluster != "ready" || out.Peers != 2 {
		t.Fatalf("test out = %#v", out)
	}
}

func TestAlertsFiltersAndDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("active") != "true" || q.Get("silenced") != "false" || q.Get("inhibited") != "false" {
			t.Fatalf("state params = %v", q)
		}
		if got := q["filter"]; len(got) != 1 || got[0] != `severity="critical"` {
			t.Fatalf("filter = %v", got)
		}
		_, _ = w.Write([]byte(`[
			{"fingerprint":"f1","labels":{"alertname":"HighErrorRate","severity":"critical"},"annotations":{"summary":"errors"},"startsAt":"2026-06-11T10:00:00Z","status":{"state":"active","silencedBy":[],"inhibitedBy":[]}}
		]`))
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[AlertsResult](t, plugin, OperationAlerts, map[string]any{
		"endpoint_ref": "am-dev", "filter": []string{`severity="critical"`},
	}, plugintest.WithHost(newAMTestHost(server.URL)))
	if out.Count != 1 || out.Alerts[0].Labels["alertname"] != "HighErrorRate" || out.Alerts[0].State != "active" {
		t.Fatalf("alerts = %#v", out)
	}
}

func TestAlertsEmptyIsArrayNotNull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[AlertsResult](t, plugin, OperationAlerts, map[string]any{"endpoint_ref": "am-dev"}, plugintest.WithHost(newAMTestHost(server.URL)))
	if out.Alerts == nil || out.Count != 0 {
		t.Fatalf("alerts = %#v, want empty array", out.Alerts)
	}
}

func TestSilenceLifecycle(t *testing.T) {
	var createdBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v2/silences":
			payload, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(payload, &createdBody)
			_, _ = w.Write([]byte(`{"silenceID":"sil-123"}`))
		case r.Method == "GET" && r.URL.Path == "/api/v2/silences":
			_, _ = w.Write([]byte(`[
				{"id":"sil-123","matchers":[{"name":"alertname","value":"HighErrorRate","isRegex":false,"isEqual":true}],"startsAt":"2026-06-11T10:00:00Z","endsAt":"2026-06-11T12:00:00Z","createdBy":"fluxplane","comment":"triage","status":{"state":"active"}},
				{"id":"sil-old","matchers":[],"status":{"state":"expired"},"createdBy":"x","comment":"old"}
			]`))
		case r.Method == "DELETE" && r.URL.Path == "/api/v2/silence/sil-123":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newAMTestHost(server.URL)

	created := plugintest.RunOK[SilenceCreateResult](t, plugin, OperationSilenceCreate, map[string]any{
		"endpoint_ref": "am-dev",
		"matchers":     []map[string]any{{"name": "alertname", "value": "HighErrorRate"}},
		"duration":     "2h",
		"comment":      "triage",
	}, plugintest.WithHost(host))
	if !created.Created || created.ID != "sil-123" {
		t.Fatalf("create = %#v", created)
	}
	if createdBody["comment"] != "triage" || createdBody["createdBy"] != "fluxplane-plugin" {
		t.Fatalf("create body = %#v", createdBody)
	}
	matchers := createdBody["matchers"].([]any)
	first := matchers[0].(map[string]any)
	if first["name"] != "alertname" || first["isEqual"] != true {
		t.Fatalf("matcher body = %#v", first)
	}

	listed := plugintest.RunOK[SilenceListResult](t, plugin, OperationSilenceList, map[string]any{"endpoint_ref": "am-dev", "state": "active"}, plugintest.WithHost(host))
	if listed.Count != 1 || listed.Silences[0].ID != "sil-123" || listed.Silences[0].State != "active" {
		t.Fatalf("list = %#v", listed)
	}

	deleted := plugintest.RunOK[SilenceDeleteResult](t, plugin, OperationSilenceDelete, map[string]any{"endpoint_ref": "am-dev", "id": "sil-123"}, plugintest.WithHost(host))
	if !deleted.Deleted {
		t.Fatalf("delete = %#v", deleted)
	}
}

func TestSilenceCreateValidation(t *testing.T) {
	plugin := NewPluginWithService(NewService())
	host := newAMTestHost("http://127.0.0.1:1")

	err := plugintest.RunError(t, plugin, OperationSilenceCreate, map[string]any{"endpoint_ref": "am-dev", "comment": "x"}, plugintest.WithHost(host))
	if err.Code != "bad_input" || !strings.Contains(err.Message, "matcher") {
		t.Fatalf("err = %#v", err)
	}
	err = plugintest.RunError(t, plugin, OperationSilenceCreate, map[string]any{
		"endpoint_ref": "am-dev", "matchers": []map[string]any{{"name": "a", "value": "b"}},
	}, plugintest.WithHost(host))
	if err.Code != "bad_input" || !strings.Contains(err.Message, "comment") {
		t.Fatalf("err = %#v", err)
	}
	err = plugintest.RunError(t, plugin, OperationSilenceCreate, map[string]any{
		"endpoint_ref": "am-dev", "matchers": []map[string]any{{"name": "a", "value": "b"}}, "comment": "x", "duration": "tomorrow",
	}, plugintest.WithHost(host))
	if err.Code != "bad_input" || !strings.Contains(err.Message, "duration") {
		t.Fatalf("err = %#v", err)
	}
}

type amTestHost struct {
	pluginbinding.HostClient
	baseURL string
}

func newAMTestHost(baseURL string) *amTestHost {
	return &amTestHost{baseURL: strings.TrimRight(baseURL, "/")}
}

func (h *amTestHost) Secret(purpose string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{Purpose: purpose}, nil
}

func (h *amTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *amTestHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	method := input.Method
	if method == "" {
		method = "GET"
	}
	target := h.baseURL + input.Path
	if len(input.Query) > 0 {
		target += "?" + url.Values(input.Query).Encode()
	}
	req, err := http.NewRequest(method, target, bytes.NewReader(input.Body))
	if err != nil {
		return pluginbinding.HTTPResponse{}, err
	}
	for key, value := range input.Headers {
		req.Header.Set(key, value)
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
	return pluginbinding.HTTPResponse{URL: target, StatusCode: resp.StatusCode, Body: body}, nil
}
