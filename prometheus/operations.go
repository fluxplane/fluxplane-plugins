package prometheus

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type Service struct {
}

func NewService() Service {
	return Service{}
}

type PrometheusTargetInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Prometheus endpoint ref resolved by the host."`
}

type TestInput struct {
	PrometheusTargetInput
}

type TestResult struct {
	URL       string `json:"url"`
	Ready     bool   `json:"ready"`
	Error     string `json:"error,omitempty"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
}

type QueryInput struct {
	PrometheusTargetInput
	Query string `json:"query,omitempty" jsonschema:"required,description=PromQL query"`
	Time  string `json:"time,omitempty" jsonschema:"description=RFC3339 or unix timestamp"`
}

type QueryResult struct {
	URL        string   `json:"url"`
	Query      string   `json:"query"`
	ResultType string   `json:"result_type"`
	Samples    []Sample `json:"samples" jsonschema:"description=Vector/scalar/string results: one value per metric."`
	Series     []Series `json:"series" jsonschema:"description=Matrix results: points over time per metric."`
	Count      int      `json:"count"`
	Truncated  bool     `json:"truncated,omitempty" jsonschema:"description=True when series or points were dropped to stay within output caps."`
}

type QueryRangeInput struct {
	PrometheusTargetInput
	Query string `json:"query,omitempty" jsonschema:"required,description=PromQL query"`
	Since string `json:"since,omitempty" jsonschema:"description=Start time as RFC3339\\, unix timestamp\\, or duration ago. Defaults to 1h."`
	Until string `json:"until,omitempty" jsonschema:"description=End time as RFC3339\\, unix timestamp\\, or duration ago. Defaults to now."`
	Step  string `json:"step,omitempty" jsonschema:"description=Range step duration. Choose step so (until-since)/step stays under 500 points per series; excess points are truncated keeping the newest."`
}

type QueryRangeResult = QueryResult

type LabelsInput struct {
	PrometheusTargetInput
	Label string   `json:"label,omitempty" jsonschema:"description=Optional label name. When set\\, returns values for that label."`
	Match []string `json:"match,omitempty" jsonschema:"description=Optional PromQL match selectors."`
}

type LabelsResult struct {
	URL    string   `json:"url"`
	Label  string   `json:"label,omitempty"`
	Values []string `json:"values"`
}

type TargetsInput struct {
	PrometheusTargetInput
	State string `json:"state,omitempty" jsonschema:"description=active (default)\\, dropped\\, or any. Defaults to active: the dropped list carries every discovered-then-relabeled-away target and can exceed the response cap on large clusters.,enum=active,enum=dropped,enum=any"`
}

type TargetsResult struct {
	URL          string   `json:"url"`
	State        string   `json:"state,omitempty"`
	Targets      []Target `json:"targets"`
	ActiveCount  int      `json:"active_count"`
	DroppedCount int      `json:"dropped_count"`
}

type AlertsResult struct {
	URL    string  `json:"url"`
	Alerts []Alert `json:"alerts"`
	Count  int     `json:"count"`
}

type RulesInput struct {
	PrometheusTargetInput
	Type string `json:"type,omitempty" jsonschema:"description=Filter by rule kind: alert or record. Empty returns both."`
}

type RulesResult struct {
	URL        string      `json:"url"`
	Groups     []RuleGroup `json:"groups"`
	GroupCount int         `json:"group_count"`
	RuleCount  int         `json:"rule_count"`
}

type SeriesInput struct {
	PrometheusTargetInput
	Match []string `json:"match,omitempty" jsonschema:"required,description=PromQL series selectors\\, e.g. up{job=\"api\"}."`
	Since string   `json:"since,omitempty" jsonschema:"description=Start time as RFC3339\\, unix timestamp\\, or duration ago"`
	Until string   `json:"until,omitempty" jsonschema:"description=End time as RFC3339\\, unix timestamp\\, or duration ago"`
	Limit int      `json:"limit,omitempty" jsonschema:"description=Maximum series returned. Default 100\\, max 1000."`
}

type SeriesResult struct {
	URL       string              `json:"url"`
	Series    []map[string]string `json:"series" jsonschema:"description=Matching series label sets."`
	Count     int                 `json:"count"`
	Truncated bool                `json:"truncated,omitempty"`
}

type QueryRecord struct {
	pluginbinding.DatasourceRecord
	Title       string         `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	Query       string         `json:"query,omitempty" datasource:"completion,view=compact|lookup|table"`
	ResultType  string         `json:"result_type,omitempty" datasource:"completion,view=compact|lookup|table"`
	Result      map[string]any `json:"result,omitempty" datasource:"view=lookup|table"`
	EndpointURL string         `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type LabelRecord struct {
	pluginbinding.DatasourceRecord
	Name        string `json:"name" datasource:"id,completion,view=compact|lookup|table"`
	Label       string `json:"label,omitempty" datasource:"completion,view=compact|lookup|table"`
	EndpointURL string `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type TargetRecord struct {
	pluginbinding.DatasourceRecord
	Title       string         `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	State       string         `json:"state,omitempty" datasource:"completion,view=compact|lookup|table"`
	Job         string         `json:"job,omitempty" datasource:"completion,view=compact|lookup|table"`
	Endpoint    string         `json:"endpoint,omitempty" datasource:"completion,view=compact|lookup|table"`
	Target      map[string]any `json:"target,omitempty" datasource:"view=lookup|table"`
	EndpointURL string         `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type AlertRecord struct {
	pluginbinding.DatasourceRecord
	Title       string            `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	Name        string            `json:"name,omitempty" datasource:"completion,view=compact|lookup|table"`
	State       string            `json:"state,omitempty" datasource:"completion,view=compact|lookup|table"`
	Severity    string            `json:"severity,omitempty" datasource:"completion,view=compact|lookup|table"`
	Labels      map[string]string `json:"labels,omitempty" datasource:"view=lookup|table"`
	Annotations map[string]string `json:"annotations,omitempty" datasource:"view=lookup|table"`
	EndpointURL string            `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type QueryDatasourceResult = pluginbinding.DatasourceSearchResult[QueryRecord]
type LabelDatasourceResult = pluginbinding.DatasourceSearchResult[LabelRecord]
type TargetDatasourceResult = pluginbinding.DatasourceSearchResult[TargetRecord]
type AlertDatasourceResult = pluginbinding.DatasourceSearchResult[AlertRecord]

func (s Service) Test(ctx pluginbinding.Context, input TestInput) (TestResult, error) {
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return TestResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start := time.Now()
	err = client.ready(context.Background())
	out := TestResult{URL: target, Ready: err == nil, LatencyMS: time.Since(start).Milliseconds()}
	if err != nil {
		out.Error = err.Error()
	}
	return out, nil
}

func (s Service) Query(ctx pluginbinding.Context, input QueryInput) (QueryResult, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return QueryResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	values := url.Values{"query": {query}}
	if strings.TrimSpace(input.Time) != "" {
		t, err := parseTimeValue(input.Time, time.Now())
		if err != nil {
			return QueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
		}
		values.Set("time", strconv.FormatInt(t.Unix(), 10))
	}
	return s.query(context.Background(), target, client, query, "/api/v1/query", values)
}

func (s Service) QueryRange(ctx pluginbinding.Context, input QueryRangeInput) (QueryRangeResult, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return QueryRangeResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return QueryRangeResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	now := time.Now()
	end, err := parseTimeValue(firstNonEmpty(input.Until, "0s"), now)
	if err != nil {
		return QueryRangeResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start, err := parseTimeValue(firstNonEmpty(input.Since, "1h"), now)
	if err != nil {
		return QueryRangeResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	step := strings.TrimSpace(input.Step)
	if step == "" {
		step = "1m"
	}
	values := url.Values{"query": {query}}
	values.Set("start", strconv.FormatInt(start.Unix(), 10))
	values.Set("end", strconv.FormatInt(end.Unix(), 10))
	values.Set("step", step)
	return s.query(context.Background(), target, client, query, "/api/v1/query_range", values)
}

func (s Service) Labels(ctx pluginbinding.Context, input LabelsInput) (LabelsResult, error) {
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return LabelsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	values := url.Values{}
	for _, match := range input.Match {
		if strings.TrimSpace(match) != "" {
			values.Add("match[]", strings.TrimSpace(match))
		}
	}
	path := "/api/v1/labels"
	label := strings.TrimSpace(input.Label)
	if label != "" {
		path = "/api/v1/label/" + url.PathEscape(label) + "/values"
	}
	data, err := client.get(context.Background(), path, values)
	if err != nil {
		return LabelsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	var valuesOut []string
	if err := json.Unmarshal(data, &valuesOut); err != nil {
		return LabelsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	return LabelsResult{URL: target, Label: label, Values: valuesOut}, nil
}

func (s Service) Targets(ctx pluginbinding.Context, input TargetsInput) (TargetsResult, error) {
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return TargetsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	values := url.Values{}
	state := strings.TrimSpace(input.State)
	if state == "" {
		state = "active" // dropped targets can exceed the response cap; opt in explicitly
	}
	if state != "any" {
		values.Set("state", state)
	}
	data, err := client.get(context.Background(), "/api/v1/targets", values)
	if err != nil {
		return TargetsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	active, dropped, err := parseTargets(data)
	if err != nil {
		return TargetsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	return TargetsResult{
		URL:          target,
		State:        input.State,
		Targets:      append(active, dropped...),
		ActiveCount:  len(active),
		DroppedCount: len(dropped),
	}, nil
}

func (s Service) Alerts(ctx pluginbinding.Context, input TestInput) (AlertsResult, error) {
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return AlertsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	data, err := client.get(context.Background(), "/api/v1/alerts", nil)
	if err != nil {
		return AlertsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	alerts, err := parseAlerts(data)
	if err != nil {
		return AlertsResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	return AlertsResult{URL: target, Alerts: alerts, Count: len(alerts)}, nil
}

// Rules lists alerting and recording rules from /api/v1/rules.
func (s Service) Rules(ctx pluginbinding.Context, input RulesInput) (RulesResult, error) {
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return RulesResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	values := url.Values{}
	switch kind := strings.ToLower(strings.TrimSpace(input.Type)); kind {
	case "":
	case "alert", "record":
		values.Set("type", kind)
	default:
		return RulesResult{}, pluginbinding.Fail("bad_input", "type must be alert or record")
	}
	data, err := client.get(context.Background(), "/api/v1/rules", values)
	if err != nil {
		return RulesResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	groups, err := parseRuleGroups(data)
	if err != nil {
		return RulesResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	ruleCount := 0
	for _, group := range groups {
		ruleCount += len(group.Rules)
	}
	return RulesResult{URL: target, Groups: groups, GroupCount: len(groups), RuleCount: ruleCount}, nil
}

// SeriesMeta lists series label sets matching the given selectors via
// /api/v1/series.
func (s Service) SeriesMeta(ctx pluginbinding.Context, input SeriesInput) (SeriesResult, error) {
	var match []string
	for _, selector := range input.Match {
		if strings.TrimSpace(selector) != "" {
			match = append(match, strings.TrimSpace(selector))
		}
	}
	if len(match) == 0 {
		return SeriesResult{}, pluginbinding.Fail("bad_input", "at least one match selector is required")
	}
	target, client, err := s.client(ctx, input.PrometheusTargetInput)
	if err != nil {
		return SeriesResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	now := time.Now()
	values := url.Values{"match[]": match}
	if strings.TrimSpace(input.Since) != "" {
		start, err := parseTimeValue(input.Since, now)
		if err != nil {
			return SeriesResult{}, pluginbinding.Errorf("bad_input", "%s", err)
		}
		values.Set("start", strconv.FormatInt(start.Unix(), 10))
	}
	if strings.TrimSpace(input.Until) != "" {
		end, err := parseTimeValue(input.Until, now)
		if err != nil {
			return SeriesResult{}, pluginbinding.Errorf("bad_input", "%s", err)
		}
		values.Set("end", strconv.FormatInt(end.Unix(), 10))
	}
	// Newer Prometheus honors limit server-side; the client-side cap below
	// covers older versions.
	values.Set("limit", strconv.Itoa(limit))
	data, err := client.get(context.Background(), "/api/v1/series", values)
	if err != nil {
		return SeriesResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	var series []map[string]string
	if err := json.Unmarshal(data, &series); err != nil {
		return SeriesResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	truncated := false
	if len(series) > limit {
		series = series[:limit]
		truncated = true
	}
	return SeriesResult{URL: target, Series: series, Count: len(series), Truncated: truncated}, nil
}

func (s Service) QueryDatasource(ctx pluginbinding.Context, input QueryInput) (QueryDatasourceResult, error) {
	out, err := s.Query(ctx, input)
	if err != nil {
		return QueryDatasourceResult{}, err
	}
	results := make([]map[string]any, 0, out.Count)
	metrics := make([]map[string]string, 0, out.Count)
	for _, sample := range out.Samples {
		results = append(results, map[string]any{"metric": sample.Metric, "value": sample.Value, "timestamp": sample.Timestamp})
		metrics = append(metrics, sample.Metric)
	}
	for _, series := range out.Series {
		results = append(results, map[string]any{"metric": series.Metric, "points": series.Points, "point_count": series.PointCount})
		metrics = append(metrics, series.Metric)
	}
	records := make([]QueryRecord, 0, len(results))
	for i, result := range results {
		id := stableID(out.URL, out.Query, strconv.Itoa(i), result)
		title := firstNonEmpty(metricTitle(metrics[i]), fmt.Sprintf("result %d", i+1))
		record := QueryRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(ctx.DatasourceSource(), EntityQueryResult, id, pluginbinding.RecordTitle(title), pluginbinding.RecordMetadata(map[string]any{"query": out.Query, "result_type": out.ResultType, "result": result, "endpoint_url": out.URL})),
			Title:            title,
			Query:            out.Query,
			ResultType:       out.ResultType,
			Result:           result,
			EndpointURL:      out.URL,
		}
		if out.URL != "" {
			record.Links = map[string]string{"endpoint": out.URL}
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", input.Query, records), nil
}

func (s Service) LabelsDatasource(ctx pluginbinding.Context, input LabelsInput) (LabelDatasourceResult, error) {
	out, err := s.Labels(ctx, input)
	if err != nil {
		return LabelDatasourceResult{}, err
	}
	records := make([]LabelRecord, 0, len(out.Values))
	for _, value := range out.Values {
		id := value
		if out.Label != "" {
			id = out.Label + "=" + value
		}
		record := LabelRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(ctx.DatasourceSource(), EntityLabel, id, pluginbinding.RecordTitle(id), pluginbinding.RecordMetadata(map[string]any{"name": value, "label": out.Label, "endpoint_url": out.URL})),
			Name:             value,
			Label:            out.Label,
			EndpointURL:      out.URL,
		}
		if out.URL != "" {
			record.Links = map[string]string{"endpoint": out.URL}
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", firstNonEmpty(input.Label, strings.Join(input.Match, " ")), records), nil
}

func (s Service) TargetsDatasource(ctx pluginbinding.Context, input TargetsInput) (TargetDatasourceResult, error) {
	out, err := s.Targets(ctx, input)
	if err != nil {
		return TargetDatasourceResult{}, err
	}
	records := make([]TargetRecord, 0, len(out.Targets))
	for i, target := range out.Targets {
		job := target.Job
		endpoint := firstNonEmpty(target.Instance, target.ScrapePool)
		state := firstNonEmpty(target.Health, out.State)
		title := firstNonEmpty(job, endpoint, fmt.Sprintf("target %d", i+1))
		id := stableID(out.URL, title, strconv.Itoa(i), target)
		detail := map[string]any{
			"job": job, "instance": target.Instance, "health": target.Health,
			"scrape_pool": target.ScrapePool, "scrape_url": target.ScrapeURL,
			"last_scrape": target.LastScrape, "last_error": target.LastError,
			"labels": target.Labels, "dropped": target.Dropped,
		}
		record := TargetRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(ctx.DatasourceSource(), EntityTarget, id, pluginbinding.RecordTitle(title), pluginbinding.RecordMetadata(map[string]any{"state": state, "job": job, "endpoint": endpoint, "target": detail, "endpoint_url": out.URL})),
			Title:            title,
			State:            state,
			Job:              job,
			Endpoint:         endpoint,
			Target:           detail,
			EndpointURL:      out.URL,
		}
		if out.URL != "" {
			record.Links = map[string]string{"endpoint": out.URL}
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", input.State, records), nil
}

func (s Service) AlertsDatasource(ctx pluginbinding.Context, input TestInput) (AlertDatasourceResult, error) {
	out, err := s.Alerts(ctx, input)
	if err != nil {
		return AlertDatasourceResult{}, err
	}
	records := make([]AlertRecord, 0, len(out.Alerts))
	for i, alert := range out.Alerts {
		title := firstNonEmpty(alert.Name, alert.Annotations["summary"], fmt.Sprintf("alert %d", i+1))
		id := stableID(out.URL, title, strconv.Itoa(i), alert)
		record := AlertRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(ctx.DatasourceSource(), EntityAlert, id, pluginbinding.RecordTitle(title), pluginbinding.RecordMetadata(map[string]any{"name": alert.Name, "state": alert.State, "severity": alert.Severity, "labels": alert.Labels, "annotations": alert.Annotations, "endpoint_url": out.URL})),
			Title:            title,
			Name:             alert.Name,
			State:            alert.State,
			Severity:         alert.Severity,
			Labels:           alert.Labels,
			Annotations:      alert.Annotations,
			EndpointURL:      out.URL,
		}
		if out.URL != "" {
			record.Links = map[string]string{"endpoint": out.URL}
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", "", records), nil
}

func (s Service) query(ctx context.Context, target string, client Client, query, path string, values url.Values) (QueryResult, error) {
	data, err := client.get(ctx, path, values)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	var wrapped struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return QueryResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	samples, series, truncated, err := parsePromQLData(wrapped.ResultType, wrapped.Result)
	if err != nil {
		return QueryResult{}, pluginbinding.Errorf("prometheus", "%s", err)
	}
	out := QueryResult{URL: target, Query: query, ResultType: wrapped.ResultType, Samples: samples, Series: series, Truncated: truncated}
	if out.Samples == nil {
		out.Samples = []Sample{}
	}
	if out.Series == nil {
		out.Series = []Series{}
	}
	out.Count = len(samples) + len(series)
	return out, nil
}

func (s Service) client(ctx pluginbinding.Context, input PrometheusTargetInput) (string, Client, error) {
	_ = s
	endpointRef := strings.TrimSpace(input.EndpointRef)
	if endpointRef == "" {
		return "", Client{}, fmt.Errorf("endpoint_ref is required")
	}
	return endpointRef, Client{EndpointRef: endpointRef, Host: ctx.Host}, nil
}

func parseTimeValue(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("time value is empty")
	}
	if d, err := time.ParseDuration(value); err == nil {
		return now.Add(-d), nil
	}
	if ts, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(ts, 0), nil
	}
	return time.Parse(time.RFC3339, value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func metricTitle(metric map[string]string) string {
	if len(metric) == 0 {
		return ""
	}
	parts := make([]string, 0, len(metric))
	for key, value := range metric {
		if value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func stableID(parts ...any) string {
	data, _ := json.Marshal(parts)
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
