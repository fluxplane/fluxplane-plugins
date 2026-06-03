package kubernetes

import (
	"strings"
	"testing"

	manifest "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestClusterListFromKubeConfig(t *testing.T) {
	cfg := &clientcmdapi.Config{
		CurrentContext: "dev",
		Contexts: map[string]*clientcmdapi.Context{
			"dev":  {Cluster: "dev-cluster", AuthInfo: "dev-user"},
			"prod": {Cluster: "prod-cluster", AuthInfo: "prod-user"},
		},
	}

	out := clusterListFromKubeConfig(cfg)
	if len(out.Contexts) != 2 {
		t.Fatalf("contexts = %#v", out.Contexts)
	}
	if out.Contexts[0].Name != "dev" || !out.Contexts[0].Current || out.Contexts[0].Cluster != "dev-cluster" || out.Contexts[0].User != "dev-user" {
		t.Fatalf("first context = %#v", out.Contexts[0])
	}
	if out.Contexts[1].Name != "prod" || out.Contexts[1].Current {
		t.Fatalf("second context = %#v", out.Contexts[1])
	}
}

func TestResolveKubeContextFromEndpointRef(t *testing.T) {
	host := &kubernetesHostTestHost{endpoint: manifest.EndpointRef{ID: "dev", URL: "kubernetes://context/dev%2Fcontext"}}
	contextName, err := resolveKubeContext(pluginbinding.Context{Host: host}, "dev", "", "")
	if err != nil {
		t.Fatalf("resolveKubeContext: %v", err)
	}
	if contextName != "dev/context" {
		t.Fatalf("context = %q", contextName)
	}
}

func TestKubePortForwardHelperArgsIncludeDuration(t *testing.T) {
	args := kubePortForwardHelperArgs("dev", "kube-system", "service/kube-dns", "127.0.0.1", 49154, 9153, 60)
	joined := strings.Join(args, " ")
	for _, want := range []string{
		kubePortForwardHelperCommand,
		"--context dev",
		"--namespace kube-system",
		"--resource service/kube-dns",
		"--local-port 49154",
		"--remote-port 9153",
		"--duration-seconds 60",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args = %q, want %q", joined, want)
		}
	}
}

type kubernetesHostTestHost struct {
	endpoint manifest.EndpointRef
}

func (h *kubernetesHostTestHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (h *kubernetesHostTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h *kubernetesHostTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h *kubernetesHostTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h *kubernetesHostTestHost) ResolveEndpoint(string) (manifest.EndpointRef, error) {
	return h.endpoint, nil
}

func (h *kubernetesHostTestHost) HTTP(pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	return pluginbinding.HTTPResponse{}, nil
}

func (h *kubernetesHostTestHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h *kubernetesHostTestHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *kubernetesHostTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *kubernetesHostTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h *kubernetesHostTestHost) ProcessRun(pluginbinding.ProcessRunRequest) (pluginbinding.ProcessRunResponse, error) {
	return pluginbinding.ProcessRunResponse{}, nil
}

func (h *kubernetesHostTestHost) ProcessStart(pluginbinding.ProcessStartRequest) (pluginbinding.ProcessStartResponse, error) {
	return pluginbinding.ProcessStartResponse{}, nil
}

func (h *kubernetesHostTestHost) ProcessStop(pluginbinding.ProcessStopRequest) (pluginbinding.ProcessStopResponse, error) {
	return pluginbinding.ProcessStopResponse{}, nil
}

func (h *kubernetesHostTestHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}
