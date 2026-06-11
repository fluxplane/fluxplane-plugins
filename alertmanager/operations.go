package alertmanager

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type Service struct{}

func NewService() Service {
	return Service{}
}

type TargetInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"required,description=Registered Alertmanager endpoint ref resolved by the host."`
}

type TestInput struct {
	TargetInput
}

type TestResult struct {
	URL       string `json:"url"`
	Ready     bool   `json:"ready"`
	Version   string `json:"version,omitempty"`
	Cluster   string `json:"cluster_status,omitempty"`
	Peers     int    `json:"cluster_peers,omitempty"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
}

type AlertsInput struct {
	TargetInput
	// Filter takes Alertmanager matchers like severity="critical" or
	// namespace=~"prod-.*".
	Filter    []string `json:"filter,omitempty" jsonschema:"description=Label matchers\\, e.g. severity=\"critical\" or namespace=~\"lyse|core\"."`
	Active    *bool    `json:"active,omitempty" jsonschema:"description=Include active alerts. Defaults to true."`
	Silenced  *bool    `json:"silenced,omitempty" jsonschema:"description=Include silenced alerts. Defaults to false."`
	Inhibited *bool    `json:"inhibited,omitempty" jsonschema:"description=Include inhibited alerts. Defaults to false."`
	Limit     int      `json:"limit,omitempty" jsonschema:"description=Maximum alerts to return. Defaults to 200.,minimum=0"`
}

type Alert struct {
	Fingerprint string            `json:"fingerprint,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	State       string            `json:"state,omitempty"`
	SilencedBy  []string          `json:"silenced_by,omitempty"`
	InhibitedBy []string          `json:"inhibited_by,omitempty"`
	StartsAt    string            `json:"starts_at,omitempty"`
	EndsAt      string            `json:"ends_at,omitempty"`
	GeneratorURL string           `json:"generator_url,omitempty"`
}

type AlertsResult struct {
	URL       string  `json:"url"`
	Alerts    []Alert `json:"alerts"`
	Count     int     `json:"count"`
	Truncated bool    `json:"truncated,omitempty"`
}

type SilenceMatcher struct {
	Name    string `json:"name" jsonschema:"required,description=Label name to match."`
	Value   string `json:"value" jsonschema:"required,description=Value (or regex when is_regex) to match."`
	IsRegex bool   `json:"is_regex,omitempty" jsonschema:"description=Treat value as a regular expression."`
	IsEqual *bool  `json:"is_equal,omitempty" jsonschema:"description=Equality matcher. Defaults to true; false negates."`
}

type Silence struct {
	ID        string           `json:"id,omitempty"`
	Matchers  []SilenceMatcher `json:"matchers,omitempty"`
	StartsAt  string           `json:"starts_at,omitempty"`
	EndsAt    string           `json:"ends_at,omitempty"`
	CreatedBy string           `json:"created_by,omitempty"`
	Comment   string           `json:"comment,omitempty"`
	State     string           `json:"state,omitempty"`
}

type SilenceListInput struct {
	TargetInput
	State string `json:"state,omitempty" jsonschema:"description=Filter by silence state.,enum=active,enum=pending,enum=expired"`
}

type SilenceListResult struct {
	URL      string    `json:"url"`
	Silences []Silence `json:"silences"`
	Count    int       `json:"count"`
}

type SilenceCreateInput struct {
	TargetInput
	Matchers  []SilenceMatcher `json:"matchers,omitempty" jsonschema:"required,description=Label matchers selecting the alerts to silence."`
	Duration  string           `json:"duration,omitempty" jsonschema:"description=Silence duration from now (e.g. 30m\\, 2h). Defaults to 1h. Alternative to ends_at."`
	EndsAt    string           `json:"ends_at,omitempty" jsonschema:"description=Explicit RFC3339 end time. Overrides duration."`
	Comment   string           `json:"comment,omitempty" jsonschema:"required,description=Why this silence exists (shown in the Alertmanager UI)."`
	CreatedBy string           `json:"created_by,omitempty" jsonschema:"description=Creator label. Defaults to fluxplane-plugin."`
}

type SilenceCreateResult struct {
	URL     string `json:"url"`
	ID      string `json:"silence_id"`
	EndsAt  string `json:"ends_at,omitempty"`
	Created bool   `json:"created"`
}

type SilenceDeleteInput struct {
	TargetInput
	ID string `json:"id,omitempty" jsonschema:"required,description=Silence id to expire."`
}

type SilenceDeleteResult struct {
	URL     string `json:"url"`
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func (s Service) client(input TargetInput, host pluginbinding.HostClient) (string, Client, error) {
	endpointRef := strings.TrimSpace(input.EndpointRef)
	if endpointRef == "" {
		return "", Client{}, fmt.Errorf("endpoint_ref is required")
	}
	return endpointRef, Client{EndpointRef: endpointRef, Host: host}, nil
}

func (s Service) Test(ctx pluginbinding.Context, input TestInput) (TestResult, error) {
	target, client, err := s.client(input.TargetInput, ctx.Host)
	if err != nil {
		return TestResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start := time.Now()
	var status struct {
		Cluster struct {
			Status string `json:"status"`
			Peers  []any  `json:"peers"`
		} `json:"cluster"`
		VersionInfo struct {
			Version string `json:"version"`
		} `json:"versionInfo"`
	}
	err = client.get(context.Background(), "/api/v2/status", nil, &status)
	out := TestResult{URL: target, Ready: err == nil, LatencyMS: time.Since(start).Milliseconds()}
	if err != nil {
		out.Error = err.Error()
		return out, nil
	}
	out.Version = status.VersionInfo.Version
	out.Cluster = status.Cluster.Status
	out.Peers = len(status.Cluster.Peers)
	return out, nil
}

// apiAlert is the GET /api/v2/alerts wire shape.
type apiAlert struct {
	Fingerprint string            `json:"fingerprint"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    string            `json:"startsAt"`
	EndsAt      string            `json:"endsAt"`
	GeneratorURL string           `json:"generatorURL"`
	Status      struct {
		State       string   `json:"state"`
		SilencedBy  []string `json:"silencedBy"`
		InhibitedBy []string `json:"inhibitedBy"`
	} `json:"status"`
}

