package kubernetes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-plugin/protocol"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInventoryDatasourceSchemaIncludesTypedKubernetesScope(t *testing.T) {
	spec := inventoryDatasourceSpec()
	schema := string(spec.Input)
	if !strings.Contains(schema, `"context"`) || !strings.Contains(schema, `"namespace"`) {
		t.Fatalf("inventory datasource schema missing Kubernetes scope fields: %s", schema)
	}
}

func TestEndpointDiscoverFindsPrometheusService(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) {
			return []corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-prometheus-stack-prometheus", Namespace: "monitoring", Labels: map[string]string{"app.kubernetes.io/name": "prometheus"}},
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{Name: "http-web", Port: 9090}}},
			}}, nil
		},
		Secrets: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) { return nil, nil },
	})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "prometheus"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "prometheus" || candidate.URL != "http://kube-prometheus-stack-prometheus.monitoring.svc:9090" || candidate.Source != "kubernetes" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestEndpointDiscoverSerializesEmptyCandidatesAsArray(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services:  func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) { return nil, nil },
		Ingresses: func(_ context.Context, _ EndpointDiscoverInput) ([]networkingv1.Ingress, error) { return nil, nil },
		Secrets:   func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) { return nil, nil },
	})

	input, err := json.Marshal(protocol.OperationCall{Name: OperationEndpointDiscover, Input: json.RawMessage(`{"product":"loki"}`)})
	if err != nil {
		t.Fatal(err)
	}
	resp := plugin.Handle(protocol.Request{Protocol: protocol.Version, Command: protocol.CommandOperationsCall, Plugin: PluginName, Payload: input})
	if !resp.OK {
		t.Fatalf("response = %#v", resp)
	}
	if !strings.Contains(string(resp.Result), `"candidates":[]`) {
		t.Fatalf("result = %s", string(resp.Result))
	}
}

func TestEndpointDiscoverFindsGrafanaIngressWithCredentialRef(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) {
			return []corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{Name: "grafana-tempo-gateway", Namespace: "monitoring"},
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{Name: "http", Port: 80}}},
			}}, nil
		},
		Ingresses: func(_ context.Context, _ EndpointDiscoverInput) ([]networkingv1.Ingress, error) {
			return []networkingv1.Ingress{{
				ObjectMeta: metav1.ObjectMeta{Name: "grafana", Namespace: "monitoring"},
				Spec: networkingv1.IngressSpec{
					TLS: []networkingv1.IngressTLS{{Hosts: []string{"grafana.infra.example.com"}, SecretName: "grafana-tls"}},
					Rules: []networkingv1.IngressRule{{
						Host: "grafana.infra.example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{
							Path:    "/grafana",
							Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "grafana"}},
						}}}},
					}},
				},
			}, {
				ObjectMeta: metav1.ObjectMeta{Name: "grafana-tempo-gateway", Namespace: "monitoring"},
				Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{
					Host: "tempo.example.com",
					IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{
						Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "grafana-tempo-gateway"}},
					}}}},
				}}},
			}}, nil
		},
		Secrets: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) {
			return []corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "grafana-admin-creds", Namespace: "monitoring"},
				Data:       map[string][]byte{"adminuser": []byte("admin"), "adminpassword": []byte("secret")},
			}}, nil
		},
	})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "grafana", "context": "infra-eks"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "grafana" || candidate.URL != "https://grafana.infra.example.com/grafana" || candidate.Source != "kubernetes_ingress" {
		t.Fatalf("candidate = %#v", candidate)
	}
	if candidate.Labels["path"] != "/grafana" {
		t.Fatalf("labels = %#v", candidate.Labels)
	}
	if candidate.CredentialRef != "kubernetes://monitoring/secrets/grafana-admin-creds?context=infra-eks" {
		t.Fatalf("credential_ref = %q", candidate.CredentialRef)
	}
}

