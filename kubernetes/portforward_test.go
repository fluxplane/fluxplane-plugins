package kubernetes

import (
	"context"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolveServicePortForwardTargetUsesSelectorAndNamedTargetPort(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "kube-dns", Namespace: "kube-system"},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"k8s-app": "kube-dns"},
				Ports: []corev1.ServicePort{{
					Name:       "metrics",
					Port:       9153,
					TargetPort: intstr.FromString("metrics"),
				}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "coredns-abc", Namespace: "kube-system", Labels: map[string]string{"k8s-app": "kube-dns"}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  "coredns",
				Ports: []corev1.ContainerPort{{Name: "metrics", ContainerPort: 9153}},
			}}},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)

	target, err := resolvePortForwardTarget(context.Background(), client, "kube-system", "service/kube-dns", 9153)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if target.PodName != "coredns-abc" || target.Port != 9153 {
		t.Fatalf("target = %#v", target)
	}
}

func TestResolveDeploymentPortForwardTargetUsesDeploymentSelector(t *testing.T) {
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "apps"},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "api-123", Namespace: "apps", Labels: map[string]string{"app": "api"}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)

	target, err := resolvePortForwardTarget(context.Background(), client, "apps", "deployment/api", 8080)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if target.PodName != "api-123" || target.Port != 8080 {
		t.Fatalf("target = %#v", target)
	}
}

func TestResolveServicePortForwardTargetRejectsSelectorlessService(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 443, TargetPort: intstr.FromInt32(443)}}},
	})

	if _, err := resolvePortForwardTarget(context.Background(), client, "default", "service/external", 443); err == nil {
		t.Fatal("resolve target error is nil, want selectorless service error")
	}
}

func TestPodLogsSelectorValidation(t *testing.T) {
	// name XOR selector is enforced before any cluster call.
	_, err := HostKubePodLogs(pluginbinding.Context{}, PodLogsInput{Namespace: "apps"})
	if err == nil || err.Error() != "pod name or selector is required" {
		t.Fatalf("err = %v", err)
	}
	_, err = HostKubePodLogs(pluginbinding.Context{}, PodLogsInput{Namespace: "apps", Name: "a", Selector: "app=x"})
	if err == nil || err.Error() != "name and selector are mutually exclusive" {
		t.Fatalf("err = %v", err)
	}
}
