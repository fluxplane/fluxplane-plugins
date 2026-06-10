package kubernetes

import (
	"context"
	"sort"
	"strings"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	corev1 "k8s.io/api/core/v1"
)

type EventListInput struct {
	EndpointRef  string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Kubernetes cluster endpoint ref resolved by the host."`
	URL          string `json:"url,omitempty" jsonschema:"description=Kubernetes endpoint URL."`
	Context      string `json:"context,omitempty" jsonschema:"description=Kubeconfig context override."`
	Namespace    string `json:"namespace,omitempty" jsonschema:"description=Namespace filter. Empty means all namespaces."`
	Name         string `json:"name,omitempty" jsonschema:"description=Filter to events about this object (involvedObject.name)."`
	Kind         string `json:"kind,omitempty" jsonschema:"description=Filter to events about this object kind (e.g. Pod, Deployment, Node)."`
	WarningsOnly bool   `json:"warnings_only,omitempty" jsonschema:"description=Only return Warning events."`
	Limit        int    `json:"limit,omitempty" jsonschema:"description=Maximum events to return (default 50), newest first."`
}

type EventRecord struct {
	Type         string `json:"type,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Message      string `json:"message,omitempty"`
	Count        int32  `json:"count,omitempty"`
	FirstSeen    string `json:"first_seen,omitempty"`
	LastSeen     string `json:"last_seen,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	InvolvedKind string `json:"involved_kind,omitempty"`
	InvolvedName string `json:"involved_name,omitempty"`
	Source       string `json:"source,omitempty"`
}

type EventListResult struct {
	Count  int           `json:"count"`
	Events []EventRecord `json:"events"`
}

type NodeListInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Kubernetes cluster endpoint ref resolved by the host."`
	URL         string `json:"url,omitempty" jsonschema:"description=Kubernetes endpoint URL."`
	Context     string `json:"context,omitempty" jsonschema:"description=Kubeconfig context override."`
	Query       string `json:"query,omitempty" jsonschema:"description=Substring filter on node name or role."`
	Limit       int    `json:"limit,omitempty" jsonschema:"description=Maximum nodes to return."`
}

type NodeRecord struct {
	Name           string   `json:"name"`
	Roles          []string `json:"roles,omitempty"`
	Ready          bool     `json:"ready"`
	Status         string   `json:"status,omitempty"`
	Problems       []string `json:"problems,omitempty"`
	KubeletVersion string   `json:"kubelet_version,omitempty"`
	InternalIP     string   `json:"internal_ip,omitempty"`
	OSImage        string   `json:"os_image,omitempty"`
	Architecture   string   `json:"architecture,omitempty"`
	CPU            string   `json:"cpu,omitempty"`
	Memory         string   `json:"memory,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
}

type NodeListResult struct {
	Count int          `json:"count"`
	Nodes []NodeRecord `json:"nodes"`
}

type PodExecInput struct {
	EndpointRef    string   `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Kubernetes cluster endpoint ref resolved by the host."`
	URL            string   `json:"url,omitempty" jsonschema:"description=Kubernetes endpoint URL."`
	Context        string   `json:"context,omitempty" jsonschema:"description=Kubeconfig context override."`
	Namespace      string   `json:"namespace,omitempty" jsonschema:"description=Pod namespace."`
	Name           string   `json:"name,omitempty" jsonschema:"description=Pod name."`
	Container      string   `json:"container,omitempty" jsonschema:"description=Container name. Empty uses Kubernetes default selection."`
	Command        []string `json:"command,omitempty" jsonschema:"description=Command argv to run (no shell; use [\"sh\",\"-c\",\"...\"] for shell syntax)."`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" jsonschema:"description=Command timeout in seconds (default 30, max 300)."`
}

type PodExecResult struct {
	Namespace       string   `json:"namespace"`
	Name            string   `json:"name"`
	Container       string   `json:"container,omitempty"`
	Command         []string `json:"command"`
	Transport       string   `json:"transport,omitempty"` // host-spdy (through host conn.dial) | websocket (direct)
	ExitCode        int      `json:"exit_code"`
	Stdout          string   `json:"stdout,omitempty"`
	Stderr          string   `json:"stderr,omitempty"`
	StdoutTruncated bool     `json:"stdout_truncated,omitempty"`
	StderrTruncated bool     `json:"stderr_truncated,omitempty"`
	DurationMS      int64    `json:"duration_ms"`
}

type DeploymentScaleInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Kubernetes cluster endpoint ref resolved by the host."`
	URL         string `json:"url,omitempty" jsonschema:"description=Kubernetes endpoint URL."`
	Context     string `json:"context,omitempty" jsonschema:"description=Kubeconfig context override."`
	Namespace   string `json:"namespace,omitempty" jsonschema:"description=Deployment namespace."`
	Name        string `json:"name,omitempty" jsonschema:"description=Deployment name."`
	Replicas    *int32 `json:"replicas,omitempty" jsonschema:"description=Desired replica count (>= 0)."`
}