func TestEndpointDiscoverFindsKubernetesClusterContext(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Contexts: func() (ClusterListResult, error) {
			return ClusterListResult{Contexts: []ClusterContext{{Name: "dev/context", Current: true, Cluster: "dev", User: "aws"}}}, nil
		},
	})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "kubernetes"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "kubernetes" || candidate.Protocol != "kubernetes" || candidate.Source != "kubeconfig" || candidate.URL != "kubernetes://context/dev%2Fcontext" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestClusterTestUsesContextFromEndpointURL(t *testing.T) {
	plugin := NewPluginWithService(Service{
		ClusterProbe: func(_ context.Context, input ClusterTestInput) (ClusterTestResult, error) {
			contextName := clusterContextFromTestInput(input)
			return ClusterTestResult{Context: contextName, OK: true, ServerVersion: "v1.30.0"}, nil
		},
	})

	out := plugintest.RunOK[ClusterTestResult](t, plugin, OperationClusterTest, map[string]any{"url": "kubernetes://context/dev%2Fcontext"})
	if !out.OK || out.Context != "dev/context" || out.ServerVersion != "v1.30.0" {
		t.Fatalf("result = %#v", out)
	}
}

func TestManifestIncludesInventoryCompletionMetadata(t *testing.T) {
	manifest := Manifest()
	if len(manifest.Datasources) != 1 {
		t.Fatalf("datasources = %#v", manifest.Datasources)
	}
	completion := manifest.Datasources[0].Completion
	if completion == nil || !containsString(completion.Fields, "context") || !containsString(completion.Fields, "namespace") || !containsString(completion.Fields, "pod") || !containsString(completion.Fields, "containers") {
		t.Fatalf("completion = %#v", completion)
	}
}

func TestManifestExposesPortForwardAndLogWindowOperations(t *testing.T) {
	manifest := Manifest()
	operations := map[string]bool{}
	for _, operation := range manifest.Operations {
		operations[operation.Name] = true
	}
	for _, name := range []string{OperationPortForwardStart, OperationPortForwardStop, OperationPodLogs} {
		if !operations[name] {
			t.Fatalf("manifest missing operation %s", name)
		}
	}
}

func TestManifestAdvertisesHostEndpointRefForOperations(t *testing.T) {
	manifest := Manifest()
	for _, operation := range manifest.Operations {
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		if err := json.Unmarshal(operation.Input, &schema); err != nil {
			t.Fatal(err)
		}
		if _, ok := schema.Properties["endpoint_ref"]; !ok {
			t.Fatalf("%s input schema missing endpoint_ref: %s", operation.Name, string(operation.Input))
		}
	}
}

