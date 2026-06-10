package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	clientgoexec "k8s.io/client-go/util/exec"
	"k8s.io/streaming/pkg/httpstream"
)

func HostKubeEvents(ctx pluginbinding.Context, input EventListInput) ([]corev1.Event, error) {
	contextName, err := resolveKubeContext(ctx, input.EndpointRef, input.URL, input.Context)
	if err != nil {
		return nil, err
	}
	client, err := kubeClientForContext(ctx, contextName)
	if err != nil {
		return nil, err
	}
	var selectors []string
	if name := strings.TrimSpace(input.Name); name != "" {
		selectors = append(selectors, "involvedObject.name="+name)
	}
	if kind := strings.TrimSpace(input.Kind); kind != "" {
		selectors = append(selectors, "involvedObject.kind="+kind)
	}
	list, err := client.CoreV1().Events(namespaceOrAll(input.Namespace)).List(context.Background(), metav1.ListOptions{
		FieldSelector: strings.Join(selectors, ","),
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func HostKubeNodes(ctx pluginbinding.Context, input NodeListInput) ([]corev1.Node, error) {
	contextName, err := resolveKubeContext(ctx, input.EndpointRef, input.URL, input.Context)
	if err != nil {
		return nil, err
	}
	client, err := kubeClientForContext(ctx, contextName)
	if err != nil {
		return nil, err
	}
	list, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// HostKubePodExec runs a one-shot command in a pod container over the exec
// subresource (WebSocket with SPDY fallback, as kubectl does) and returns
// bounded stdout/stderr plus the command's exit code.
//
// Unlike the clientset API calls, the exec upgrade stream dials directly:
// client-go's SPDY and websocket round trippers build their own dialers and
// ignore rest.Config.Dial, so the host conn.dial capability cannot carry this
// stream without reimplementing the round tripper (same limitation that makes
// port-forward run as a helper subprocess via the host process capability).
func HostKubePodExec(ctx pluginbinding.Context, input PodExecInput) (PodExecResult, error) {
	contextName, err := resolveKubeContext(ctx, input.EndpointRef, input.URL, input.Context)
	if err != nil {
		return PodExecResult{}, err
	}
	restConfig, err := kubeRestConfigForContext(ctx, contextName)
	if err != nil {
		return PodExecResult{}, err
	}
	client, err := kubeClientForContext(ctx, contextName)
	if err != nil {
		return PodExecResult{}, err
	}
	namespace := strings.TrimSpace(input.Namespace)
	name := strings.TrimSpace(input.Name)
	if namespace == "" || name == "" {
		return PodExecResult{}, fmt.Errorf("namespace and name are required")
	}
	if len(input.Command) == 0 {
		return PodExecResult{}, fmt.Errorf("command is required")
	}
	container := strings.TrimSpace(input.Container)
	request := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   input.Command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)
	websocketExecutor, err := remotecommand.NewWebSocketExecutor(restConfig, "GET", request.URL().String())
	if err != nil {
		return PodExecResult{}, err
	}
	spdyExecutor, err := remotecommand.NewSPDYExecutor(restConfig, "POST", request.URL())
	if err != nil {
		return PodExecResult{}, err
	}
	executor, err := remotecommand.NewFallbackExecutor(websocketExecutor, spdyExecutor, httpstream.IsUpgradeFailure)
	if err != nil {
		return PodExecResult{}, err
	}
	timeout := time.Duration(podExecTimeoutSeconds(input.TimeoutSeconds)) * time.Second
	execCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	stdout := newBoundedBuffer(podExecMaxStreamBytes)
	stderr := newBoundedBuffer(podExecMaxStreamBytes)
	start := time.Now()
	streamErr := executor.StreamWithContext(execCtx, remotecommand.StreamOptions{Stdout: stdout, Stderr: stderr})
	result := PodExecResult{
		Namespace:       namespace,
		Name:            name,
		Container:       container,
		Command:         input.Command,
		Stdout:          stdout.String(),
		Stderr:          stderr.String(),
		StdoutTruncated: stdout.Truncated(),
		StderrTruncated: stderr.Truncated(),
		DurationMS:      time.Since(start).Milliseconds(),
	}
	if streamErr != nil {
		var exitErr clientgoexec.CodeExitError
		if errors.As(streamErr, &exitErr) {
			result.ExitCode = exitErr.Code
			return result, nil
		}
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return result, fmt.Errorf("exec timed out after %s", timeout)
		}
		return result, streamErr
	}
	return result, nil
}

func HostKubeDeploymentScale(ctx pluginbinding.Context, input DeploymentScaleInput) (DeploymentScaleResult, error) {
	contextName, err := resolveKubeContext(ctx, input.EndpointRef, input.URL, input.Context)
	if err != nil {
		return DeploymentScaleResult{}, err
	}
	client, err := kubeClientForContext(ctx, contextName)
	if err != nil {
		return DeploymentScaleResult{}, err
	}
	namespace := strings.TrimSpace(input.Namespace)
	name := strings.TrimSpace(input.Name)
	if namespace == "" || name == "" {
		return DeploymentScaleResult{}, fmt.Errorf("namespace and name are required")
	}
	if input.Replicas == nil || *input.Replicas < 0 {
		return DeploymentScaleResult{}, fmt.Errorf("replicas is required and must be >= 0")
	}
	scale, err := client.AppsV1().Deployments(namespace).GetScale(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return DeploymentScaleResult{}, err
	}
	previous := scale.Spec.Replicas
	scale.Spec.Replicas = *input.Replicas
	updated, err := client.AppsV1().Deployments(namespace).UpdateScale(context.Background(), name, scale, metav1.UpdateOptions{})
	if err != nil {
		return DeploymentScaleResult{}, err
	}
	return DeploymentScaleResult{
		OK:               true,
		Namespace:        namespace,
		Name:             name,
		PreviousReplicas: previous,
		Replicas:         updated.Spec.Replicas,
	}, nil
}

func HostKubeDeploymentRestart(ctx pluginbinding.Context, input DeploymentRestartInput) (DeploymentRestartResult, error) {
	contextName, err := resolveKubeContext(ctx, input.EndpointRef, input.URL, input.Context)
	if err != nil {
		return DeploymentRestartResult{}, err
	}
	client, err := kubeClientForContext(ctx, contextName)
	if err != nil {
		return DeploymentRestartResult{}, err
	}
	namespace := strings.TrimSpace(input.Namespace)
	name := strings.TrimSpace(input.Name)
	if namespace == "" || name == "" {
		return DeploymentRestartResult{}, fmt.Errorf("namespace and name are required")
	}
	restartedAt := time.Now().UTC().Format(time.RFC3339)
	// kubectl rollout restart: bump the pod-template restart annotation so the
	// deployment controller rolls new pods.
	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`, restartedAt)
	if _, err := client.AppsV1().Deployments(namespace).Patch(context.Background(), name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{}); err != nil {
		return DeploymentRestartResult{}, err
	}
	return DeploymentRestartResult{OK: true, Namespace: namespace, Name: name, RestartedAt: restartedAt}, nil
}

const podExecMaxStreamBytes = 1024 * 1024

func podExecTimeoutSeconds(value int) int {
	if value <= 0 {
		return 30
	}
	if value > 300 {
		return 300
	}
	return value
}

// boundedBuffer collects stream output up to a byte cap, recording whether
// anything was dropped.
type boundedBuffer struct {
	data      []byte
	max       int
	truncated bool
}

func newBoundedBuffer(max int) *boundedBuffer {
	return &boundedBuffer{max: max}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	remaining := b.max - len(b.data)
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		b.data = append(b.data, p[:remaining]...)
		b.truncated = true
		return len(p), nil
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *boundedBuffer) String() string { return string(b.data) }

func (b *boundedBuffer) Truncated() bool { return b.truncated }