func (s Service) Alerts(ctx pluginbinding.Context, input AlertsInput) (AlertsResult, error) {
	target, client, err := s.client(input.TargetInput, ctx.Host)
	if err != nil {
		return AlertsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	values := url.Values{}
	values.Set("active", boolParam(input.Active, true))
	values.Set("silenced", boolParam(input.Silenced, false))
	values.Set("inhibited", boolParam(input.Inhibited, false))
	for _, matcher := range input.Filter {
		if matcher = strings.TrimSpace(matcher); matcher != "" {
			values.Add("filter", matcher)
		}
	}
	var wire []apiAlert
	if err := client.get(context.Background(), "/api/v2/alerts", values, &wire); err != nil {
		return AlertsResult{}, pluginbinding.Errorf("alertmanager", "%s", err)
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 200
	}
	alerts := []Alert{}
	truncated := false
	for _, alert := range wire {
		if len(alerts) >= limit {
			truncated = true
			break
		}
		alerts = append(alerts, Alert{
			Fingerprint:  alert.Fingerprint,
			Labels:       alert.Labels,
			Annotations:  alert.Annotations,
			State:        alert.Status.State,
			SilencedBy:   alert.Status.SilencedBy,
			InhibitedBy:  alert.Status.InhibitedBy,
			StartsAt:     alert.StartsAt,
			EndsAt:       alert.EndsAt,
			GeneratorURL: alert.GeneratorURL,
		})
	}
	return AlertsResult{URL: target, Alerts: alerts, Count: len(alerts), Truncated: truncated}, nil
}

// apiSilence is the /api/v2/silences wire shape.
type apiSilence struct {
	ID       string `json:"id"`
	Matchers []struct {
		Name    string `json:"name"`
		Value   string `json:"value"`
		IsRegex bool   `json:"isRegex"`
		IsEqual *bool  `json:"isEqual"`
	} `json:"matchers"`
	StartsAt  string `json:"startsAt"`
	EndsAt    string `json:"endsAt"`
	CreatedBy string `json:"createdBy"`
	Comment   string `json:"comment"`
	Status    struct {
		State string `json:"state"`
	} `json:"status"`
}

func silenceFromAPI(wire apiSilence) Silence {
	matchers := make([]SilenceMatcher, 0, len(wire.Matchers))
	for _, m := range wire.Matchers {
		matchers = append(matchers, SilenceMatcher{Name: m.Name, Value: m.Value, IsRegex: m.IsRegex, IsEqual: m.IsEqual})
	}
	return Silence{
		ID:        wire.ID,
		Matchers:  matchers,
		StartsAt:  wire.StartsAt,
		EndsAt:    wire.EndsAt,
		CreatedBy: wire.CreatedBy,
		Comment:   wire.Comment,
		State:     wire.Status.State,
	}
}

func (s Service) SilenceList(ctx pluginbinding.Context, input SilenceListInput) (SilenceListResult, error) {
	target, client, err := s.client(input.TargetInput, ctx.Host)
	if err != nil {
		return SilenceListResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	var wire []apiSilence
	if err := client.get(context.Background(), "/api/v2/silences", nil, &wire); err != nil {
		return SilenceListResult{}, pluginbinding.Errorf("alertmanager", "%s", err)
	}
	state := strings.ToLower(strings.TrimSpace(input.State))
	silences := []Silence{}
	for _, item := range wire {
		if state != "" && item.Status.State != state {
			continue
		}
		silences = append(silences, silenceFromAPI(item))
	}
	return SilenceListResult{URL: target, Silences: silences, Count: len(silences)}, nil
}

func (s Service) SilenceCreate(ctx pluginbinding.Context, input SilenceCreateInput) (SilenceCreateResult, error) {
	target, client, err := s.client(input.TargetInput, ctx.Host)
	if err != nil {
		return SilenceCreateResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if len(input.Matchers) == 0 {
		return SilenceCreateResult{}, pluginbinding.Fail("bad_input", "at least one matcher is required")
	}
	if strings.TrimSpace(input.Comment) == "" {
		return SilenceCreateResult{}, pluginbinding.Fail("bad_input", "comment is required — say why the silence exists")
	}
	now := time.Now().UTC()
	endsAt := strings.TrimSpace(input.EndsAt)
	if endsAt == "" {
		duration := 1 * time.Hour
		if raw := strings.TrimSpace(input.Duration); raw != "" {
			parsed, err := time.ParseDuration(raw)
			if err != nil {
				return SilenceCreateResult{}, pluginbinding.Errorf("bad_input", "invalid duration %q — use Go durations like 30m or 2h", raw)
			}
			duration = parsed
		}
		endsAt = now.Add(duration).Format(time.RFC3339)
	} else if _, err := time.Parse(time.RFC3339, endsAt); err != nil {
		return SilenceCreateResult{}, pluginbinding.Errorf("bad_input", "invalid ends_at %q — RFC3339 required", endsAt)
	}
	matchers := make([]map[string]any, 0, len(input.Matchers))
	for i, matcher := range input.Matchers {
		name := strings.TrimSpace(matcher.Name)
		if name == "" {
			return SilenceCreateResult{}, pluginbinding.Errorf("bad_input", "matchers[%d]: name is required", i)
		}
		isEqual := true
		if matcher.IsEqual != nil {
			isEqual = *matcher.IsEqual
		}
		matchers = append(matchers, map[string]any{
			"name":    name,
			"value":   matcher.Value,
			"isRegex": matcher.IsRegex,
			"isEqual": isEqual,
		})
	}
	body := map[string]any{
		"matchers":  matchers,
		"startsAt":  now.Format(time.RFC3339),
		"endsAt":    endsAt,
		"createdBy": firstNonEmpty(strings.TrimSpace(input.CreatedBy), "fluxplane-plugin"),
		"comment":   strings.TrimSpace(input.Comment),
	}
	var created struct {
		SilenceID string `json:"silenceID"`
	}
	if err := client.request(context.Background(), "POST", "/api/v2/silences", nil, body, &created); err != nil {
		return SilenceCreateResult{}, pluginbinding.Errorf("alertmanager", "%s", err)
	}
	return SilenceCreateResult{URL: target, ID: created.SilenceID, EndsAt: endsAt, Created: created.SilenceID != ""}, nil
}

func (s Service) SilenceDelete(ctx pluginbinding.Context, input SilenceDeleteInput) (SilenceDeleteResult, error) {
	target, client, err := s.client(input.TargetInput, ctx.Host)
	if err != nil {
		return SilenceDeleteResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return SilenceDeleteResult{}, pluginbinding.Fail("bad_input", "id is required")
	}
	if err := client.request(context.Background(), "DELETE", "/api/v2/silence/"+url.PathEscape(id), nil, nil, nil); err != nil {
		return SilenceDeleteResult{}, pluginbinding.Errorf("alertmanager", "%s", err)
	}
	return SilenceDeleteResult{URL: target, ID: id, Deleted: true}, nil
}

func boolParam(value *bool, fallback bool) string {
	resolved := fallback
	if value != nil {
		resolved = *value
	}
	if resolved {
		return "true"
	}
	return "false"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
