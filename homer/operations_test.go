package homer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

// fakeHomer serves the Homer 7 API surface the plugin uses.
func fakeHomer(t *testing.T) *httptest.Server {
	t.Helper()
	searchData := []map[string]any{
		{
			"id": 1.0, "create_date": int64(1770000000000), "micro_ts": int64(1770000000000123),
			"srcIp": "10.0.0.1", "srcPort": 5060.0, "dstIp": "10.0.0.2", "dstPort": 5060.0,
			"sid": "leg-a@pbx", "method": "INVITE", "from_user": "4930111", "to_user": "4930222",
			"user_agent": "Asterisk PBX 11.13.1~dfsg-2+deb8u4", "cseq": "1 INVITE",
			"aliasSrc": "pbx-a", "aliasDst": "kamailio",
		},
		{
			"id": 2.0, "create_date": int64(1770000000200), "micro_ts": int64(1770000000200456),
			"srcIp": "10.0.0.2", "srcPort": 5060.0, "dstIp": "10.0.0.1", "dstPort": 5060.0,
			"sid": "leg-a@pbx", "method": "200", "from_user": "4930111", "to_user": "4930222",
			"cseq": "1 INVITE",
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/auth":
			var creds map[string]string
			_ = json.NewDecoder(r.Body).Decode(&creds)
			if creds["username"] != "ops" || creds["password"] != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"statuscode":401,"error":"Unauthorized","message":"incorrect password"}`))
				return
			}
			_, _ = w.Write([]byte(`{"token":"jwt-test-token","scope":"x"}`))
		case r.URL.Path == "/api/v3/agent/check":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Header.Get("Authorization") != "Bearer jwt-test-token":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Unauthorized"}`))
		case r.URL.Path == "/api/v3/search/call/data":
			body, _ := io.ReadAll(r.Body)
			if !bytes.Contains(body, []byte(`"smartinput"`)) && !bytes.Contains(body, []byte(`"limit"`)) {
				t.Fatalf("search payload missing filters: %s", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": searchData})
		case r.URL.Path == "/api/v3/call/transaction":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"messages": []map[string]any{
					{
						"id": 1, "sid": "leg-a@pbx", "method": "INVITE",
						"srcIp": "10.0.0.1", "srcPort": 5060, "dstIp": "10.0.0.2", "dstPort": 5060,
						"create_date": int64(1770000000000), "micro_ts": int64(1770000000000123),
						"profile": "1_call", "from_user": "4930111", "to_user": "4930222", "cseq": "1 INVITE",
						"raw": "INVITE sip:4930222@10.0.0.2 SIP/2.0\r\nCall-ID: leg-a@pbx\r\nX-CID: corr-1\r\n\r\nv=0\r\nm=audio 17818 RTP/AVP 8\r\na=rtpmap:8 PCMA/8000\r\n",
					},
					{
						"id": 2, "sid": "leg-a@pbx", "method": "200",
						"srcIp": "10.0.0.2", "srcPort": 5060, "dstIp": "10.0.0.1", "dstPort": 5060,
						"create_date": int64(1770000000200), "micro_ts": int64(1770000000200456),
						"profile": "1_call", "cseq": "1 INVITE",
						"raw": "SIP/2.0 200 OK\r\nCall-ID: leg-a@pbx\r\n\r\n",
					},
					{
						"id": 3, "sid": "leg-a@pbx",
						"srcIp": "10.0.0.1", "srcPort": 17818, "dstIp": "10.0.0.2", "dstPort": 20000,
						"create_date": int64(1770000000300), "micro_ts": int64(1770000000300000),
						"profile": "5_default", "raw": "{}",
					},
				}},
				"total": 3,
			})
		case r.URL.Path == "/api/v3/call/report/qos":
			rtcpRaw := `{"type":202,"ssrc":797632257,"report_count":1,"sender_information":{"packets":118,"octets":18880},"report_blocks":[{"source_ssrc":1354749014,"fraction_lost":0,"packets_lost":2,"highest_seq_no":42710,"ia_jitter":16,"lsr":0,"dlsr":0}]}`
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rtcp": map[string]any{"data": []map[string]any{{
					"id": 1, "srcIp": "10.0.0.1", "srcPort": 17818, "dstIp": "10.0.0.2", "dstPort": 20000,
					"sid": "leg-a@pbx", "create_date": "2026-02-07T00:10:14.943753Z", "raw": rtcpRaw,
				}}, "total": 1},
				"rtp": map[string]any{"data": []map[string]any{}, "total": 0},
			})
		case r.URL.Path == "/api/v3/export/call/messages/pcap":
			_, _ = w.Write([]byte{0xd4, 0xc3, 0xb2, 0xa1, 0x02, 0x00})
		case r.URL.Path == "/api/v3/alias":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": 1.0, "ip": "10.0.0.1", "port": 5060.0, "alias": "pbx-a", "status": true},
				{"id": 2.0, "ip": "10.0.0.2", "port": 5060.0, "alias": "kamailio", "status": true},
			}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
}

