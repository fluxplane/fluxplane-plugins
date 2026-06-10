package kubernetes

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func HostKubeContexts() (ClusterListResult, error) {
	cfg, err := loadKubeConfig()
	if err != nil {
		return ClusterListResult{}, err
	}
	return clusterListFromKubeConfig(cfg), nil
}

func HostKubeClusterProbe(ctx pluginbinding.Context, input ClusterTestInput) (ClusterTestResult, error) {
	contextName, err := resolveKubeContext(ctx, input.EndpointRef, input.URL, input.Context)
	if err != nil {
		return ClusterTestResult{}, err
	}
	client, err := kubeClientForContext(ctx, contextName)
	if err != nil {
		return ClusterTestResult{}, err
	}
	start := time.Now()
	version, err := client.Discovery().ServerVersion()
	if err != nil {
		return ClusterTestResult{}, err
	}
	return ClusterTestResult{
		Context:       contextName,
		OK:            true,
		ServerVersion: strings.TrimSpace(version.GitVersion),
		Platform:      strings.TrimSpace(version.Platform),
		DurationMS:    time.Since(start).Milliseconds(),
	}, nil
}

func HostKubeNamespaces(ctx pluginbinding.Context, input InventoryInput) ([]corev1.Namespace, error) {
	client, err := kubeClientFromInventory(ctx, input)
	if err != nil {
		return nil, err
	}
	list, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func HostKubeServices(ctx pluginbinding.Context, input EndpointDiscoverInput) ([]corev1.Service, error) {
	client, err := kubeClientFromEndpointInput(ctx, input)
	if err != nil {
		return nil, err
	}
	list, err := client.CoreV1().Services(namespaceOrAll(input.Namespace)).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func HostKubeIngresses(ctx pluginbinding.Context, input EndpointDiscoverInput) ([]networkingv1.Ingress, error) {
	client, err := kubeClientFromEndpointInput(ctx, input)
	if err != nil {
		return nil, err
	}
	list, err := client.NetworkingV1().Ingresses(namespaceOrAll(input.Namespace)).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func HostKubePods(ctx pluginbinding.Context, input InventoryInput) ([]corev1.Pod, error) {
	client, err := kubeClientFromInventory(ctx, input)
	if err != nil {
		return nil, err
	}
	list, err := client.CoreV1().Pods(namespaceOrAll(input.Namespace)).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func HostKubeDeployments(ctx pluginbinding.Context, input InventoryInput) ([]appsv1.Deployment, error) {
	client, err := kubeClientFromInventory(ctx, input)
	if err != nil {
		return nil, err
	}
	list, err := client.AppsV1().Deployments(namespaceOrAll(input.Namespace)).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func HostKubeSecrets(ctx pluginbinding.Context, input EndpointDiscoverInput) ([]corev1.Secret, error) {
	client, err := kubeClientFromEndpointInput(ctx, input)
	if err != nil {
		return nil, err
	}
	list, err := client.CoreV1().Secrets(namespaceOrAll(input.Namespace)).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func HostKubePodLogs(ctx pluginbinding.Context, input PodLogsInput) (PodLogsResult, error) {
	contextName, err := resolveKubeContext(ctx, input.EndpointRef, input.URL, input.Context)
	if err != nil {
		return PodLogsResult{}, err
	}
	client, err := kubeClientForContext(ctx, contextName)
	if err != nil {
		return PodLogsResult{}, err
	}
	namespace := strings.TrimSpace(input.Namespace)
	name := strings.TrimSpace(input.Name)
	if namespace == "" {
		return PodLogsResult{}, fmt.Errorf("namespace is required")
	}
	if name == "" {
		return PodLogsResult{}, fmt.Errorf("pod name is required")
	}
	bounds, err := podLogBounds(input)
	if err != nil {
		return PodLogsResult{}, err
	}
	options := &corev1.PodLogOptions{
		Container:  strings.TrimSpace(input.Container),
		Follow:     false,
		Previous:   input.Previous,
		Timestamps: input.Timestamps || bounds.Until != nil,
		TailLines:  bounds.TailLines,
		LimitBytes: bounds.LimitBytes,
	}
	if bounds.SinceSeconds != nil {
		options.SinceSeconds = bounds.SinceSeconds
	}
	if bounds.SinceTime != nil {
		options.SinceTime = bounds.SinceTime
	}
	stream, err := client.CoreV1().Pods(namespace).GetLogs(name, options).Stream(context.Background())
	if err != nil {
		return PodLogsResult{}, err
	}
	defer stream.Close()
	maxBytes := int64(4 * 1024 * 1024)
	if bounds.LimitBytes != nil && *bounds.LimitBytes > 0 {
		maxBytes = *bounds.LimitBytes + 64*1024
	}
	data, err := io.ReadAll(io.LimitReader(stream, maxBytes))
	if err != nil {
		return PodLogsResult{}, err
	}
	text := filterPodLogText(strings.TrimRight(string(data), "\n"), bounds, input.Timestamps)
	lines := splitNonEmptyLines(text)
	return PodLogsResult{
		Namespace:  namespace,
		Name:       name,
		Container:  strings.TrimSpace(input.Container),
		Lines:      lines,
		Text:       text,
		LineCount:  len(lines),
		TailLines:  valueOrZero(bounds.TailLines),
		LimitBytes: valueOrZero(bounds.LimitBytes),
		Since:      strings.TrimSpace(input.Since),
		Until:      strings.TrimSpace(input.Until),
		Previous:   input.Previous,
		Timestamps: input.Timestamps,
	}, nil
}

func HostKubePortForwardStart(ctx pluginbinding.Context, input PortForwardStartInput) (PortForwardResult, error) {
	if ctx.Host == nil {
		return PortForwardResult{}, fmt.Errorf("host client is unavailable")
	}
	contextName, err := resolveKubeContext(ctx, input.EndpointRef, input.URL, input.Context)
	if err != nil {
		return PortForwardResult{}, err
	}
	namespace := strings.TrimSpace(input.Namespace)
	if namespace == "" {
		return PortForwardResult{}, fmt.Errorf("namespace is required")
	}
	resource := normalizedPortForwardResource(input)
	if resource == "" {
		return PortForwardResult{}, fmt.Errorf("resource is required")
	}
	if input.RemotePort <= 0 {
		return PortForwardResult{}, fmt.Errorf("remote_port is required")
	}
	address := firstNonEmpty(input.Address, "127.0.0.1")
	localPort := input.LocalPort
	if localPort <= 0 {
		localPort, err = allocateLocalPort(address)
		if err != nil {
			return PortForwardResult{}, err
		}
	}
	duration := input.DurationSeconds
	if duration <= 0 {
		duration = 3600
	}
	if duration > 28800 {
		duration = 28800
	}
	command, err := os.Executable()
	if err != nil {
		return PortForwardResult{}, err
	}
	id := portForwardID(namespace, resource, localPort, input.RemotePort)
	args := kubePortForwardHelperArgs(contextName, namespace, resource, address, localPort, input.RemotePort, duration)
	started, err := ctx.Host.ProcessStart(pluginbinding.ProcessStartRequest{
		ID:        id,
		Command:   command,
		Args:      args,
		Label:     filepathBase(command) + " " + strings.Join(args, " "),
		Group:     "kubernetes.portforward",
		Tags:      []string{"kubernetes", "port-forward"},
		StartedOK: "Forwarding from",
		TimeoutMS: 5000,
		Metadata: map[string]string{
			"context":     contextName,
			"namespace":   namespace,
			"resource":    resource,
			"local_port":  strconv.Itoa(localPort),
			"remote_port": strconv.Itoa(input.RemotePort),
		},
	})
	if err != nil {
		return PortForwardResult{}, err
	}
	return PortForwardResult{
		ID: id, EndpointRef: strings.TrimSpace(input.EndpointRef), Context: contextName, Namespace: namespace, Resource: resource,
		Address: address, LocalPort: localPort, RemotePort: input.RemotePort, LocalURL: "http://" + address + ":" + strconv.Itoa(localPort),
		PID: started.PID, ProcessGroup: started.ProcessGroup, DurationSeconds: duration, ExpiresAt: time.Now().Add(time.Duration(duration) * time.Second).UTC(),
		LogPath: started.LogPath, Command: append([]string{command}, args...),
	}, nil
}

func HostKubePortForwardStop(ctx pluginbinding.Context, input PortForwardStopInput) (PortForwardStopResult, error) {
	if ctx.Host == nil {
		return PortForwardStopResult{}, fmt.Errorf("host client is unavailable")
	}
	stopped, err := ctx.Host.ProcessStop(pluginbinding.ProcessStopRequest{ID: input.ID, PID: input.PID, ProcessGroup: input.ProcessGroup, Signal: "SIGTERM"})
	if err != nil {
		return PortForwardStopResult{}, err
	}
	return PortForwardStopResult{ID: stopped.ID, Stopped: stopped.Stopped, Signal: stopped.Signal, Error: stopped.Error}, nil
}

// HostKubePortForwardList returns the managed port-forwards from the host
// process store (group kubernetes.portforward), each probed for liveness so a
// dead forward is recognizable, optionally filtered by namespace/context.
func HostKubePortForwardList(ctx pluginbinding.Context, input PortForwardListInput) (PortForwardListResult, error) {
	lister, ok := ctx.Host.(pluginbinding.ProcessLister)
	if !ok || ctx.Host == nil {
		return PortForwardListResult{}, fmt.Errorf("host does not support the process.list capability required by portforward.list (upgrade fluxplane-plugin)")
	}
	listed, err := lister.ProcessList(pluginbinding.ProcessListRequest{Group: "kubernetes.portforward"})
	if err != nil {
		return PortForwardListResult{}, err
	}
	namespace := strings.TrimSpace(input.Namespace)
	contextName := strings.TrimSpace(input.Context)
	out := PortForwardListResult{Forwards: []PortForwardRecord{}}
	for _, process := range listed.Processes {
		meta := process.Metadata
		if namespace != "" && meta["namespace"] != namespace {
			continue
		}
		if contextName != "" && meta["context"] != contextName {
			continue
		}
		record := PortForwardRecord{
			ID:        process.ID,
			Context:   meta["context"],
			Namespace: meta["namespace"],
			Resource:  meta["resource"],
			PID:       process.PID,
			Alive:     process.Alive,
			StartedAt: process.StartedAt,
			LogPath:   process.LogPath,
		}
		record.LocalPort, _ = strconv.Atoi(meta["local_port"])
		record.RemotePort, _ = strconv.Atoi(meta["remote_port"])
		if record.LocalPort > 0 {
			record.LocalURL = "http://127.0.0.1:" + strconv.Itoa(record.LocalPort)
		}
		out.Forwards = append(out.Forwards, record)
	}
	out.Count = len(out.Forwards)
	return out, nil
}

func kubeClientFromInventory(ctx pluginbinding.Context, input InventoryInput) (kubernetes.Interface, error) {
	contextName, err := resolveKubeContext(ctx, input.EndpointRef, input.URL, input.Context)
	if err != nil {
		return nil, err
	}
	return kubeClientForContext(ctx, contextName)
}

func kubeClientFromEndpointInput(ctx pluginbinding.Context, input EndpointDiscoverInput) (kubernetes.Interface, error) {
	contextName, err := resolveKubeContext(ctx, input.EndpointRef, input.URL, input.Context)
	if err != nil {
		return nil, err
	}
	return kubeClientForContext(ctx, contextName)
}

func kubeClientForContext(ctx pluginbinding.Context, contextName string) (kubernetes.Interface, error) {
	restConfig, err := kubeRestConfigForContext(ctx, contextName)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restConfig)
}

func kubeRestConfigForContext(ctx pluginbinding.Context, contextName string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if contextName = strings.TrimSpace(contextName); contextName != "" {
		overrides.CurrentContext = contextName
	}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, err
	}
	// Route the API-server connection through the host conn capability so the
	// plugin performs no direct network IO; client-go still terminates TLS using
	// the kubeconfig CA over the host-dialed stream.
	if dial := hostDial(ctx); dial != nil {
		restConfig.Dial = dial
	}
	return restConfig, nil
}

// hostDial returns a dialer backed by the host conn capability, or nil when the
// host does not advertise it (so client-go falls back to its own dialer).
func hostDial(ctx pluginbinding.Context) func(context.Context, string, string) (net.Conn, error) {
	if ctx.Host == nil {
		return nil
	}
	if _, ok := ctx.Host.(pluginbinding.ConnDialer); !ok {
		return nil
	}
	return pluginbinding.HostDialer(ctx.Host)
}

func loadKubeConfig() (*clientcmdapi.Config, error) {
	return clientcmd.NewDefaultClientConfigLoadingRules().Load()
}

func clusterListFromKubeConfig(cfg *clientcmdapi.Config) ClusterListResult {
	if cfg == nil {
		return ClusterListResult{}
	}
	out := ClusterListResult{Contexts: make([]ClusterContext, 0, len(cfg.Contexts))}
	for name, item := range cfg.Contexts {
		if strings.TrimSpace(name) == "" || item == nil {
			continue
		}
		out.Contexts = append(out.Contexts, ClusterContext{
			Name:    strings.TrimSpace(name),
			Current: name == strings.TrimSpace(cfg.CurrentContext),
			Cluster: strings.TrimSpace(item.Cluster),
			User:    strings.TrimSpace(item.AuthInfo),
		})
	}
	sortClusterContexts(out.Contexts)
	return out
}

func sortClusterContexts(contexts []ClusterContext) {
	sort.Slice(contexts, func(i, j int) bool {
		if contexts[i].Current != contexts[j].Current {
			return contexts[i].Current
		}
		return contexts[i].Name < contexts[j].Name
	})
}

func resolveKubeContext(ctx pluginbinding.Context, endpointRef, rawURL, contextName string) (string, error) {
	if contextName = strings.TrimSpace(contextName); contextName != "" {
		return contextName, nil
	}
	if parsed := clusterContextFromEndpointURL(rawURL); parsed != "" {
		return parsed, nil
	}
	if endpointRef = strings.TrimSpace(endpointRef); endpointRef == "" {
		return "", nil
	}
	if ctx.Host == nil {
		return "", fmt.Errorf("host client is unavailable")
	}
	endpoint, err := ctx.Host.ResolveEndpoint(endpointRef)
	if err != nil {
		return "", err
	}
	if parsed := clusterContextFromEndpointURL(endpoint.URL); parsed != "" {
		return parsed, nil
	}
	return "", fmt.Errorf("endpoint %q does not contain a Kubernetes context URL", endpointRef)
}

func namespaceOrAll(namespace string) string {
	if namespace = strings.TrimSpace(namespace); namespace != "" {
		return namespace
	}
	return metav1.NamespaceAll
}

func kubePortForwardHelperArgs(contextName, namespace, resource, address string, localPort, remotePort, durationSeconds int) []string {
	out := []string{
		kubePortForwardHelperCommand,
		"--namespace", namespace,
		"--resource", resource,
		"--address", address,
		"--local-port", strconv.Itoa(localPort),
		"--remote-port", strconv.Itoa(remotePort),
		"--duration-seconds", strconv.Itoa(durationSeconds),
	}
	if contextName = strings.TrimSpace(contextName); contextName != "" {
		out = append(out, "--context", contextName)
	}
	return out
}

func filepathBase(path string) string {
	base := path
	if idx := strings.LastIndexAny(path, `/\`); idx >= 0 {
		base = path[idx+1:]
	}
	if strings.TrimSpace(base) == "" {
		return "fluxplane-plugin-kubernetes"
	}
	return base
}

func allocateLocalPort(address string) (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(address, "0"))
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("allocated listener is not TCP")
	}
	return addr.Port, nil
}

func splitNonEmptyLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	out := lines[:0]
	for _, line := range lines {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