type DeploymentScaleResult struct {
	OK               bool   `json:"ok"`
	Namespace        string `json:"namespace"`
	Name             string `json:"name"`
	PreviousReplicas int32  `json:"previous_replicas"`
	Replicas         int32  `json:"replicas"`
}

type DeploymentRestartInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Kubernetes cluster endpoint ref resolved by the host."`
	URL         string `json:"url,omitempty" jsonschema:"description=Kubernetes endpoint URL."`
	Context     string `json:"context,omitempty" jsonschema:"description=Kubeconfig context override."`
	Namespace   string `json:"namespace,omitempty" jsonschema:"description=Deployment namespace."`
	Name        string `json:"name,omitempty" jsonschema:"description=Deployment name."`
}

type DeploymentRestartResult struct {
	OK          bool   `json:"ok"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	RestartedAt string `json:"restarted_at"`
}

func (s Service) EventList(ctx pluginbinding.Context, input EventListInput) (EventListResult, error) {
	items, err := s.events(ctx)(context.Background(), input)
	if err != nil {
		return EventListResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	records := eventRecords(items)
	if input.WarningsOnly {
		filtered := records[:0]
		for _, record := range records {
			if record.Type == corev1.EventTypeWarning {
				filtered = append(filtered, record)
			}
		}
		records = filtered
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].LastSeen > records[j].LastSeen
	})
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	records = limitSlice(records, limit)
	return EventListResult{Count: len(records), Events: records}, nil
}

func (s Service) NodeList(ctx pluginbinding.Context, input NodeListInput) (NodeListResult, error) {
	items, err := s.nodes(ctx)(context.Background(), input)
	if err != nil {
		return NodeListResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	records := nodeRecords(items)
	if query := strings.ToLower(strings.TrimSpace(input.Query)); query != "" {
		filtered := records[:0]
		for _, record := range records {
			if strings.Contains(strings.ToLower(record.Name), query) || strings.Contains(strings.ToLower(strings.Join(record.Roles, " ")), query) {
				filtered = append(filtered, record)
			}
		}
		records = filtered
	}
	records = limitSlice(records, input.Limit)
	return NodeListResult{Count: len(records), Nodes: records}, nil
}

func (s Service) PodExec(ctx pluginbinding.Context, input PodExecInput) (PodExecResult, error) {
	if strings.TrimSpace(input.Namespace) == "" || strings.TrimSpace(input.Name) == "" {
		return PodExecResult{}, pluginbinding.Fail("bad_input", "namespace and name are required")
	}
	if len(input.Command) == 0 {
		return PodExecResult{}, pluginbinding.Fail("bad_input", "command is required")
	}
	result, err := s.exec(ctx)(context.Background(), input)
	if err != nil {
		return PodExecResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	return result, nil
}

func (s Service) DeploymentScale(ctx pluginbinding.Context, input DeploymentScaleInput) (DeploymentScaleResult, error) {
	if strings.TrimSpace(input.Namespace) == "" || strings.TrimSpace(input.Name) == "" {
		return DeploymentScaleResult{}, pluginbinding.Fail("bad_input", "namespace and name are required")
	}
	if input.Replicas == nil || *input.Replicas < 0 {
		return DeploymentScaleResult{}, pluginbinding.Fail("bad_input", "replicas is required and must be >= 0")
	}
	result, err := s.scaleDeployment(ctx)(context.Background(), input)
	if err != nil {
		return DeploymentScaleResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	return result, nil
}

func (s Service) DeploymentRestart(ctx pluginbinding.Context, input DeploymentRestartInput) (DeploymentRestartResult, error) {
	if strings.TrimSpace(input.Namespace) == "" || strings.TrimSpace(input.Name) == "" {
		return DeploymentRestartResult{}, pluginbinding.Fail("bad_input", "namespace and name are required")
	}
	result, err := s.restartDeployment(ctx)(context.Background(), input)
	if err != nil {
		return DeploymentRestartResult{}, pluginbinding.Errorf("kubernetes", "%s", err)
	}
	return result, nil
}

func (s Service) events(ctx pluginbinding.Context) func(context.Context, EventListInput) ([]corev1.Event, error) {
	if s.Events != nil {
		return s.Events
	}
	return func(_ context.Context, input EventListInput) ([]corev1.Event, error) {
		return HostKubeEvents(ctx, input)
	}
}

func (s Service) nodes(ctx pluginbinding.Context) func(context.Context, NodeListInput) ([]corev1.Node, error) {
	if s.Nodes != nil {
		return s.Nodes
	}
	return func(_ context.Context, input NodeListInput) ([]corev1.Node, error) {
		return HostKubeNodes(ctx, input)
	}
}

func (s Service) exec(ctx pluginbinding.Context) func(context.Context, PodExecInput) (PodExecResult, error) {
	if s.Exec != nil {
		return s.Exec
	}
	return func(_ context.Context, input PodExecInput) (PodExecResult, error) {
		return HostKubePodExec(ctx, input)
	}
}

func (s Service) scaleDeployment(ctx pluginbinding.Context) func(context.Context, DeploymentScaleInput) (DeploymentScaleResult, error) {
	if s.ScaleDeployment != nil {
		return s.ScaleDeployment
	}
	return func(_ context.Context, input DeploymentScaleInput) (DeploymentScaleResult, error) {
		return HostKubeDeploymentScale(ctx, input)
	}
}

func (s Service) restartDeployment(ctx pluginbinding.Context) func(context.Context, DeploymentRestartInput) (DeploymentRestartResult, error) {
	if s.RestartDeployment != nil {
		return s.RestartDeployment
	}
	return func(_ context.Context, input DeploymentRestartInput) (DeploymentRestartResult, error) {
		return HostKubeDeploymentRestart(ctx, input)
	}
}

func eventRecords(items []corev1.Event) []EventRecord {
	records := make([]EventRecord, 0, len(items))
	for _, item := range items {
		records = append(records, EventRecord{
			Type:         item.Type,
			Reason:       item.Reason,
			Message:      strings.TrimSpace(item.Message),
			Count:        eventCount(item),
			FirstSeen:    eventFirstSeen(item),
			LastSeen:     eventLastSeen(item),
			Namespace:    item.Namespace,
			InvolvedKind: item.InvolvedObject.Kind,
			InvolvedName: item.InvolvedObject.Name,
			Source:       firstNonEmpty(item.Source.Component, item.ReportingController),
		})
	}
	return records
}

// eventLastSeen prefers the classic lastTimestamp but falls back to the
// events.k8s.io series/eventTime fields modern controllers populate.
func eventLastSeen(item corev1.Event) string {
	if item.Series != nil && !item.Series.LastObservedTime.IsZero() {
		return item.Series.LastObservedTime.UTC().Format(timeFormatRFC3339)
	}
	if !item.LastTimestamp.IsZero() {
		return item.LastTimestamp.UTC().Format(timeFormatRFC3339)
	}
	if !item.EventTime.IsZero() {
		return item.EventTime.UTC().Format(timeFormatRFC3339)
	}
	return ""
}

func eventFirstSeen(item corev1.Event) string {
	if !item.FirstTimestamp.IsZero() {
		return item.FirstTimestamp.UTC().Format(timeFormatRFC3339)
	}
	if !item.EventTime.IsZero() {
		return item.EventTime.UTC().Format(timeFormatRFC3339)
	}
	return ""
}

func eventCount(item corev1.Event) int32 {
	if item.Series != nil && item.Series.Count > 0 {
		return item.Series.Count
	}
	return item.Count
}

func nodeRecords(items []corev1.Node) []NodeRecord {
	records := make([]NodeRecord, 0, len(items))
	for _, item := range items {
		records = append(records, nodeRecord(item))
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })
	return records
}

func nodeRecord(node corev1.Node) NodeRecord {
	record := NodeRecord{
		Name:           node.Name,
		Roles:          nodeRoles(node.Labels),
		KubeletVersion: node.Status.NodeInfo.KubeletVersion,
		OSImage:        node.Status.NodeInfo.OSImage,
		Architecture:   node.Status.NodeInfo.Architecture,
		CPU:            node.Status.Capacity.Cpu().String(),
		Memory:         node.Status.Capacity.Memory().String(),
		CreatedAt:      node.CreationTimestamp.UTC().Format(timeFormatRFC3339),
	}
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			record.InternalIP = address.Address
			break
		}
	}
	record.Status = "NotReady"
	for _, condition := range node.Status.Conditions {
		switch {
		case condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue:
			record.Ready = true
			record.Status = "Ready"
		case condition.Type != corev1.NodeReady && condition.Status == corev1.ConditionTrue:
			// Pressure/unavailability conditions are abnormal when true.
			record.Problems = append(record.Problems, string(condition.Type))
		}
	}
	if node.Spec.Unschedulable {
		record.Problems = append(record.Problems, "Unschedulable")
		record.Status += ",SchedulingDisabled"
	}
	return record
}

func nodeRoles(labels map[string]string) []string {
	var roles []string
	for label := range labels {
		if role, ok := strings.CutPrefix(label, "node-role.kubernetes.io/"); ok && role != "" {
			roles = append(roles, role)
		}
	}
	sort.Strings(roles)
	return roles
}

const timeFormatRFC3339 = "2006-01-02T15:04:05Z07:00"