func TestInventoryOperationsListResources(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Namespaces: func(_ context.Context, _ InventoryInput) ([]corev1.Namespace, error) {
			return []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "latest", Labels: map[string]string{"team": "platform"}}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}}, nil
		},
		Services: func(_ context.Context, input EndpointDiscoverInput) ([]corev1.Service, error) {
			if input.Context != "dev" || input.Namespace != "latest" {
				t.Fatalf("input = %#v", input)
			}
			return []corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "latest", Labels: map[string]string{"app": "api"}},
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: "10.0.0.1", Ports: []corev1.ServicePort{{Name: "http", Port: 8080}}},
			}}, nil
		},
		Pods: func(_ context.Context, _ InventoryInput) ([]corev1.Pod, error) {
			return []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: "api-123", Namespace: "latest", Labels: map[string]string{"app": "api"}},
				Spec:       corev1.PodSpec{NodeName: "ip-10-0-0-1", Containers: []corev1.Container{{Name: "api", Image: "registry.example.com/api:latest"}}},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{{
						Name:         "api",
						Image:        "registry.example.com/api:latest",
						ImageID:      "sha256:abc",
						ContainerID:  "containerd://123",
						Ready:        true,
						RestartCount: 1,
						State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
					}},
				},
			}}, nil
		},
		Deployments: func(_ context.Context, _ InventoryInput) ([]appsv1.Deployment, error) {
			return []appsv1.Deployment{{
				ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "latest", Labels: map[string]string{"app": "api"}},
				Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(2), Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}},
				Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, AvailableReplicas: 1, UpdatedReplicas: 2},
			}}, nil
		},
		Logs: func(_ context.Context, input PodLogsInput) (PodLogsResult, error) {
			if input.Namespace != "latest" || input.Name != "api-123" || input.Container != "api" || input.TailLines != 25 || !input.Timestamps {
				t.Fatalf("input = %#v", input)
			}
			return PodLogsResult{Namespace: input.Namespace, Name: input.Name, Container: input.Container, Lines: []string{"one", "two"}, Text: "one\ntwo", LineCount: 2, TailLines: input.TailLines, Timestamps: input.Timestamps}, nil
		},
	})

	namespaces := plugintest.RunOK[NamespaceListResult](t, plugin, OperationNamespaceList, map[string]any{"query": "platform"})
	if namespaces.Count != 1 || namespaces.Namespaces[0].Name != "latest" {
		t.Fatalf("namespaces = %#v", namespaces)
	}
	services := plugintest.RunOK[ServiceListResult](t, plugin, OperationServiceList, map[string]any{"context": "dev", "namespace": "latest"})
	if services.Count != 1 || services.Services[0].ID != "latest/api" || services.Services[0].Ports[0] != "http:8080" {
		t.Fatalf("services = %#v", services)
	}
	pods := plugintest.RunOK[PodListResult](t, plugin, OperationPodList, map[string]any{"query": "api"})
	if pods.Count != 1 || pods.Pods[0].Phase != "Running" || pods.Pods[0].Containers[0] != "api" {
		t.Fatalf("pods = %#v", pods)
	}
	if pods.Pods[0].Metadata != nil {
		t.Fatalf("typed pod records should not duplicate fields into metadata: %#v", pods.Pods[0].Metadata)
	}
	containers := plugintest.RunOK[ContainerListResult](t, plugin, OperationContainerList, map[string]any{"query": "api"})
	if containers.Count != 1 || containers.Containers[0].ID != "latest/api-123/api" || containers.Containers[0].State != "running" || !containers.Containers[0].Ready {
		t.Fatalf("containers = %#v", containers)
	}
	container := plugintest.RunOK[ContainerShowResult](t, plugin, OperationContainerShow, map[string]any{"namespace": "latest", "name": "api-123/api"})
	if container.Container.Image != "registry.example.com/api:latest" || container.Container.RestartCount != 1 {
		t.Fatalf("container = %#v", container)
	}
	deployments := plugintest.RunOK[DeploymentListResult](t, plugin, OperationDeploymentList, map[string]any{"query": "api"})
	if deployments.Count != 1 || deployments.Deployments[0].ReadyReplicas != 1 || deployments.Deployments[0].Replicas != 2 {
		t.Fatalf("deployments = %#v", deployments)
	}
	deployment := plugintest.RunOK[DeploymentShowResult](t, plugin, OperationDeploymentShow, map[string]any{"namespace": "latest", "name": "api"})
	if deployment.Deployment.ID != "latest/api" || deployment.Deployment.Strategy != "RollingUpdate" {
		t.Fatalf("deployment = %#v", deployment)
	}
	logs := plugintest.RunOK[PodLogsResult](t, plugin, OperationPodLogs, map[string]any{"namespace": "latest", "name": "api-123", "container": "api", "tail_lines": 25, "timestamps": true})
	if logs.LineCount != 2 || logs.Lines[1] != "two" {
		t.Fatalf("logs = %#v", logs)
	}
}

func TestPodListCarriesTriageFields(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Pods: func(_ context.Context, _ InventoryInput) ([]corev1.Pod, error) {
			return []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: "api-123", Namespace: "latest"},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "api"}, {Name: "sidecar"}}},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{Name: "api", Ready: true, RestartCount: 2, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
						{
							Name: "sidecar", Ready: false, RestartCount: 7,
							State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
							LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled"}},
						},
					},
				},
			}}, nil
		},
	})
	pods := plugintest.RunOK[PodListResult](t, plugin, OperationPodList, map[string]any{})
	pod := pods.Pods[0]
	if pod.Ready != "1/2" || pod.Restarts != 9 {
		t.Fatalf("triage summary = ready %q restarts %d", pod.Ready, pod.Restarts)
	}
	byName := map[string]PodContainerState{}
	for _, state := range pod.ContainerStates {
		byName[state.Name] = state
	}
	if byName["sidecar"].State != "waiting:CrashLoopBackOff" || byName["sidecar"].LastTerminationReason != "OOMKilled" || byName["sidecar"].RestartCount != 7 {
		t.Fatalf("sidecar state = %#v", byName["sidecar"])
	}
	if !byName["api"].Ready || byName["api"].State != "running" {
		t.Fatalf("api state = %#v", byName["api"])
	}
}

