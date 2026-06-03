package kubernetes

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const kubePortForwardHelperCommand = "__fluxplane-kubernetes-port-forward"

func PortForwardHelperCommand() string {
	return kubePortForwardHelperCommand
}

type kubePortForwardOptions struct {
	Context    string
	Namespace  string
	Resource   string
	Address    string
	LocalPort  int
	RemotePort int
	Duration   int
}

func RunKubePortForwardCommand(ctx context.Context, args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet(kubePortForwardHelperCommand, flag.ContinueOnError)
	flags.SetOutput(errOut)
	var opts kubePortForwardOptions
	flags.StringVar(&opts.Context, "context", "", "kubeconfig context")
	flags.StringVar(&opts.Namespace, "namespace", "", "namespace")
	flags.StringVar(&opts.Resource, "resource", "", "resource")
	flags.StringVar(&opts.Address, "address", "127.0.0.1", "local bind address")
	flags.IntVar(&opts.LocalPort, "local-port", 0, "local port")
	flags.IntVar(&opts.RemotePort, "remote-port", 0, "remote port")
	flags.IntVar(&opts.Duration, "duration-seconds", 0, "maximum duration in seconds")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return RunKubePortForward(ctx, opts, out, errOut)
}

func RunKubePortForward(ctx context.Context, opts kubePortForwardOptions, out, errOut io.Writer) error {
	opts.Context = strings.TrimSpace(opts.Context)
	opts.Namespace = strings.TrimSpace(opts.Namespace)
	opts.Resource = strings.TrimSpace(opts.Resource)
	opts.Address = strings.TrimSpace(opts.Address)
	if opts.Address == "" {
		opts.Address = "127.0.0.1"
	}
	if opts.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if opts.Resource == "" {
		return fmt.Errorf("resource is required")
	}
	if opts.LocalPort <= 0 {
		return fmt.Errorf("local port is required")
	}
	if opts.RemotePort <= 0 {
		return fmt.Errorf("remote port is required")
	}
	if opts.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.Duration)*time.Second)
		defer cancel()
	}
	restConfig, client, err := kubeRESTClientForContext(opts.Context)
	if err != nil {
		return err
	}
	target, err := resolvePortForwardTarget(ctx, client, opts.Namespace, opts.Resource, opts.RemotePort)
	if err != nil {
		return err
	}
	err = runPodPortForward(ctx, restConfig, client, opts.Namespace, target.PodName, opts.Address, opts.LocalPort, target.Port, out, errOut)
	if errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	return err
}

type portForwardTarget struct {
	PodName string
	Port    int
}

func resolvePortForwardTarget(ctx context.Context, client kubernetes.Interface, namespace, resource string, remotePort int) (portForwardTarget, error) {
	resourceType, name := splitPortForwardResource(resource)
	switch resourceType {
	case "pod", "pods", "po":
		if name == "" {
			return portForwardTarget{}, fmt.Errorf("pod name is required")
		}
		pod, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return portForwardTarget{}, err
		}
		if !podCanPortForward(pod) {
			return portForwardTarget{}, fmt.Errorf("pod %s/%s is not running", namespace, name)
		}
		return portForwardTarget{PodName: pod.Name, Port: remotePort}, nil
	case "service", "services", "svc":
		return resolveServicePortForwardTarget(ctx, client, namespace, name, remotePort)
	case "deployment", "deployments", "deploy":
		return resolveDeploymentPortForwardTarget(ctx, client, namespace, name, remotePort)
	default:
		return portForwardTarget{}, fmt.Errorf("unsupported port-forward resource %q", resourceType)
	}
}

func resolveServicePortForwardTarget(ctx context.Context, client kubernetes.Interface, namespace, name string, remotePort int) (portForwardTarget, error) {
	if name == "" {
		return portForwardTarget{}, fmt.Errorf("service name is required")
	}
	service, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return portForwardTarget{}, err
	}
	if len(service.Spec.Selector) == 0 {
		return portForwardTarget{}, fmt.Errorf("service %s/%s has no selector; cannot select a pod for port-forward", namespace, name)
	}
	pod, err := firstForwardablePod(ctx, client, namespace, labels.SelectorFromSet(service.Spec.Selector).String())
	if err != nil {
		return portForwardTarget{}, err
	}
	port, err := servicePortForwardTargetPort(service, pod, remotePort)
	if err != nil {
		return portForwardTarget{}, err
	}
	return portForwardTarget{PodName: pod.Name, Port: port}, nil
}

