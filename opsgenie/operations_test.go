package opsgenie

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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

func TestAlertListAndGet(t *testing.T) {
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		switch {
		case r.URL.Path == "/v2/alerts" && r.Method == "GET":
			if r.URL.Query().Get("query") != "status: open" || r.URL.Query().Get("limit") != "20" {
				t.Fatalf("query = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"a-1","tinyId":"3","message":"High error rate","status":"open","acknowledged":false,"priority":"P2","createdAt":"2026-06-11T10:00:00Z"}]}`))
		case strings.HasPrefix(r.URL.Path, "/v2/alerts/3") && r.Method == "GET":
			if r.URL.Query().Get("identifierType") != "tiny" {
				t.Fatalf("identifierType = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`{"data":{"id":"a-1","tinyId":"3","message":"High error rate","status":"open","description":"errors spiking","details":{"namespace":"lyse"}}}`))
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newOGTestHost(server.URL, "secret-key")

	listed := plugintest.RunOK[AlertListResult](t, plugin, OperationAlertList, map[string]any{"query": "status: open"}, plugintest.WithHost(host))
	if listed.Count != 1 || listed.Alerts[0].TinyID != "3" || listed.Alerts[0].Priority != "P2" {
		t.Fatalf("list = %#v", listed)
	}
	if seenAuth != "GenieKey secret-key" {
		t.Fatalf("auth header = %q", seenAuth)
	}

	got := plugintest.RunOK[AlertGetResult](t, plugin, OperationAlertGet, map[string]any{"id": "3", "identifier_type": "tiny"}, plugintest.WithHost(host))
	if got.Alert.ID != "a-1" || got.Description != "errors spiking" || got.Details["namespace"] != "lyse" {
		t.Fatalf("get = %#v", got)
	}
}

func TestAlertAckSendsActionBody(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/alerts/a-1/acknowledge" || r.Method != "POST" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		payload, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(payload, &body)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"result":"Request will be processed","requestId":"req-9"}`))
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[AlertActionResult](t, plugin, OperationAlertAck, map[string]any{"id": "a-1", "note": "on it"}, plugintest.WithHost(newOGTestHost(server.URL, "k")))
	if !out.Accepted || out.RequestID != "req-9" {
		t.Fatalf("ack = %#v", out)
	}
	if body["note"] != "on it" || body["user"] != "fluxplane-plugin" || body["source"] != "fluxplane-plugin" {
		t.Fatalf("body = %#v", body)
	}
}

func TestOnCallFlattensSchedules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/schedules":
			_, _ = w.Write([]byte(`{"data":[
				{"id":"s-1","name":"Platform On-Call","timezone":"Europe/Berlin","enabled":true,"ownerTeam":{"name":"platform"}},
				{"id":"s-2","name":"Disabled","enabled":false}
			]}`))
		case r.URL.Path == "/v2/schedules/s-1/on-calls":
			if r.URL.Query().Get("flat") != "true" {
				t.Fatalf("flat = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`{"data":{"onCallRecipients":["ada@example.com"]}}`))
		default:
			t.Fatalf("unexpected %s", r.URL.Path)
		}
	}))
	defer server.Close()
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[OnCallResult](t, plugin, OperationOnCall, map[string]any{}, plugintest.WithHost(newOGTestHost(server.URL, "k")))
	if out.Count != 1 || out.Entries[0].Schedule != "Platform On-Call" || out.Entries[0].OnCall[0] != "ada@example.com" {
		t.Fatalf("oncall = %#v", out)
	}
}

func TestMissingKeyIsActionable(t *testing.T) {
	plugin := NewPluginWithService(NewService())
	err := plugintest.RunError(t, plugin, OperationTest, map[string]any{}, plugintest.WithHost(newOGTestHost("http://127.0.0.1:1", "")))
	if err.Code != "opsgenie" || !strings.Contains(err.Message, "auth connect opsgenie") {
		t.Fatalf("err = %#v", err)
	}
}

type ogTestHost struct {
	pluginbinding.HostClient
	baseURL string
	apiKey  string
}

func newOGTestHost(baseURL, apiKey string) *ogTestHost {
	return &ogTestHost{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey}
}

func (h *ogTestHost) Secret(purpose string) (pluginbinding.SecretMaterial, error) {
	if purpose == AuthPurposeAPIKey && h.apiKey != "" {
		return pluginbinding.SecretMaterial{Purpose: purpose, Value: h.apiKey}, nil
	}
	return pluginbinding.SecretMaterial{Purpose: purpose}, nil
}

func (h *ogTestHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
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