func TestIngressListProjectsRules(t *testing.T) {
	className := "nginx"
	plugin := NewPluginWithService(Service{
		Ingresses: func(_ context.Context, input EndpointDiscoverInput) ([]networkingv1.Ingress, error) {
			if input.Namespace != "latest" {
				t.Fatalf("input = %#v", input)
			}
			return []networkingv1.Ingress{{
				ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "latest"},
				Spec: networkingv1.IngressSpec{
					IngressClassName: &className,
					Rules: []networkingv1.IngressRule{{
						Host: "api.example.com",
						IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{
							Path:    "/v1",
							Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "api", Port: networkingv1.ServiceBackendPort{Number: 8080}}},
						}}}},
					}},
					TLS: []networkingv1.IngressTLS{{Hosts: []string{"api.example.com"}}},
				},
			}}, nil
		},
	})
	out := plugintest.RunOK[IngressListResult](t, plugin, OperationIngressList, map[string]any{"namespace": "latest"})
	if out.Count != 1 {
		t.Fatalf("out = %#v", out)
	}
	ingress := out.Ingresses[0]
	if ingress.Class != "nginx" || ingress.Hosts[0] != "api.example.com" || ingress.TLSHosts[0] != "api.example.com" {
		t.Fatalf("ingress = %#v", ingress)
	}
	if len(ingress.Rules) != 1 || ingress.Rules[0].Backend != "api:8080" || ingress.Rules[0].Path != "/v1" {
		t.Fatalf("rules = %#v", ingress.Rules)
	}
}

func TestDeploymentHistoryListsRevisions(t *testing.T) {
	plugin := NewPluginWithService(Service{
		ReplicaSets: func(_ context.Context, input InventoryInput) ([]appsv1.ReplicaSet, error) {
			if input.Namespace != "latest" {
				t.Fatalf("input = %#v", input)
			}
			owned := metav1.OwnerReference{Kind: "Deployment", Name: "api"}
			return []appsv1.ReplicaSet{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "api-old", Namespace: "latest", OwnerReferences: []metav1.OwnerReference{owned}, Annotations: map[string]string{"deployment.kubernetes.io/revision": "3"}},
					Spec:       appsv1.ReplicaSetSpec{Replicas: int32Ptr(0), Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "api:1.2.0"}}}}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "api-new", Namespace: "latest", OwnerReferences: []metav1.OwnerReference{owned}, Annotations: map[string]string{"deployment.kubernetes.io/revision": "4"}},
					Spec:       appsv1.ReplicaSetSpec{Replicas: int32Ptr(2), Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "api:1.3.0"}}}}},
					Status:     appsv1.ReplicaSetStatus{ReadyReplicas: 2},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "other-rs", Namespace: "latest", OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "other"}}},
				},
			}, nil
		},
	})
	out := plugintest.RunOK[DeploymentHistoryResult](t, plugin, OperationDeploymentHistory, map[string]any{"namespace": "latest", "name": "api"})
	if out.Count != 2 || out.Deployment != "api" {
		t.Fatalf("out = %#v", out)
	}
	if out.Revisions[0].Revision != 4 || !out.Revisions[0].Current || out.Revisions[0].Images[0] != "api:1.3.0" || out.Revisions[0].Ready != 2 {
		t.Fatalf("newest = %#v", out.Revisions[0])
	}
	if out.Revisions[1].Revision != 3 || out.Revisions[1].Current || out.Revisions[1].Images[0] != "api:1.2.0" {
		t.Fatalf("previous = %#v", out.Revisions[1])
	}

	err := plugintest.RunError(t, plugin, OperationDeploymentHistory, map[string]any{"namespace": "latest"})
	if err.Code != "bad_input" {
		t.Fatalf("missing name = %#v", err)
	}
}