func resolveDeploymentPortForwardTarget(ctx context.Context, client kubernetes.Interface, namespace, name string, remotePort int) (portForwardTarget, error) {
	if name == "" {
		return portForwardTarget{}, fmt.Errorf("deployment name is required")
	}
	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return portForwardTarget{}, err
	}
	selector, err := deploymentSelector(deployment)
	if err != nil {
		return portForwardTarget{}, err
	}
	pod, err := firstForwardablePod(ctx, client, namespace, selector)
	if err != nil {
		return portForwardTarget{}, err
	}
	return portForwardTarget{PodName: pod.Name, Port: remotePort}, nil
}

func firstForwardablePod(ctx context.Context, client kubernetes.Interface, namespace, selector string) (corev1.Pod, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return corev1.Pod{}, err
	}
	for _, pod := range pods.Items {
		if podCanPortForward(&pod) {
			return pod, nil
		}
	}
	return corev1.Pod{}, fmt.Errorf("no running pod matches selector %q in namespace %s", selector, namespace)
}

func servicePortForwardTargetPort(service *corev1.Service, pod corev1.Pod, remotePort int) (int, error) {
	for _, port := range service.Spec.Ports {
		if int(port.Port) != remotePort && int(port.TargetPort.IntVal) != remotePort {
			continue
		}
		return resolveTargetPort(port.TargetPort, pod, remotePort)
	}
	return 0, fmt.Errorf("service %s/%s does not expose port %d", service.Namespace, service.Name, remotePort)
}

func resolveTargetPort(target intstr.IntOrString, pod corev1.Pod, fallback int) (int, error) {
	if target.Type == intstr.Int && target.IntVal > 0 {
		return int(target.IntVal), nil
	}
	if target.Type == intstr.String && strings.TrimSpace(target.StrVal) != "" {
		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.Name == target.StrVal && port.ContainerPort > 0 {
					return int(port.ContainerPort), nil
				}
			}
		}
		return 0, fmt.Errorf("target port %q was not found on pod %s/%s", target.StrVal, pod.Namespace, pod.Name)
	}
	return fallback, nil
}

func deploymentSelector(deployment *appsv1.Deployment) (string, error) {
	if deployment.Spec.Selector == nil {
		return "", fmt.Errorf("deployment %s/%s has no selector", deployment.Namespace, deployment.Name)
	}
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return "", err
	}
	if selector.Empty() {
		return "", fmt.Errorf("deployment %s/%s has an empty selector", deployment.Namespace, deployment.Name)
	}
	return selector.String(), nil
}

func podCanPortForward(pod *corev1.Pod) bool {
	return pod != nil && pod.Status.Phase == corev1.PodRunning && pod.DeletionTimestamp == nil
}

func runPodPortForward(ctx context.Context, restConfig *rest.Config, client kubernetes.Interface, namespace, podName, address string, localPort, remotePort int, out, errOut io.Writer) error {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())
	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	forwarder, err := portforward.NewOnAddresses(
		dialer,
		[]string{address},
		[]string{strconv.Itoa(localPort) + ":" + strconv.Itoa(remotePort)},
		stopCh,
		readyCh,
		out,
		errOut,
	)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- forwarder.ForwardPorts() }()
	select {
	case <-ctx.Done():
		close(stopCh)
		err := <-done
		if err != nil {
			return err
		}
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func kubeRESTClientForContext(contextName string) (*rest.Config, kubernetes.Interface, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if contextName = strings.TrimSpace(contextName); contextName != "" {
		overrides.CurrentContext = contextName
	}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}
	return restConfig, client, nil
}

func splitPortForwardResource(resource string) (string, string) {
	resource = strings.Trim(strings.TrimSpace(resource), "/")
	resourceType, name, ok := strings.Cut(resource, "/")
	if !ok {
		return "service", resourceType
	}
	return strings.ToLower(strings.TrimSpace(resourceType)), strings.TrimSpace(name)
}
