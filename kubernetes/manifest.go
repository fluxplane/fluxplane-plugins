package kubernetes

import (
	"encoding/json"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

// withInputExamples injects JSON Schema `examples` into an operation's input
// schema. The fluxplane-plugin CLI surfaces the first example as the runnable
// invocation in `operation describe` and treats an example-bearing op as having
// conditional (one-of) input during local `--dry-run` validation. Kept local to
// the kubernetes plugin rather than promoted to the SDK.
func withInputExamples(spec core.OperationSpec, examples ...map[string]any) core.OperationSpec {
	if len(examples) == 0 || len(spec.Input) == 0 {
		return spec
	}
	var schema map[string]any
	if err := json.Unmarshal(spec.Input, &schema); err != nil {
		return spec
	}
	arr := make([]any, 0, len(examples))
	for _, example := range examples {
		arr = append(arr, example)
	}
	schema["examples"] = arr
	if raw, err := json.Marshal(schema); err == nil {
		spec.Input = raw
	}
	return spec
}

const (
	PluginName        = "kubernetes"
	PluginVersion     = "0.21.0"
	PluginDescription = "Kubernetes cluster discovery, inventory, debugging (logs, events, exec), and deployment operations using kubeconfig."

	OperationClusterList       = "kubernetes.cluster.list"
	OperationClusterTest       = "kubernetes.cluster.test"
	OperationEndpointDiscover  = "kubernetes.endpoint.discover"
	OperationNamespaceList     = "kubernetes.namespace.list"
	OperationServiceList       = "kubernetes.service.list"
	OperationServiceShow       = "kubernetes.service.show"
	OperationPodList           = "kubernetes.pod.list"
	OperationPodShow           = "kubernetes.pod.show"
	OperationPodLogs           = "kubernetes.pod.logs"
	OperationPortForwardStart  = "kubernetes.portforward.start"
	OperationPortForwardStop   = "kubernetes.portforward.stop"
	OperationPortForwardList   = "kubernetes.portforward.list"
	OperationDeploymentList    = "kubernetes.deployment.list"
	OperationDeploymentShow    = "kubernetes.deployment.show"
	OperationContainerList     = "kubernetes.container.list"
	OperationContainerShow     = "kubernetes.container.show"
	OperationEventList         = "kubernetes.event.list"
	OperationNodeList          = "kubernetes.node.list"
	OperationPodExec           = "kubernetes.pod.exec"
	OperationDeploymentScale   = "kubernetes.deployment.scale"
	OperationDeploymentRestart = "kubernetes.deployment.restart"

	EndpointClusterDiscovered = "kubernetes.discovered_endpoints"
	DatasourceInventory       = "kubernetes.inventory"

	EntityResource   = "kubernetes.resource"
	EntityNamespace  = "kubernetes.namespace"
	EntityService    = "kubernetes.service"
	EntityPod        = "kubernetes.pod"
	EntityDeployment = "kubernetes.deployment"
	EntityContainer  = "kubernetes.container"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"kube", "k8s"},
		Operations: []core.OperationSpec{
			clusterListSpec(),
			clusterTestSpec(),
			endpointDiscoverSpec(),
			namespaceListSpec(),
			serviceListSpec(),
			serviceShowSpec(),
			podListSpec(),
			podShowSpec(),
			podLogsSpec(),
			portForwardStartSpec(),
			portForwardStopSpec(),
			portForwardListSpec(),
			deploymentListSpec(),
			deploymentShowSpec(),
			deploymentScaleSpec(),
			deploymentRestartSpec(),
			containerListSpec(),
			containerShowSpec(),
			eventListSpec(),
			nodeListSpec(),
			podExecSpec(),
		},
		Datasources: []core.DatasourceSpec{
			inventoryDatasourceSpec(),
		},
		Endpoints: []core.EndpointSpec{
			pluginbinding.Endpoint(EndpointClusterDiscovered, "Product endpoints discovered inside Kubernetes clusters.", "kubernetes", "prometheus", "loki", "homer", "mysql", "postgres"),
		},
		Metadata: map[string]string{
			pluginbinding.ManifestProtocolKey: protocol.Version,
		},
	}
}

func clusterListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ClusterListInput, ClusterListResult](
		OperationClusterList,
		"List kubeconfig contexts.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func clusterTestSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[ClusterTestInput, ClusterTestResult](
		OperationClusterTest,
		"Probe Kubernetes cluster reachability through kubeconfig.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func endpointDiscoverSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[EndpointDiscoverInput, EndpointDiscoverResult](
		OperationEndpointDiscover,
		"Discover product endpoints from Kubernetes services.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func namespaceListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, NamespaceListResult](
		OperationNamespaceList,
		"List Kubernetes namespaces.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func serviceListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, ServiceListResult](
		OperationServiceList,
		"List Kubernetes services.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func serviceShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, ServiceShowResult](
		OperationServiceShow,
		"Show one Kubernetes service.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func podListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, PodListResult](
		OperationPodList,
		"List Kubernetes pods.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func podShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, PodShowResult](
		OperationPodShow,
		"Show one Kubernetes pod.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func podLogsSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PodLogsInput, PodLogsResult](
		OperationPodLogs,
		"Read bounded logs for one Kubernetes pod with optional time bounds.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func portForwardStartSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[PortForwardStartInput, PortForwardResult](
			OperationPortForwardStart,
			"Start a managed Kubernetes port-forward for a service, pod, or deployment. List with kubernetes.portforward.list; stop with kubernetes.portforward.stop.",
			pluginbinding.Effects(core.OperationEffectWrite),
			pluginbinding.Access(core.OperationAccessProvider),
			pluginbinding.Risk(core.OperationRiskMedium),
			pluginbinding.Idempotency(core.OperationNonIdempotent),
		),
		map[string]any{"context": "my-cluster", "namespace": "monitoring", "resource": "service/homer-webapp", "remote_port": 80, "local_port": 19080},
	)
}

func portForwardStopSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[PortForwardStopInput, PortForwardStopResult](
			OperationPortForwardStop,
			"Stop a managed Kubernetes port-forward by ID or process group.",
			pluginbinding.Effects(core.OperationEffectWrite),
			pluginbinding.Access(core.OperationAccessProvider),
			pluginbinding.Risk(core.OperationRiskMedium),
			pluginbinding.Idempotency(core.OperationIdempotent),
		),
		map[string]any{"id": "portforward-monitoring-service-homer-webapp-19080-80"},
	)
}

func portForwardListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PortForwardListInput, PortForwardListResult](
		OperationPortForwardList,
		"List managed Kubernetes port-forwards with liveness, local URL, and target metadata.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func deploymentListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, DeploymentListResult](
		OperationDeploymentList,
		"List Kubernetes deployments.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func deploymentShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, DeploymentShowResult](
		OperationDeploymentShow,
		"Show one Kubernetes deployment.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func containerListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, ContainerListResult](
		OperationContainerList,
		"List Kubernetes containers derived from pods.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func containerShowSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InventoryInput, ContainerShowResult](
		OperationContainerShow,
		"Show one Kubernetes container derived from a pod.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func eventListSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[EventListInput, EventListResult](
			OperationEventList,
			"List Kubernetes events (newest first), filterable by namespace, involved object name/kind, and Warning type — the first stop when debugging scheduling, image, or crash issues.",
			kubernetesReadOptions(core.OperationIdempotent)...,
		),
		map[string]any{"namespace": "default", "name": "my-pod-abc123", "kind": "Pod", "limit": 20},
	)
}

func nodeListSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[NodeListInput, NodeListResult](
		OperationNodeList,
		"List Kubernetes nodes with readiness, roles, abnormal conditions, kubelet version, and capacity.",
		kubernetesReadOptions(core.OperationIdempotent)...,
	)
}

func podExecSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[PodExecInput, PodExecResult](
			OperationPodExec,
			"Run a one-shot command in a pod container and return bounded stdout/stderr with the exit code. No TTY or stdin; output is capped at 1 MiB per stream.",
			pluginbinding.Effects(core.OperationEffectProcess),
			pluginbinding.Access(core.OperationAccessProvider),
			pluginbinding.Risk(core.OperationRiskHigh),
			pluginbinding.Idempotency(core.OperationNonIdempotent),
		),
		map[string]any{"namespace": "default", "name": "my-pod-abc123", "container": "app", "command": []string{"sh", "-c", "ls -la /tmp"}, "timeout_seconds": 30},
	)
}

func deploymentScaleSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[DeploymentScaleInput, DeploymentScaleResult](
			OperationDeploymentScale,
			"Scale a Kubernetes deployment to a desired replica count via the scale subresource.",
			pluginbinding.Effects(core.OperationEffectWrite),
			pluginbinding.Access(core.OperationAccessProvider),
			pluginbinding.Risk(core.OperationRiskHigh),
			pluginbinding.Idempotency(core.OperationIdempotent),
		),
		map[string]any{"namespace": "default", "name": "my-app", "replicas": 3},
	)
}

func deploymentRestartSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[DeploymentRestartInput, DeploymentRestartResult](
			OperationDeploymentRestart,
			"Rolling-restart a Kubernetes deployment (kubectl rollout restart) by bumping the pod-template restart annotation.",
			pluginbinding.Effects(core.OperationEffectWrite),
			pluginbinding.Access(core.OperationAccessProvider),
			pluginbinding.Risk(core.OperationRiskHigh),
			pluginbinding.Idempotency(core.OperationNonIdempotent),
		),
		map[string]any{"namespace": "default", "name": "my-app"},
	)
}

func inventoryDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[InventorySearchInput, InventorySearchResult](
		DatasourceInventory,
		EntityResource,
		"Kubernetes namespaces, services, pods, deployments, and containers.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.DatasourceAccess(core.OperationAccessProvider),
		pluginbinding.Completion(
			"Kubernetes contexts, endpoints, namespaces, resource names, pod names, container names, and labels.",
			"endpoint_ref",
			"context",
			"namespace",
			"id",
			"title",
			"name",
			"pod",
			"containers",
			"labels",
		),
	)
}

func kubernetesReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.Effects(core.OperationEffectRead),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}