func TestPodLogBoundsDoNotDefaultTailWhenByteOrTimeBounded(t *testing.T) {
	bounds, err := podLogBounds(PodLogsInput{LimitBytes: 2048})
	if err != nil {
		t.Fatal(err)
	}
	if bounds.TailLines != nil || bounds.LimitBytes == nil || *bounds.LimitBytes != 2048 {
		t.Fatalf("limit-only bounds = %#v", bounds)
	}
	bounds, err = podLogBounds(PodLogsInput{TailLines: 5000, LimitBytes: 3 * 1024 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	if bounds.TailLines == nil || *bounds.TailLines != 5000 || bounds.LimitBytes == nil || *bounds.LimitBytes != 3*1024*1024 {
		t.Fatalf("explicit bounds should not be capped = %#v", bounds)
	}
	bounds, err = podLogBounds(PodLogsInput{Since: "2h"})
	if err != nil {
		t.Fatal(err)
	}
	if bounds.TailLines != nil || bounds.SinceSeconds == nil || *bounds.SinceSeconds != 7200 {
		t.Fatalf("since bounds = %#v", bounds)
	}
	bounds, err = podLogBounds(PodLogsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if bounds.TailLines == nil || *bounds.TailLines != 100 {
		t.Fatalf("default bounds = %#v", bounds)
	}
}

func TestPodLogUntilFiltersTimestampedLines(t *testing.T) {
	bounds, err := podLogBounds(PodLogsInput{Until: "2026-05-28T10:01:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	text := filterPodLogText(strings.Join([]string{
		"2026-05-28T10:00:00Z first",
		"2026-05-28T10:02:00Z second",
	}, "\n"), bounds, false)
	if text != "first" {
		t.Fatalf("filtered text = %q", text)
	}
}

func TestPortForwardOperationsUseInjectedLifecycle(t *testing.T) {
	plugin := NewPluginWithService(Service{
		ForwardStart: func(_ context.Context, input PortForwardStartInput) (PortForwardResult, error) {
			if input.Namespace != "monitoring" || input.Resource != "service/loki" || input.RemotePort != 3100 {
				t.Fatalf("start input = %#v", input)
			}
			return PortForwardResult{ID: "kpf-test", Namespace: input.Namespace, Resource: input.Resource, LocalPort: 49152, RemotePort: input.RemotePort, LocalURL: "http://127.0.0.1:49152", PID: 123, ProcessGroup: 123}, nil
		},
		ForwardStop: func(_ context.Context, input PortForwardStopInput) (PortForwardStopResult, error) {
			if input.ID != "kpf-test" {
				t.Fatalf("stop input = %#v", input)
			}
			return PortForwardStopResult{ID: input.ID, Stopped: true, Signal: "SIGTERM"}, nil
		},
	})
	start := plugintest.RunOK[PortForwardResult](t, plugin, OperationPortForwardStart, map[string]any{"namespace": "monitoring", "resource": "service/loki", "remote_port": 3100})
	if start.ID != "kpf-test" || start.LocalURL != "http://127.0.0.1:49152" {
		t.Fatalf("start = %#v", start)
	}
	stop := plugintest.RunOK[PortForwardStopResult](t, plugin, OperationPortForwardStop, map[string]any{"id": "kpf-test"})
	if !stop.Stopped {
		t.Fatalf("stop = %#v", stop)
	}
}

func TestPortForwardListUsesInjectedLifecycleAndFilters(t *testing.T) {
	plugin := NewPluginWithService(Service{
		ForwardList: func(_ context.Context, input PortForwardListInput) (PortForwardListResult, error) {
			if input.Namespace != "monitoring" {
				t.Fatalf("list input = %#v", input)
			}
			return PortForwardListResult{Forwards: []PortForwardRecord{{
				ID: "kpf-test", Context: "dev", Namespace: "monitoring", Resource: "service/loki",
				LocalPort: 49152, RemotePort: 3100, LocalURL: "http://127.0.0.1:49152", PID: 123, Alive: true,
			}}, Count: 1}, nil
		},
	})
	listed := plugintest.RunOK[PortForwardListResult](t, plugin, OperationPortForwardList, map[string]any{"namespace": "monitoring"})
	if listed.Count != 1 || listed.Forwards[0].ID != "kpf-test" || !listed.Forwards[0].Alive {
		t.Fatalf("list = %#v", listed)
	}
}

// fakeProcessListerHost provides the optional ProcessLister capability for the
// HostKubePortForwardList mapping/filter test.
type fakeProcessListerHost struct {
	pluginbinding.HostClient
	processes []pluginbinding.ProcessRecord
}

func (h *fakeProcessListerHost) ProcessList(input pluginbinding.ProcessListRequest) (pluginbinding.ProcessListResponse, error) {
	if input.Group != "kubernetes.portforward" {
		return pluginbinding.ProcessListResponse{}, nil
	}
	return pluginbinding.ProcessListResponse{Processes: h.processes, Count: len(h.processes)}, nil
}

func TestHostKubePortForwardListMapsProcessRecords(t *testing.T) {
	host := &fakeProcessListerHost{processes: []pluginbinding.ProcessRecord{
		{
			ID: "kpf-1", PID: 11, Alive: true, LogPath: "/tmp/kpf-1.log",
			Metadata: map[string]string{"context": "dev", "namespace": "latest", "resource": "service/homer-webapp", "local_port": "19080", "remote_port": "80"},
		},
		{
			ID: "kpf-2", PID: 12, Alive: false,
			Metadata: map[string]string{"context": "prod", "namespace": "eu", "resource": "service/loki", "local_port": "3100", "remote_port": "3100"},
		},
	}}
	ctx := pluginbinding.Context{Host: host}

	all, err := HostKubePortForwardList(ctx, PortForwardListInput{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if all.Count != 2 {
		t.Fatalf("all = %#v", all)
	}
	record := all.Forwards[0]
	if record.ID != "kpf-1" || record.LocalPort != 19080 || record.RemotePort != 80 || record.LocalURL != "http://127.0.0.1:19080" || !record.Alive {
		t.Fatalf("record = %#v", record)
	}

	filtered, err := HostKubePortForwardList(ctx, PortForwardListInput{Context: "prod"})
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}
	if filtered.Count != 1 || filtered.Forwards[0].ID != "kpf-2" || filtered.Forwards[0].Alive {
		t.Fatalf("filtered = %#v", filtered)
	}
}

func TestInventoryDatasourceSearchFindsServicesPodsAndDeployments(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Namespaces: func(_ context.Context, input InventoryInput) ([]corev1.Namespace, error) {
			if input.URL != "kubernetes://context/dev" || input.Namespace != "" {
				t.Fatalf("namespace input = %#v", input)
			}
			return []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "latest"}}}, nil
		},
		Services: func(_ context.Context, input EndpointDiscoverInput) ([]corev1.Service, error) {
			if input.Context != "dev" || input.Namespace != "" {
				t.Fatalf("service input = %#v", input)
			}
			return []corev1.Service{{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "latest"}}}, nil
		},
		Pods: func(_ context.Context, input InventoryInput) ([]corev1.Pod, error) {
			if input.URL != "kubernetes://context/dev" || input.Namespace != "" {
				t.Fatalf("pod input = %#v", input)
			}
			return []corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{Name: "api-123", Namespace: "latest"},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "api", Image: "registry.example.com/api:latest"}}},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "api", Ready: true}}},
			}}, nil
		},
		Deployments: func(_ context.Context, input InventoryInput) ([]appsv1.Deployment, error) {
			if input.URL != "kubernetes://context/dev" || input.Namespace != "" {
				t.Fatalf("deployment input = %#v", input)
			}
			return []appsv1.Deployment{{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "latest"}, Spec: appsv1.DeploymentSpec{Replicas: int32Ptr(1)}}}, nil
		},
	})

	out := plugintest.DatasourceSearchOK[InventorySearchResult](t, plugin, map[string]any{"query": "api", "limit": 10, "url": "kubernetes://context/dev"})
	if out.Count != 4 {
		t.Fatalf("search = %#v", out)
	}
	entities := map[string]bool{}
	for _, record := range out.Records {
		entities[record.Entity] = true
	}
	if !entities[EntityService] || !entities[EntityPod] || !entities[EntityDeployment] || !entities[EntityContainer] {
		t.Fatalf("records = %#v", out.Records)
	}
	for _, record := range out.Records {
		if len(record.Metadata) == 0 {
			t.Fatalf("inventory datasource record missing metadata: %#v", record)
		}
		if _, ok := record.Metadata["name"]; ok {
			t.Fatalf("inventory datasource record duplicates name in metadata: %#v", record)
		}
	}
}

