package kubernetes

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	machineryspdy "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/streaming/pkg/httpstream"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

const (
	execTransportHostSPDY = "host-spdy"
	execTransportDirect   = "websocket"
)

// podExecExecutor builds the exec stream executor (issue #2). With a
// conn-dialing host the entire upgrade stream runs over the host conn
// capability: client-go's websocket executor cannot take a custom dialer, but
// its SPDY round tripper accepts an UpgradeTransport whose DialContext we
// point at the host dialer (kube-apiserver continues to serve SPDY exec).
// Without the capability the previous direct websocket-with-SPDY-fallback
// behavior is kept.
func podExecExecutor(ctx pluginbinding.Context, restConfig *restclient.Config, requestURL *url.URL) (remotecommand.Executor, string, error) {
	if dial := hostDial(ctx); dial != nil {
		executor, err := spdyExecutorForDial(restConfig, dial, requestURL)
		if err != nil {
			return nil, "", err
		}
		return executor, execTransportHostSPDY, nil
	}
	websocketExecutor, err := remotecommand.NewWebSocketExecutor(restConfig, "GET", requestURL.String())
	if err != nil {
		return nil, "", err
	}
	spdyExecutor, err := remotecommand.NewSPDYExecutor(restConfig, "POST", requestURL)
	if err != nil {
		return nil, "", err
	}
	executor, err := remotecommand.NewFallbackExecutor(websocketExecutor, spdyExecutor, httpstream.IsUpgradeFailure)
	if err != nil {
		return nil, "", err
	}
	return executor, execTransportDirect, nil
}

// spdyExecutorForDial mirrors client-go's transport/spdy.RoundTripperFor but
// dials the upgrade connection through the given dial function. TLS still
// terminates in-plugin using the kubeconfig CA, and auth wrappers (bearer
// token, exec credential plugins) apply exactly as in client-go's own path —
// only the socket crosses the host boundary.
func spdyExecutorForDial(restConfig *restclient.Config, dial func(context.Context, string, string) (net.Conn, error), requestURL *url.URL) (remotecommand.Executor, error) {
	tlsConfig, err := restclient.TLSConfigFor(restConfig)
	if err != nil {
		return nil, err
	}
	upgradeTransport := utilnet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: tlsConfig,
		DialContext:     dial,
	})
	roundTripper, err := machineryspdy.NewRoundTripperWithConfig(machineryspdy.RoundTripperConfig{
		UpgradeTransport: upgradeTransport,
		PingPeriod:       5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	wrapper, err := restclient.HTTPWrappersForConfig(restConfig, roundTripper)
	if err != nil {
		return nil, err
	}
	return remotecommand.NewSPDYExecutorForTransports(wrapper, roundTripper, "POST", requestURL)
}