// homerTestHost forwards host HTTP to the fake server and serves credentials
// from the simulated secret store.
type homerTestHost struct {
	pluginbinding.HostClient
	baseURL string
	secrets map[string]string
	blobs   map[string][]byte
}

func newHomerTestHost(baseURL string) *homerTestHost {
	return &homerTestHost{
		baseURL: strings.TrimRight(baseURL, "/"),
		secrets: map[string]string{"username": "ops", "password": "secret"},
		blobs:   map[string][]byte{},
	}
}

func (h *homerTestHost) Secret(purpose string) (pluginbinding.SecretMaterial, error) {
	value, ok := h.secrets[purpose]
	if !ok {
		return pluginbinding.SecretMaterial{}, fmt.Errorf("secret %q is not connected", purpose)
	}
	return pluginbinding.SecretMaterial{Value: value}, nil
}

func (h *homerTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *homerTestHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	method := input.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequest(method, h.baseURL+"/"+strings.TrimLeft(input.Path, "/"), bytes.NewReader(input.Body))
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
	return pluginbinding.HTTPResponse{URL: req.URL.String(), StatusCode: resp.StatusCode, Body: body}, nil
}

func (h *homerTestHost) BlobWrite(input pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	ref := "blob://" + input.Filename
	h.blobs[ref] = append([]byte(nil), input.Content...)
	return pluginbinding.BlobRef{Ref: ref, Filename: input.Filename, Size: int64(len(input.Content))}, nil
}

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestSearchAuthenticatesAndShapesRecords(t *testing.T) {
	server := fakeHomer(t)
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newHomerTestHost(server.URL)

	out := plugintest.RunOK[SearchResultOutput](t, plugin, OperationSearch, map[string]any{"endpoint_ref": "homer-dev", "number": "4930111", "since": "1h"}, plugintest.WithHost(host))
	if out.Count != 2 || out.Truncated {
		t.Fatalf("search = %#v", out)
	}
	first := out.Messages[0]
	if first.Method != "INVITE" || first.SrcAlias != "pbx-a" || first.UserAgent != "Asterisk 11.13.1" || first.CallID != "leg-a@pbx" {
		t.Fatalf("record = %#v", first)
	}
	if !strings.HasPrefix(first.Time, "2026-") || first.Src != "10.0.0.1:5060" {
		t.Fatalf("record time/src = %#v", first)
	}
}

func TestSearchFailsActionablyWithoutSecrets(t *testing.T) {
	server := fakeHomer(t)
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newHomerTestHost(server.URL)
	host.secrets = map[string]string{} // nothing connected

	perr := plugintest.RunError(t, plugin, OperationSearch, map[string]any{"endpoint_ref": "homer-dev", "number": "4930111"}, plugintest.WithHost(host))
	if perr.Code != "homer" || !strings.Contains(perr.Message, "auth connect homer") {
		t.Fatalf("error = %#v", perr)
	}
}

func TestCallListGroupsAndDerivesStatus(t *testing.T) {
	server := fakeHomer(t)
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newHomerTestHost(server.URL)

	out := plugintest.RunOK[CallListResult](t, plugin, OperationCallList, map[string]any{"endpoint_ref": "homer-dev", "since": "1h"}, plugintest.WithHost(host))
	if out.Count != 1 {
		t.Fatalf("calls = %#v", out)
	}
	call := out.Calls[0]
	if call.CallID != "leg-a@pbx" || call.Status != "answered" || call.Caller != "4930111" || call.MsgCount != 2 {
		t.Fatalf("call = %#v", call)
	}
	if call.Route != "10.0.0.1 → 10.0.0.2 → 10.0.0.1" {
		t.Fatalf("route = %q", call.Route)
	}
}

func TestCallShowBuildsFlowAndLadder(t *testing.T) {
	server := fakeHomer(t)
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newHomerTestHost(server.URL)

	out := plugintest.RunOK[CallShowResult](t, plugin, OperationCallShow, map[string]any{"endpoint_ref": "homer-dev", "call_ids": []string{"leg-a@pbx"}}, plugintest.WithHost(host))
	if out.Count != 2 { // RTCP message filtered out
		t.Fatalf("events = %#v", out)
	}
	invite := out.Events[0]
	if invite.Method != "INVITE" || invite.SDP != "PCMA :17818" || invite.OffsetMS != 0 || invite.Raw != "" {
		t.Fatalf("invite event = %#v", invite)
	}
	if out.Events[1].OffsetMS != 200 || out.Events[1].Method != "200" {
		t.Fatalf("answer event = %#v", out.Events[1])
	}
	if !strings.Contains(out.Ladder, "INVITE (PCMA :17818)") || !strings.Contains(out.Ladder, "+200ms") {
		t.Fatalf("ladder = %q", out.Ladder)
	}
	if out.Status != "answered" || out.Caller != "4930111" {
		t.Fatalf("summary = %#v", out)
	}

	withRaw := plugintest.RunOK[CallShowResult](t, plugin, OperationCallShow, map[string]any{"endpoint_ref": "homer-dev", "call_ids": []string{"leg-a@pbx"}, "include_raw": true}, plugintest.WithHost(host))
	if !strings.HasPrefix(withRaw.Events[0].Raw, "INVITE sip:") {
		t.Fatalf("raw missing: %#v", withRaw.Events[0])
	}
}