func TestEndpointsDiscoverProtocolUsesKubernetes(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) {
			return []corev1.Service{{
				ObjectMeta: metav1.ObjectMeta{Name: "loki-gateway", Namespace: "logging", Labels: map[string]string{"app.kubernetes.io/name": "loki"}},
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{Name: "http", Port: 3100}}},
			}}, nil
		},
		Secrets: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) { return nil, nil },
	})
	payload, _ := json.Marshal(map[string]any{"product": "loki"})
	resp := plugin.Handle(protocol.Request{Protocol: protocol.Version, Command: protocol.CommandEndpointsDiscover, Plugin: PluginName, Payload: payload})
	if !resp.OK {
		t.Fatalf("response = %#v", resp)
	}
	var out EndpointDiscoverResult
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Candidates) != 1 || out.Candidates[0].Product != "loki" {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
}

func TestEndpointDiscoverFindsMySQLConnectionSecret(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) { return nil, nil },
		Secrets: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) {
			return []corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "app-mysql", Namespace: "apps", Labels: map[string]string{"crossplane.io/claim-name": "app-mysql"}},
				Data: map[string][]byte{
					"host":     []byte("mysql.apps.svc"),
					"port":     []byte("3306"),
					"database": []byte("app"),
					"username": []byte("appuser"),
					"password": []byte("secret"),
				},
			}}, nil
		},
	})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "mysql", "context": "dev"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "mysql" || candidate.URL != "mysql://mysql.apps.svc:3306/app" || candidate.CredentialRef == "" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestEndpointDiscoverDefaultsExplicitMySQLSecretPort(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) { return nil, nil },
		Secrets: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) {
			return []corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "connection-secret", Namespace: "apps"},
				Data: map[string][]byte{
					"host":     []byte("database.apps.svc"),
					"database": []byte("app"),
					"username": []byte("appuser"),
					"password": []byte("secret"),
				},
			}}, nil
		},
	})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "mysql", "context": "dev"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "mysql" || candidate.URL != "mysql://database.apps.svc:3306/app" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestEndpointDiscoverFindsPostgresConnectionSecret(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Services: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Service, error) { return nil, nil },
		Secrets: func(_ context.Context, _ EndpointDiscoverInput) ([]corev1.Secret, error) {
			return []corev1.Secret{{
				ObjectMeta: metav1.ObjectMeta{Name: "crossplane-provider-sql-db-secret-user-latest-acd-providerconfig-latest-aurora-postgresql2", Namespace: "latest"},
				Data: map[string][]byte{
					"endpoint": []byte("postgres.example.com"),
					"port":     []byte("5432"),
					"username": []byte("latest-acd"),
					"password": []byte("secret"),
				},
			}}, nil
		},
	})

	out := plugintest.RunOK[EndpointDiscoverResult](t, plugin, OperationEndpointDiscover, map[string]any{"product": "postgres", "context": "dev", "namespace": "latest"})
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %#v", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Product != "postgres" || candidate.URL != "postgres://postgres.example.com:5432/latest-acd?sslmode=require" || candidate.CredentialRef == "" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func int32Ptr(value int32) *int32 {
	return &value
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestEventListSortsFiltersAndLimits(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Events: func(_ context.Context, input EventListInput) ([]corev1.Event, error) {
			if input.Name != "my-pod" || input.Kind != "Pod" {
				t.Fatalf("event input = %#v", input)
			}
			return []corev1.Event{
				{
					Type: corev1.EventTypeNormal, Reason: "Scheduled", Message: "assigned",
					LastTimestamp:  metav1.NewTime(mustParseTime(t, "2026-06-09T10:00:00Z")),
					FirstTimestamp: metav1.NewTime(mustParseTime(t, "2026-06-09T10:00:00Z")),
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "my-pod", Namespace: "default"},
				},
				{
					Type: corev1.EventTypeWarning, Reason: "BackOff", Message: "restarting failed container",
					Count:          7,
					LastTimestamp:  metav1.NewTime(mustParseTime(t, "2026-06-09T11:00:00Z")),
					FirstTimestamp: metav1.NewTime(mustParseTime(t, "2026-06-09T10:30:00Z")),
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "my-pod", Namespace: "default"},
				},
			}, nil
		},
	})

	out := plugintest.RunOK[EventListResult](t, plugin, OperationEventList, map[string]any{"namespace": "default", "name": "my-pod", "kind": "Pod"})
	if out.Count != 2 || out.Events[0].Reason != "BackOff" {
		t.Fatalf("events should be newest first: %#v", out)
	}

	warnings := plugintest.RunOK[EventListResult](t, plugin, OperationEventList, map[string]any{"name": "my-pod", "kind": "Pod", "warnings_only": true})
	if warnings.Count != 1 || warnings.Events[0].Type != corev1.EventTypeWarning || warnings.Events[0].Count != 7 {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestNodeListSummarizesStatus(t *testing.T) {
	plugin := NewPluginWithService(Service{
		Nodes: func(_ context.Context, _ NodeListInput) ([]corev1.Node, error) {
			return []corev1.Node{{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "worker-1",
					Labels: map[string]string{"node-role.kubernetes.io/worker": "true"},
				},
				Spec: corev1.NodeSpec{Unschedulable: true},
				Status: corev1.NodeStatus{
					NodeInfo:  corev1.NodeSystemInfo{KubeletVersion: "v1.32.0", Architecture: "amd64"},
					Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.5"}},
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue},
					},
				},
			}}, nil
		},
	})

	out := plugintest.RunOK[NodeListResult](t, plugin, OperationNodeList, map[string]any{})
	if out.Count != 1 {
		t.Fatalf("nodes = %#v", out)
	}
	node := out.Nodes[0]
	if !node.Ready || node.Status != "Ready,SchedulingDisabled" || node.InternalIP != "10.0.0.5" {
		t.Fatalf("node = %#v", node)
	}
	if !containsString(node.Problems, "MemoryPressure") || !containsString(node.Problems, "Unschedulable") || !containsString(node.Roles, "worker") {
		t.Fatalf("node problems/roles = %#v", node)
	}
}

