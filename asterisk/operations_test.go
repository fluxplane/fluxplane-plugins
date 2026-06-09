package asterisk

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func TestAMIPingConnectsViaHostDialer(t *testing.T) {
	addr := startFakeAMIServer(t)
	host := &amiTestHost{url: "ami://operator:topsecret@" + addr}
	plugin := NewPluginWithService(NewService())

	out := plugintest.RunOK[AMIPingResult](t, plugin, OperationAMIPing,
		map[string]any{"endpoint_ref": "pbx-dev"}, plugintest.WithHost(host))
	if !out.OK || !out.Authenticated || !out.Pong {
		t.Fatalf("out = %#v", out)
	}
}

// startFakeAMIServer runs a minimal Asterisk Manager Interface server that
// greets, accepts a Login, and answers Ping with Pong. It returns its address.
func startFakeAMIServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		_, _ = fmt.Fprint(conn, "Asterisk Call Manager/2.10.0\r\n")
		drainAMIAction(reader) // Login
		_, _ = fmt.Fprint(conn, "Response: Success\r\nMessage: Authentication accepted\r\n\r\n")
		drainAMIAction(reader) // Ping
		_, _ = fmt.Fprint(conn, "Response: Success\r\nPing: Pong\r\nTimestamp: 0\r\n\r\n")
		drainAMIAction(reader) // Logoff (best effort)
	}()
	return ln.Addr().String()
}

func drainAMIAction(reader *bufio.Reader) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		if line == "\r\n" || line == "\n" {
			return
		}
	}
}

// amiTestHost provides only the host surface runAMIPing needs: endpoint
// resolution plus the conn dial capability (backed by real loopback sockets).
type amiTestHost struct {
	pluginbinding.HostClient

	url   string
	mu    sync.Mutex
	seq   int
	conns map[string]net.Conn
}

func (h *amiTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{URL: h.url}, nil
}

