package grafana

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"
)

// maxSpansPerTrace bounds tempo.trace.get output; large traces carry
// thousands of spans.
const maxSpansPerTrace = 200

// TraceSummary is one hit from Tempo /api/search.
type TraceSummary struct {
	TraceID         string `json:"trace_id"`
	RootServiceName string `json:"root_service_name,omitempty"`
	RootTraceName   string `json:"root_trace_name,omitempty"`
	StartTime       string `json:"start_time,omitempty"`
	DurationMS      int64  `json:"duration_ms,omitempty"`
}

// parseTempoSearch decodes Tempo /api/search results, tolerating both
// traceID/traceId key casings.
func parseTempoSearch(raw json.RawMessage) ([]TraceSummary, error) {
	var wire struct {
		Traces []struct {
			TraceID           string  `json:"traceID"`
			TraceIDAlt        string  `json:"traceId"`
			RootServiceName   string  `json:"rootServiceName"`
			RootTraceName     string  `json:"rootTraceName"`
			StartTimeUnixNano string  `json:"startTimeUnixNano"`
			DurationMS        float64 `json:"durationMs"`
		} `json:"traces"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, fmt.Errorf("decode tempo search: %w", err)
	}
	out := make([]TraceSummary, 0, len(wire.Traces))
	for _, trace := range wire.Traces {
		out = append(out, TraceSummary{
			TraceID:         firstNonEmpty(trace.TraceID, trace.TraceIDAlt),
			RootServiceName: trace.RootServiceName,
			RootTraceName:   trace.RootTraceName,
			StartTime:       unixNanoToRFC3339(trace.StartTimeUnixNano),
			DurationMS:      int64(trace.DurationMS),
		})
	}
	return out, nil
}

// SpanSummary is one span of a fetched trace, reduced to what an engineer
// reads when following a request: identity, service, timing, and status.
type SpanSummary struct {
	SpanID       string `json:"span_id"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
	Service      string `json:"service,omitempty"`
	Name         string `json:"name"`
	StartTime    string `json:"start_time,omitempty"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
	StatusCode   string `json:"status_code,omitempty" jsonschema:"description=unset\\, ok\\, or error."`
}

type tempoWireSpan struct {
	SpanID            string `json:"spanId"`
	SpanIDAlt         string `json:"spanID"`
	ParentSpanID      string `json:"parentSpanId"`
	ParentSpanIDAlt   string `json:"parentSpanID"`
	Name              string `json:"name"`
	StartTimeUnixNano string `json:"startTimeUnixNano"`
	EndTimeUnixNano   string `json:"endTimeUnixNano"`
	Status            struct {
		Code any `json:"code"`
	} `json:"status"`
}

// parseTempoTrace summarizes an OTLP-shaped Tempo trace (both the "batches"
// key Tempo emits and the standard "resourceSpans"). It returns the spans
// (root first, then by start time, capped at maxSpansPerTrace with a
// truncation flag), the distinct services, and the root span's duration.
func parseTempoTrace(raw json.RawMessage) (spans []SpanSummary, services []string, rootSpan string, durationMS int64, truncated bool, err error) {
	var wire struct {
		Batches       []tempoWireBatch `json:"batches"`
		ResourceSpans []tempoWireBatch `json:"resourceSpans"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, nil, "", 0, false, fmt.Errorf("decode tempo trace: %w", err)
	}
	batches := wire.Batches
	if len(batches) == 0 {
		batches = wire.ResourceSpans
	}
	if len(batches) == 0 {
		return nil, nil, "", 0, false, fmt.Errorf("unrecognized tempo trace shape: no batches or resourceSpans")
	}
	serviceSet := map[string]bool{}
	for _, batch := range batches {
		service := batch.serviceName()
		if service != "" {
			serviceSet[service] = true
		}
		for _, scope := range append(batch.ScopeSpans, batch.InstrumentationLibrarySpans...) {
			for _, span := range scope.Spans {
				start := parseUnixNano(span.StartTimeUnixNano)
				end := parseUnixNano(span.EndTimeUnixNano)
				summary := SpanSummary{
					SpanID:       firstNonEmpty(span.SpanID, span.SpanIDAlt),
					ParentSpanID: firstNonEmpty(span.ParentSpanID, span.ParentSpanIDAlt),
					Service:      service,
					Name:         span.Name,
					StartTime:    unixNanoToRFC3339(span.StartTimeUnixNano),
					StatusCode:   tempoStatusCode(span.Status.Code),
				}
				if end > start && start > 0 {
					summary.DurationMS = (end - start) / int64(time.Millisecond)
				}
				spans = append(spans, summary)
			}
		}
	}
	sort.Slice(spans, func(i, j int) bool {
		// roots (no parent) first, then chronological
		if (spans[i].ParentSpanID == "") != (spans[j].ParentSpanID == "") {
			return spans[i].ParentSpanID == ""
		}
		return spans[i].StartTime < spans[j].StartTime
	})
	if len(spans) > 0 && spans[0].ParentSpanID == "" {
		rootSpan = spans[0].Name
		durationMS = spans[0].DurationMS
	}
	if len(spans) > maxSpansPerTrace {
		spans = spans[:maxSpansPerTrace]
		truncated = true
	}
	services = make([]string, 0, len(serviceSet))
	for service := range serviceSet {
		services = append(services, service)
	}
	sort.Strings(services)
	return spans, services, rootSpan, durationMS, truncated, nil
}

type tempoWireBatch struct {
	Resource struct {
		Attributes []struct {
			Key   string `json:"key"`
			Value struct {
				StringValue string `json:"stringValue"`
			} `json:"value"`
		} `json:"attributes"`
	} `json:"resource"`
	ScopeSpans                  []tempoWireScope `json:"scopeSpans"`
	InstrumentationLibrarySpans []tempoWireScope `json:"instrumentationLibrarySpans"`
}

type tempoWireScope struct {
	Spans []tempoWireSpan `json:"spans"`
}

func (b tempoWireBatch) serviceName() string {
	for _, attribute := range b.Resource.Attributes {
		if attribute.Key == "service.name" {
			return attribute.Value.StringValue
		}
	}
	return ""
}

// tempoStatusCode maps OTLP status codes (numeric or string) to a readable
// form: 0/unset, 1/ok, 2/error.
func tempoStatusCode(code any) string {
	switch typed := code.(type) {
	case nil:
		return ""
	case float64:
		switch int(typed) {
		case 0:
			return "unset"
		case 1:
			return "ok"
		case 2:
			return "error"
		}
	case string:
		switch typed {
		case "STATUS_CODE_UNSET", "":
			return "unset"
		case "STATUS_CODE_OK":
			return "ok"
		case "STATUS_CODE_ERROR":
			return "error"
		default:
			return typed
		}
	}
	return fmt.Sprint(code)
}

func parseUnixNano(value string) int64 {
	nanos, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return nanos
}

func unixNanoToRFC3339(value string) string {
	nanos := parseUnixNano(value)
	if nanos <= 0 {
		return ""
	}
	return time.Unix(0, nanos).UTC().Format(time.RFC3339Nano)
}