func TestPodExecValidatesAndForwards(t *testing.T) {
	var captured PodExecInput
	plugin := NewPluginWithService(Service{
		Exec: func(_ context.Context, input PodExecInput) (PodExecResult, error) {
			captured = input
			return PodExecResult{Namespace: input.Namespace, Name: input.Name, Command: input.Command, ExitCode: 0, Stdout: "ok\n"}, nil
		},
	})

	out := plugintest.RunOK[PodExecResult](t, plugin, OperationPodExec, map[string]any{
		"namespace": "default", "name": "my-pod", "container": "app", "command": []string{"sh", "-c", "echo ok"},
	})
	if out.Stdout != "ok\n" || out.ExitCode != 0 || captured.Container != "app" {
		t.Fatalf("exec = %#v captured = %#v", out, captured)
	}

	if err := plugintest.RunError(t, plugin, OperationPodExec, map[string]any{"namespace": "default", "name": "my-pod"}); err.Code != "bad_input" {
		t.Fatalf("exec without command should fail with bad_input: %#v", err)
	}
}

func TestDeploymentScaleAndRestart(t *testing.T) {
	plugin := NewPluginWithService(Service{
		ScaleDeployment: func(_ context.Context, input DeploymentScaleInput) (DeploymentScaleResult, error) {
			return DeploymentScaleResult{OK: true, Namespace: input.Namespace, Name: input.Name, PreviousReplicas: 2, Replicas: *input.Replicas}, nil
		},
		RestartDeployment: func(_ context.Context, input DeploymentRestartInput) (DeploymentRestartResult, error) {
			return DeploymentRestartResult{OK: true, Namespace: input.Namespace, Name: input.Name, RestartedAt: "2026-06-09T12:00:00Z"}, nil
		},
	})

	scaled := plugintest.RunOK[DeploymentScaleResult](t, plugin, OperationDeploymentScale, map[string]any{"namespace": "default", "name": "my-app", "replicas": 3})
	if !scaled.OK || scaled.PreviousReplicas != 2 || scaled.Replicas != 3 {
		t.Fatalf("scale = %#v", scaled)
	}
	if err := plugintest.RunError(t, plugin, OperationDeploymentScale, map[string]any{"namespace": "default", "name": "my-app"}); err.Code != "bad_input" {
		t.Fatalf("scale without replicas should fail with bad_input: %#v", err)
	}

	restarted := plugintest.RunOK[DeploymentRestartResult](t, plugin, OperationDeploymentRestart, map[string]any{"namespace": "default", "name": "my-app"})
	if !restarted.OK || restarted.RestartedAt == "" {
		t.Fatalf("restart = %#v", restarted)
	}
	if err := plugintest.RunError(t, plugin, OperationDeploymentRestart, map[string]any{"name": "my-app"}); err.Code != "bad_input" {
		t.Fatalf("restart without namespace should fail with bad_input: %#v", err)
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
}

func TestScoreWithCanonicalPortPrefersAPIPort(t *testing.T) {
	api := scoreWithCanonicalPort(0.95, "loki", "http://loki.monitoring.svc:3100")
	other := scoreWithCanonicalPort(0.95, "loki", "http://loki-memberlist.monitoring.svc:7946")
	if api <= other {
		t.Fatalf("canonical port must outrank: api=%v other=%v", api, other)
	}
	if got := scoreWithCanonicalPort(0.7, "homer", "http://homer-webapp.latest.svc:80"); got != 0.7 {
		t.Fatalf("products without canonical port keep their score: %v", got)
	}
}
