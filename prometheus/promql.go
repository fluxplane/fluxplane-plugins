package prometheus

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Result-size caps. Range queries can return thousands of points per series;
// the caps keep operation output agent-readable and signal truncation
// explicitly instead of dumping everything.
const (
	maxSeriesPerResult = 200
	maxPointsPerSeries = 500
)

// SamplePoint is one timestamped value. Value stays a string because
// Prometheus legitimately returns "NaN", "+Inf", and "-Inf", which cannot be
// encoded as JSON numbers.
type SamplePoint struct {
	Timestamp string `json:"timestamp" jsonschema:"description=Sample time, RFC3339."`
	Value     string `json:"value" jsonschema:"description=Sample value as Prometheus returns it; may be NaN or +/-Inf."`
}

// Sample is one instant-query result: a metric label set with a single value.
// Scalar and string results carry an empty metric.
type Sample struct {
	Metric map[string]string `json:"metric,omitempty" jsonschema:"description=Metric label set."`
	SamplePoint
}

// Series is one range-query result: a metric label set with its points over
// the queried window. PointCount is the pre-truncation count; when Truncated
// is set, Points keeps the newest maxPointsPerSeries entries.
type Series struct {
	Metric     map[string]string `json:"metric,omitempty" jsonschema:"description=Metric label set."`
	Points     []SamplePoint     `json:"points"`
	PointCount int               `json:"point_count" jsonschema:"description=Number of points before truncation."`
	Truncated  bool              `json:"truncated,omitempty" jsonschema:"description=True when points were dropped; increase step to avoid."`
}