func (h *amiTestHost) ConnDial(req pluginbinding.ConnDialRequest) (pluginbinding.ConnDialResponse, error) {
	conn, err := net.Dial(req.Network, req.Address)
	if err != nil {
		return pluginbinding.ConnDialResponse{}, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns == nil {
		h.conns = map[string]net.Conn{}
	}
	h.seq++
	id := "c" + strconv.Itoa(h.seq)
	h.conns[id] = conn
	return pluginbinding.ConnDialResponse{ID: id, Network: req.Network}, nil
}

func (h *amiTestHost) conn(id string) net.Conn {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.conns[id]
}

func (h *amiTestHost) ConnRead(req pluginbinding.ConnReadRequest) (pluginbinding.ConnReadResponse, error) {
	c := h.conn(req.ID)
	if c == nil {
		return pluginbinding.ConnReadResponse{}, fmt.Errorf("no conn %q", req.ID)
	}
	max := req.MaxBytes
	if max <= 0 {
		max = 4096
	}
	buf := make([]byte, max)
	n, err := c.Read(buf)
	resp := pluginbinding.ConnReadResponse{Data: buf[:n]}
	if err != nil {
		if n == 0 {
			return pluginbinding.ConnReadResponse{}, err
		}
	}
	return resp, nil
}

func (h *amiTestHost) ConnWrite(req pluginbinding.ConnWriteRequest) (pluginbinding.ConnWriteResponse, error) {
	c := h.conn(req.ID)
	if c == nil {
		return pluginbinding.ConnWriteResponse{}, fmt.Errorf("no conn %q", req.ID)
	}
	n, err := c.Write(req.Data)
	return pluginbinding.ConnWriteResponse{Written: n}, err
}

func (h *amiTestHost) ConnClose(req pluginbinding.ConnCloseRequest) (pluginbinding.ConnCloseResponse, error) {
	c := h.conn(req.ID)
	if c == nil {
		return pluginbinding.ConnCloseResponse{}, nil
	}
	return pluginbinding.ConnCloseResponse{Closed: true}, c.Close()
}

func TestEndpointDiscoverFindsAMIServiceAndSecret(t *testing.T) {
	plugin := NewPluginWithService(Service{ProviderCall: fakeKubernetesProvider(t, map[string]json.RawMessage{
		"services": json.RawMessage(`[
			{"metadata":{"name":"asterisk","namespace":"latest","labels":{"app":"asterisk"}},"spec":{"type":"ClusterIP","ports":[{"name":"ami","port":5038}]}}
		]`),
		"secrets": json.RawMessage(`[
			{"metadata":{"name":"asterisk-ami","namespace":"latest"},"data":{"username":"ZGV4","secret":"c2VjcmV0"}}
		]`),
		"configmaps": json.RawMessage(`[]`),
	})})

	resp := plugin.Handle(request(t, protocol.CommandEndpointsDiscover, map[string]any{"product": "asterisk", "context": "dev", "namespace": "latest"}))
	if !resp.OK {
		t.Fatalf("response = %#v", resp.Error)
	}
	var discovery EndpointDiscoverResult
	if err := json.Unmarshal(resp.Result, &discovery); err != nil {
		t.Fatal(err)
	}
	if len(discovery.Candidates) != 1 {
		t.Fatalf("candidates = %#v", discovery.Candidates)
	}
	candidate := discovery.Candidates[0]
	if candidate.URL != "ami://asterisk.latest.svc:5038" || candidate.Protocol != "ami" || candidate.Product != "asterisk" {
		t.Fatalf("candidate = %#v", candidate)
	}
	if candidate.CredentialRef != "kubernetes://latest/secrets/asterisk-ami?context=dev" {
		t.Fatalf("credential_ref = %q", candidate.CredentialRef)
	}
}

func TestEndpointDiscoverFindsManagerConfConfigMap(t *testing.T) {
	plugin := NewPluginWithService(Service{ProviderCall: fakeKubernetesProvider(t, map[string]json.RawMessage{
		"services": json.RawMessage(`[
			{"metadata":{"name":"pbx","namespace":"latest","labels":{"app":"asterisk"}},"spec":{"ports":[{"name":"manager","port":5038}]}}
		]`),
		"secrets": json.RawMessage(`[]`),
		"configmaps": json.RawMessage(`[
			{"metadata":{"name":"asterisk-config","namespace":"latest"},"data":{"manager.conf":"[general]\nenabled = yes\n[dex]\nsecret = topsecret\nread = system\n"}}
		]`),
	})})

	resp := plugin.Handle(request(t, protocol.CommandEndpointsDiscover, map[string]any{"product": "ami", "context": "dev", "namespace": "latest"}))
	if !resp.OK {
		t.Fatalf("response = %#v", resp.Error)
	}
	var discovery EndpointDiscoverResult
	if err := json.Unmarshal(resp.Result, &discovery); err != nil {
		t.Fatal(err)
	}
	if len(discovery.Candidates) != 1 {
		t.Fatalf("candidates = %#v", discovery.Candidates)
	}
	if discovery.Candidates[0].CredentialRef != "kubernetes://latest/configmaps/asterisk-config?context=dev" {
		t.Fatalf("credential_ref = %q", discovery.Candidates[0].CredentialRef)
	}
}

func TestEndpointDiscoverIgnoresGenericDatabaseSecrets(t *testing.T) {
	plugin := NewPluginWithService(Service{ProviderCall: fakeKubernetesProvider(t, map[string]json.RawMessage{
		"services": json.RawMessage(`[]`),
		"secrets": json.RawMessage(`[
			{"metadata":{"name":"crossplane-provider-sql-db-secret-user-latest-freepbx","namespace":"latest","annotations":{"sealedsecrets.bitnami.com/cluster-wide":"true"}},"data":{"host":"ZGIuZXhhbXBsZS5jb20=","port":"NTQzMg==","username":"YXBw","password":"c2VjcmV0"}}
		]`),
		"configmaps": json.RawMessage(`[]`),
	})})

	resp := plugin.Handle(request(t, protocol.CommandEndpointsDiscover, map[string]any{"product": "asterisk", "context": "dev", "namespace": "latest"}))
	if !resp.OK {
		t.Fatalf("response = %#v", resp.Error)
	}
	var discovery EndpointDiscoverResult
	if err := json.Unmarshal(resp.Result, &discovery); err != nil {
		t.Fatal(err)
	}
	if len(discovery.Candidates) != 0 {
		t.Fatalf("candidates = %#v", discovery.Candidates)
	}
}

func TestEndpointDiscoverFindsExplicitAMISecretEndpoint(t *testing.T) {
	plugin := NewPluginWithService(Service{ProviderCall: fakeKubernetesProvider(t, map[string]json.RawMessage{
		"services": json.RawMessage(`[]`),
		"secrets": json.RawMessage(`[
			{"metadata":{"name":"pbx-ami","namespace":"latest"},"data":{"ami_host":"cGJ4LmV4YW1wbGUuY29t","ami_port":"NTAzOA==","ami_username":"ZGV4","ami_secret":"c2VjcmV0"}}
		]`),
		"configmaps": json.RawMessage(`[]`),
	})})

	resp := plugin.Handle(request(t, protocol.CommandEndpointsDiscover, map[string]any{"product": "asterisk", "context": "dev", "namespace": "latest"}))
	if !resp.OK {
		t.Fatalf("response = %#v", resp.Error)
	}
	var discovery EndpointDiscoverResult
	if err := json.Unmarshal(resp.Result, &discovery); err != nil {
		t.Fatal(err)
	}
	if len(discovery.Candidates) != 1 || discovery.Candidates[0].URL != "ami://pbx.example.com:5038" {
		t.Fatalf("candidates = %#v", discovery.Candidates)
	}
}

func fakeKubernetesProvider(t *testing.T, responses map[string]json.RawMessage) func(pluginbinding.Context, string, any) (json.RawMessage, error) {
	t.Helper()
	return func(_ pluginbinding.Context, action string, _ any) (json.RawMessage, error) {
		raw, ok := responses[action]
		if !ok {
			t.Fatalf("unexpected action %q", action)
		}
		return raw, nil
	}
}

func request(t *testing.T, command string, input any) protocol.Request {
	t.Helper()
	req, err := protocol.NewRequest(command, PluginName, input)
	if err != nil {
		t.Fatal(err)
	}
	return req
}