func TestCallQoSAggregatesStreams(t *testing.T) {
	server := fakeHomer(t)
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newHomerTestHost(server.URL)

	out := plugintest.RunOK[CallQoSResult](t, plugin, OperationCallQoS, map[string]any{"endpoint_ref": "homer-dev", "call_ids": []string{"leg-a@pbx"}}, plugintest.WithHost(host))
	if out.Count != 1 {
		t.Fatalf("qos = %#v", out)
	}
	stream := out.Streams[0]
	if stream.PacketsLost != 2 || stream.Packets != 118 {
		t.Fatalf("stream = %#v", stream)
	}
	// ia_jitter 16 at 8000 Hz = 2ms
	if stream.AvgJitterMS != 2 || stream.MOS < 1 || stream.MOS > 4.5 {
		t.Fatalf("jitter/mos = %#v", stream)
	}
}

func TestCallAnalyzeCorrelatesLegs(t *testing.T) {
	server := fakeHomer(t)
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newHomerTestHost(server.URL)

	out := plugintest.RunOK[CallAnalyzeResult](t, plugin, OperationCallAnalyze, map[string]any{
		"endpoint_ref": "homer-dev", "call_id": "leg-a@pbx", "correlation_header": "X-CID",
	}, plugintest.WithHost(host))
	if out.SeedCallID != "leg-a@pbx" || out.LegCount != 1 {
		t.Fatalf("analyze = %#v", out)
	}
	leg := out.Legs[0]
	if !leg.Seed || leg.Correlation != "corr-1" || leg.Status != "answered" {
		t.Fatalf("leg = %#v", leg)
	}
	if len(out.CorrelationValues) != 1 || out.CorrelationValues[0] != "corr-1" {
		t.Fatalf("correlation values = %#v", out.CorrelationValues)
	}
	if out.EventCount != 2 || out.Ladder == "" {
		t.Fatalf("events = %#v", out)
	}

	perr := plugintest.RunError(t, plugin, OperationCallAnalyze, map[string]any{"endpoint_ref": "homer-dev", "call_id": "x"}, plugintest.WithHost(host))
	if perr.Code != "bad_input" || !strings.Contains(perr.Message, "correlation_header") {
		t.Fatalf("missing header error = %#v", perr)
	}
}

func TestPCAPExportWritesBlob(t *testing.T) {
	server := fakeHomer(t)
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newHomerTestHost(server.URL)

	out := plugintest.RunOK[PCAPExportResult](t, plugin, OperationPCAPExport, map[string]any{"endpoint_ref": "homer-dev", "call_ids": []string{"leg-a@pbx"}}, plugintest.WithHost(host))
	if out.Bytes != 6 || out.Filename != "homer-leg-a_pbx.pcap" || out.BlobRef == "" {
		t.Fatalf("pcap = %#v", out)
	}
	if data := host.blobs[out.BlobRef]; len(data) != 6 || data[0] != 0xd4 {
		t.Fatalf("blob content = %v", data)
	}
}

func TestAliasListReturnsTypedAliases(t *testing.T) {
	server := fakeHomer(t)
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newHomerTestHost(server.URL)

	out := plugintest.RunOK[AliasListResult](t, plugin, OperationAliasList, map[string]any{"endpoint_ref": "homer-dev"}, plugintest.WithHost(host))
	if out.Count != 2 || out.Aliases[0].Alias != "kamailio" || !out.Aliases[0].Active {
		t.Fatalf("aliases = %#v", out)
	}
}

func TestCallsDatasourceSearch(t *testing.T) {
	server := fakeHomer(t)
	defer server.Close()
	plugin := NewPluginWithService(NewService())
	host := newHomerTestHost(server.URL)

	out := plugintest.DatasourceSearchOK[CallsDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceCalls, "endpoint_ref": "homer-dev", "since": "1h"}, plugintest.WithHost(host))
	if out.Count != 1 || out.Records[0].Caller != "4930111" || out.Records[0].Status != "answered" {
		t.Fatalf("datasource = %#v", out)
	}
}

func TestManifestExamplesDecodeAgainstInputs(t *testing.T) {
	manifest := Manifest()
	declared := map[string]bool{}
	for _, operation := range manifest.Operations {
		declared[operation.Name] = true
		var schema struct {
			Examples []map[string]any `json:"examples"`
		}
		_ = json.Unmarshal(operation.Input, &schema)
		switch operation.Name {
		case OperationSearch, OperationCallList, OperationCallShow, OperationCallQoS, OperationCallAnalyze, OperationPCAPExport:
			if len(schema.Examples) == 0 {
				t.Fatalf("operation %s should declare examples", operation.Name)
			}
		}
	}
	for _, name := range []string{OperationTest, OperationSearch, OperationCallList, OperationCallShow, OperationCallQoS, OperationCallAnalyze, OperationPCAPExport, OperationAliasList} {
		if !declared[name] {
			t.Fatalf("manifest missing operation %s", name)
		}
	}
}
