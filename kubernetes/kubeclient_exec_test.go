package kubernetes

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// TestSpdyExecutorForDialRoutesThroughDialer proves the exec upgrade
// connection is carried by the injected dial function (the host conn.dial
// path): the upgrade itself fails against a plain HTTP server, but the dialer
// must have been invoked for the attempt.
func TestSpdyExecutorForDialRoutesThroughDialer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK) // not 101: upgrade fails after the dial
	}))
	defer server.Close()

	var dialed atomic.Int32
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialed.Add(1)
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
	requestURL, err := url.Parse(server.URL + "/api/v1/namespaces/latest/pods/x/exec")
	if err != nil {
		t.Fatal(err)
	}
	executor, err := spdyExecutorForDial(&restclient.Config{Host: server.URL}, dial, requestURL)
	if err != nil {
		t.Fatalf("spdyExecutorForDial: %v", err)
	}
	streamCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := executor.StreamWithContext(streamCtx, remotecommand.StreamOptions{Stdout: io.Discard}); err == nil {
		t.Fatal("expected upgrade failure against a plain HTTP server")
	}
	if dialed.Load() == 0 {
		t.Fatal("the injected dialer was not used for the exec upgrade connection")
	}
}

// Without a conn-dialing host the executor keeps the direct
// websocket-with-SPDY-fallback path.
func TestPodExecExecutorWithoutConnDialerUsesDirectTransport(t *testing.T) {
	requestURL, err := url.Parse("https://example.invalid/api/v1/namespaces/x/pods/y/exec")
	if err != nil {
		t.Fatal(err)
	}
	executor, transport, err := podExecExecutor(pluginbinding.Context{}, &restclient.Config{Host: "https://example.invalid"}, requestURL)
	if err != nil {
		t.Fatalf("podExecExecutor: %v", err)
	}
	if executor == nil || transport != execTransportDirect {
		t.Fatalf("transport = %q, want %q", transport, execTransportDirect)
	}
}