// parsePromQLData decodes the data.result payload of /api/v1/query and
// /api/v1/query_range for all four result types. Vector/scalar/string land in
// samples; matrix lands in series with per-series and total-series caps.
func parsePromQLData(resultType string, raw json.RawMessage) (samples []Sample, series []Series, truncated bool, err error) {
	switch strings.ToLower(strings.TrimSpace(resultType)) {
	case "vector":
		var wire []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, nil, false, fmt.Errorf("decode vector result: %w", err)
		}
		for _, item := range wire {
			point, ok := samplePointFromPair(item.Value)
			if !ok {
				continue
			}
			samples = append(samples, Sample{Metric: item.Metric, SamplePoint: point})
		}
		if len(samples) > maxSeriesPerResult {
			samples = samples[:maxSeriesPerResult]
			truncated = true
		}
		return samples, nil, truncated, nil
	case "matrix":
		var wire []struct {
			Metric map[string]string `json:"metric"`
			Values [][]any           `json:"values"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			return nil, nil, false, fmt.Errorf("decode matrix result: %w", err)
		}
		for _, item := range wire {
			one := Series{Metric: item.Metric, PointCount: len(item.Values)}
			values := item.Values
			if len(values) > maxPointsPerSeries {
				values = values[len(values)-maxPointsPerSeries:] // keep the newest
				one.Truncated = true
				truncated = true
			}
			for _, pair := range values {
				if point, ok := samplePointFromPair(pair); ok {
					one.Points = append(one.Points, point)
				}
			}
			series = append(series, one)
		}
		if len(series) > maxSeriesPerResult {
			series = series[:maxSeriesPerResult]
			truncated = true
		}
		return nil, series, truncated, nil
	case "scalar", "string":
		var pair []any
		if err := json.Unmarshal(raw, &pair); err != nil {
			return nil, nil, false, fmt.Errorf("decode %s result: %w", resultType, err)
		}
		if point, ok := samplePointFromPair(pair); ok {
			samples = append(samples, Sample{SamplePoint: point})
		}
		return samples, nil, false, nil
	case "":
		return nil, nil, false, nil
	default:
		return nil, nil, false, fmt.Errorf("unsupported result type %q", resultType)
	}
}

// samplePointFromPair decodes Prometheus's [unixSeconds, "value"] pair.
func samplePointFromPair(pair []any) (SamplePoint, bool) {
	if len(pair) != 2 {
		return SamplePoint{}, false
	}
	return SamplePoint{Timestamp: timestampFromAny(pair[0]), Value: stringFromAny(pair[1])}, true
}

func timestampFromAny(value any) string {
	switch typed := value.(type) {
	case float64:
		sec := int64(typed)
		nsec := int64((typed - float64(sec)) * float64(time.Second))
		return time.Unix(sec, nsec).UTC().Format(time.RFC3339)
	case string:
		if ts, err := strconv.ParseFloat(typed, 64); err == nil {
			return timestampFromAny(ts)
		}
		return typed
	case json.Number:
		if ts, err := typed.Float64(); err == nil {
			return timestampFromAny(ts)
		}
		return typed.String()
	default:
		return stringFromAny(value)
	}
}

// Target is one scrape target from /api/v1/targets, flattened to what an
// engineer reads first: identity (job/instance), health, and the last error.
type Target struct {
	Job        string            `json:"job,omitempty"`
	Instance   string            `json:"instance,omitempty"`
	Health     string            `json:"health,omitempty" jsonschema:"description=up, down, or unknown."`
	ScrapePool string            `json:"scrape_pool,omitempty"`
	ScrapeURL  string            `json:"scrape_url,omitempty"`
	LastScrape string            `json:"last_scrape,omitempty"`
	LastError  string            `json:"last_error,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Dropped    bool              `json:"dropped,omitempty" jsonschema:"description=True for targets dropped by relabeling."`
}

type wireTarget struct {
	Labels           map[string]string `json:"labels"`
	DiscoveredLabels map[string]string `json:"discoveredLabels"`
	ScrapePool       string            `json:"scrapePool"`
	ScrapeURL        string            `json:"scrapeUrl"`
	Health           string            `json:"health"`
	LastScrape       string            `json:"lastScrape"`
	LastError        string            `json:"lastError"`
}

// parseTargets decodes the data payload of /api/v1/targets into typed records.
func parseTargets(raw json.RawMessage) (active, dropped []Target, err error) {
	var wire struct {
		ActiveTargets  []wireTarget `json:"activeTargets"`
		DroppedTargets []wireTarget `json:"droppedTargets"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, nil, fmt.Errorf("decode targets: %w", err)
	}
	for _, item := range wire.ActiveTargets {
		active = append(active, targetFromWire(item, false))
	}
	for _, item := range wire.DroppedTargets {
		dropped = append(dropped, targetFromWire(item, true))
	}
	return active, dropped, nil
}

func targetFromWire(wire wireTarget, dropped bool) Target {
	labels := wire.Labels
	if len(labels) == 0 {
		labels = wire.DiscoveredLabels
	}
	return Target{
		Job:        firstNonEmpty(labels["job"], wire.DiscoveredLabels["job"]),
		Instance:   firstNonEmpty(labels["instance"], wire.DiscoveredLabels["__address__"]),
		Health:     wire.Health,
		ScrapePool: wire.ScrapePool,
		ScrapeURL:  wire.ScrapeURL,
		LastScrape: wire.LastScrape,
		LastError:  wire.LastError,
		Labels:     labels,
		Dropped:    dropped,
	}
}

// Alert is one alert from /api/v1/alerts.
type Alert struct {
	Name        string            `json:"name,omitempty"`
	State       string            `json:"state,omitempty" jsonschema:"description=firing, pending, or inactive."`
	Severity    string            `json:"severity,omitempty"`
	ActiveAt    string            `json:"active_at,omitempty"`
	Value       string            `json:"value,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type wireAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"`
	ActiveAt    string            `json:"activeAt"`
	Value       string            `json:"value"`
}

// parseAlerts decodes the data payload of /api/v1/alerts.
func parseAlerts(raw json.RawMessage) ([]Alert, error) {
	var wire struct {
		Alerts []wireAlert `json:"alerts"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("decode alerts: %w", err)
	}
	out := make([]Alert, 0, len(wire.Alerts))
	for _, item := range wire.Alerts {
		out = append(out, alertFromWire(item))
	}
	return out, nil
}

func alertFromWire(wire wireAlert) Alert {
	return Alert{
		Name:        wire.Labels["alertname"],
		State:       wire.State,
		Severity:    wire.Labels["severity"],
		ActiveAt:    wire.ActiveAt,
		Value:       wire.Value,
		Labels:      wire.Labels,
		Annotations: wire.Annotations,
	}
}

// RuleGroup is one group from /api/v1/rules.
type RuleGroup struct {
	Name     string `json:"name"`
	File     string `json:"file,omitempty"`
	Interval string `json:"interval,omitempty"`
	Rules    []Rule `json:"rules"`
}

// Rule is one alerting or recording rule.
type Rule struct {
	Name        string            `json:"name"`
	Type        string            `json:"type" jsonschema:"description=alerting or recording."`
	Query       string            `json:"query"`
	State       string            `json:"state,omitempty" jsonschema:"description=firing, pending, or inactive (alerting rules only)."`
	For         string            `json:"for,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Health      string            `json:"health,omitempty"`
	LastError   string            `json:"last_error,omitempty"`
	ActiveCount int               `json:"active_count,omitempty" jsonschema:"description=Number of currently active alerts on an alerting rule."`
}

// parseRuleGroups decodes the data payload of /api/v1/rules.
func parseRuleGroups(raw json.RawMessage) ([]RuleGroup, error) {
	var wire struct {
		Groups []struct {
			Name     string  `json:"name"`
			File     string  `json:"file"`
			Interval float64 `json:"interval"`
			Rules    []struct {
				Name        string            `json:"name"`
				Type        string            `json:"type"`
				Query       string            `json:"query"`
				State       string            `json:"state"`
				Duration    float64           `json:"duration"`
				Labels      map[string]string `json:"labels"`
				Annotations map[string]string `json:"annotations"`
				Health      string            `json:"health"`
				LastError   string            `json:"lastError"`
				Alerts      []json.RawMessage `json:"alerts"`
			} `json:"rules"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("decode rules: %w", err)
	}
	groups := make([]RuleGroup, 0, len(wire.Groups))
	for _, group := range wire.Groups {
		out := RuleGroup{Name: group.Name, File: group.File, Interval: formatSeconds(group.Interval)}
		for _, rule := range group.Rules {
			out.Rules = append(out.Rules, Rule{
				Name:        rule.Name,
				Type:        rule.Type,
				Query:       rule.Query,
				State:       rule.State,
				For:         formatSeconds(rule.Duration),
				Labels:      rule.Labels,
				Annotations: rule.Annotations,
				Health:      rule.Health,
				LastError:   rule.LastError,
				ActiveCount: len(rule.Alerts),
			})
		}
		groups = append(groups, out)
	}
	return groups, nil
}

// formatSeconds renders the API's seconds-as-float durations ("300" -> "5m0s").
func formatSeconds(seconds float64) string {
	if seconds == 0 {
		return ""
	}
	return (time.Duration(seconds * float64(time.Second))).String()
}
