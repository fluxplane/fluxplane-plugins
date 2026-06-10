package homer

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

const (
	defaultSearchLimit = 200
	maxSearchLimit     = 1000
	defaultCallLimit   = 50
	maxCallLimit       = 200
)

// Service implements the homer operations. The function fields are test
// hooks; when nil, the live client (host HTTP + secret store) is used.
type Service struct {
	NewClient func(ctx pluginbinding.Context, endpointRef string) (*Client, error)
}

func NewService() Service {
	return Service{}
}

func (s Service) client(ctx pluginbinding.Context, endpointRef string) (*Client, error) {
	endpointRef = strings.TrimSpace(endpointRef)
	if endpointRef == "" {
		return nil, fmt.Errorf("endpoint_ref is required")
	}
	if s.NewClient != nil {
		return s.NewClient(ctx, endpointRef)
	}
	return &Client{EndpointRef: endpointRef, Host: ctx.Host}, nil
}

type HomerTargetInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"required,description=Registered Homer endpoint ref resolved by the host."`
}

type TimeRangeInput struct {
	Since string `json:"since,omitempty" jsonschema:"description=Start time as RFC3339, unix seconds, or duration ago (e.g. 1h). Defaults to 1h."`
	Until string `json:"until,omitempty" jsonschema:"description=End time as RFC3339, unix seconds, or duration ago. Defaults to now."`
}

// window resolves the search window with the given default lookback.
func (t TimeRangeInput) window(defaultSince string) (time.Time, time.Time, error) {
	now := time.Now()
	from, err := parseTimeValue(firstNonEmpty(t.Since, defaultSince), now)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid since: %s", err)
	}
	to := now
	if strings.TrimSpace(t.Until) != "" {
		to, err = parseTimeValue(t.Until, now)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid until: %s", err)
		}
	}
	if !from.Before(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("since must be before until")
	}
	return from, to, nil
}

// parseTimeValue accepts a duration-ago ("30m"), unix seconds, or RFC3339.
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

// ---- homer.test ----

type TestInput struct {
	HomerTargetInput
}

type TestResult struct {
	URL           string `json:"url"`
	Reachable     bool   `json:"reachable"`
	Authenticated bool   `json:"authenticated"`
	Error         string `json:"error,omitempty"`
	LatencyMS     int64  `json:"latency_ms,omitempty"`
}

func (s Service) Test(ctx pluginbinding.Context, input TestInput) (TestResult, error) {
	client, err := s.client(ctx, input.EndpointRef)
	if err != nil {
		return TestResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start := time.Now()
	out := TestResult{URL: input.EndpointRef}
	if err := client.Check(); err != nil {
		// agent/check is unauthenticated on stock Homer but some deployments
		// guard it; auth success below still proves reachability.
		out.Error = err.Error()
	} else {
		out.Reachable = true
	}
	if err := client.login(); err != nil {
		out.Error = firstNonEmpty(out.Error, err.Error())
	} else {
		out.Reachable = true
		out.Authenticated = true
		out.Error = ""
	}
	out.LatencyMS = time.Since(start).Milliseconds()
	if !out.Authenticated {
		return out, pluginbinding.Errorf("homer", "homer test failed: %s", out.Error)
	}
	return out, nil
}

// ---- homer.search ----

type SearchInput struct {
	HomerTargetInput
	TimeRangeInput
	Number   string `json:"number,omitempty" jsonschema:"description=Phone number matched against caller OR callee, with and without + prefix."`
	FromUser string `json:"from_user,omitempty" jsonschema:"description=Caller filter (use % as wildcard)."`
	ToUser   string `json:"to_user,omitempty" jsonschema:"description=Callee filter (use % as wildcard)."`
	CallID   string `json:"call_id,omitempty" jsonschema:"description=Exact SIP Call-ID."`
	UA       string `json:"ua,omitempty" jsonschema:"description=User-Agent filter (use % as wildcard)."`
	Method   string `json:"method,omitempty" jsonschema:"description=SIP method or response code, e.g. INVITE or 486."`
	Query    string `json:"query,omitempty" jsonschema:"description=Query DSL: field = 'value' with AND/OR and % wildcards. Fields: call_id, cseq, from_user, method, ruri_user, sid, status, to_user, ua, user_agent."`
	Limit    int    `json:"limit,omitempty" jsonschema:"description=Maximum messages. Default 200, max 1000."`
}

// MessageRecord is one SIP message, shaped for reading: RFC3339 time,
// alias-resolved endpoints, compact user agent.
type MessageRecord struct {
	Time      string `json:"time"`
	Src       string `json:"src"`
	Dst       string `json:"dst"`
	SrcAlias  string `json:"src_alias,omitempty"`
	DstAlias  string `json:"dst_alias,omitempty"`
	Method    string `json:"method"`
	FromUser  string `json:"from_user,omitempty"`
	ToUser    string `json:"to_user,omitempty"`
	CallID    string `json:"call_id"`
	CSeq      string `json:"cseq,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

type SearchResultOutput struct {
	URL       string          `json:"url"`
	Messages  []MessageRecord `json:"messages"`
	Count     int             `json:"count"`
	Truncated bool            `json:"truncated,omitempty" jsonschema:"description=True when the page is full; narrow the window or raise limit."`
}

func (s Service) Search(ctx pluginbinding.Context, input SearchInput) (SearchResultOutput, error) {
	client, err := s.client(ctx, input.EndpointRef)
	if err != nil {
		return SearchResultOutput{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	from, to, err := input.window("1h")
	if err != nil {
		return SearchResultOutput{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	smartInput, err := buildSearchFilters(input)
	if err != nil {
		return SearchResultOutput{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	limit := clampLimit(input.Limit, defaultSearchLimit, maxSearchLimit)
	result, err := client.SearchCalls(SearchParams{From: from, To: to, SmartInput: smartInput, CallID: strings.TrimSpace(input.CallID), Limit: limit})
	if err != nil {
		return SearchResultOutput{}, pluginbinding.Errorf("homer", "%s", err)
	}
	// Sort the raw records by epoch: RFC3339Nano strings drop trailing zeros,
	// which breaks lexicographic time ordering.
	sort.Slice(result.Data, func(i, j int) bool { return result.Data[i].Date < result.Data[j].Date })
	records := make([]MessageRecord, 0, len(result.Data))
	for _, record := range result.Data {
		records = append(records, messageRecord(record))
	}
	return SearchResultOutput{
		URL:       input.EndpointRef,
		Messages:  records,
		Count:     len(records),
		Truncated: len(records) >= limit,
	}, nil
}

func messageRecord(record CallRecord) MessageRecord {
	method := firstNonEmpty(record.Method, record.MethodText)
	return MessageRecord{
		Time:      time.UnixMilli(record.Date).UTC().Format(time.RFC3339Nano),
		Src:       record.SourceIP + ":" + strconv.Itoa(int(record.SourcePort)),
		Dst:       record.DestIP + ":" + strconv.Itoa(int(record.DestPort)),
		SrcAlias:  record.AliasSrc,
		DstAlias:  record.AliasDst,
		Method:    method,
		FromUser:  record.FromUser,
		ToUser:    firstNonEmpty(record.ToUser, record.RuriUser),
		CallID:    record.CallID,
		CSeq:      record.CSeq,
		UserAgent: FormatUserAgent(record.UserAgent),
	}
}

// buildSearchFilters combines the flag-style filters (cartesian product, no
// parentheses) with the optional query DSL.
func buildSearchFilters(input SearchInput) (string, error) {
	var criteria [][]string
	if number := strings.TrimSpace(input.Number); number != "" {
		alternatives := append(NumberAlternatives("data_header.from_user", number), NumberAlternatives("data_header.to_user", number)...)
		criteria = append(criteria, alternatives)
	}
	if from := strings.TrimSpace(input.FromUser); from != "" {
		criteria = append(criteria, []string{fmt.Sprintf("data_header.from_user = '%s'", from)})
	}
	if to := strings.TrimSpace(input.ToUser); to != "" {
		criteria = append(criteria, []string{fmt.Sprintf("data_header.to_user = '%s'", to)})
	}
	if ua := strings.TrimSpace(input.UA); ua != "" {
		criteria = append(criteria, []string{fmt.Sprintf("data_header.user_agent = '%s'", ua)})
	}
	if method := strings.TrimSpace(input.Method); method != "" {
		criteria = append(criteria, []string{fmt.Sprintf("method = '%s'", strings.ToUpper(method))})
	}
	if query := strings.TrimSpace(input.Query); query != "" {
		parsed, err := ParseQuery(query)
		if err != nil {
			return "", err
		}
		if parsed != "" {
			// A top-level OR must be grouped before AND-merging with other
			// filters; Homer tolerates one level of parentheses.
			if len(criteria) > 0 && strings.Contains(parsed, " OR ") {
				parsed = "(" + parsed + ")"
			}
			criteria = append(criteria, []string{parsed})
		}
	}
	return BuildSmartInput(criteria), nil
}

func clampLimit(limit, fallback, max int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > max {
		return max
	}
	return limit
}

// ---- homer.call.list ----

type CallListInput struct {
	HomerTargetInput
	TimeRangeInput
	Number   string `json:"number,omitempty" jsonschema:"description=Phone number matched against caller OR callee; also sets direction on results."`
	FromUser string `json:"from_user,omitempty" jsonschema:"description=Caller filter (use % as wildcard)."`
	ToUser   string `json:"to_user,omitempty" jsonschema:"description=Callee filter (use % as wildcard)."`
	Query    string `json:"query,omitempty" jsonschema:"description=Query DSL filter, see homer.search."`
	Limit    int    `json:"limit,omitempty" jsonschema:"description=Maximum calls. Default 50, max 200."`
}

// CallSummaryRecord is one grouped call.
type CallSummaryRecord struct {
	CallID    string `json:"call_id"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time,omitempty"`
	Duration  string `json:"duration,omitempty"`
	Caller    string `json:"caller"`
	Callee    string `json:"callee"`
	Direction string `json:"direction,omitempty"`
	Status    string `json:"status,omitempty"`
	MsgCount  int    `json:"msg_count"`
	Route     string `json:"route,omitempty"`
}

type CallListResult struct {
	URL       string              `json:"url"`
	Calls     []CallSummaryRecord `json:"calls"`
	Count     int                 `json:"count"`
	Truncated bool                `json:"truncated,omitempty" jsonschema:"description=True when call discovery was cut off; narrow the window."`
}

func (s Service) CallList(ctx pluginbinding.Context, input CallListInput) (CallListResult, error) {
	client, err := s.client(ctx, input.EndpointRef)
	if err != nil {
		return CallListResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	from, to, err := input.window("1h")
	if err != nil {
		return CallListResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	smartInput, err := buildSearchFilters(SearchInput{Number: input.Number, FromUser: input.FromUser, ToUser: input.ToUser, Query: input.Query})
	if err != nil {
		return CallListResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	limit := clampLimit(input.Limit, defaultCallLimit, maxCallLimit)
	calls, truncated, err := client.FetchCalls(SearchParams{From: from, To: to, SmartInput: smartInput}, strings.TrimSpace(input.Number), limit)
	if err != nil {
		return CallListResult{}, pluginbinding.Errorf("homer", "%s", err)
	}
	records := make([]CallSummaryRecord, 0, len(calls))
	for _, call := range calls {
		records = append(records, callSummaryRecord(call))
	}
	return CallListResult{URL: input.EndpointRef, Calls: records, Count: len(records), Truncated: truncated}, nil
}

func callSummaryRecord(call CallSummary) CallSummaryRecord {
	record := CallSummaryRecord{
		CallID:    call.CallID,
		StartTime: call.StartTime.UTC().Format(time.RFC3339),
		Caller:    call.Caller,
		Callee:    call.Callee,
		Direction: call.Direction,
		Status:    call.Status,
		MsgCount:  call.MsgCount,
		Route:     FormatRoute(DeriveRoute(call.Messages)),
	}
	if call.MsgCount > 1 {
		record.EndTime = call.EndTime.UTC().Format(time.RFC3339)
		record.Duration = formatDuration(call.Duration)
	}
	return record
}

// ---- homer.call.show ----

type CallShowInput struct {
	HomerTargetInput
	TimeRangeInput
	CallIDs    []string `json:"call_ids,omitempty" jsonschema:"required,description=One or more SIP Call-IDs."`
	IncludeRaw bool     `json:"include_raw,omitempty" jsonschema:"description=Attach the full raw SIP message to each flow event."`
}

// FlowEvent is one message in a call flow, ordered by time.
type FlowEvent struct {
	OffsetMS int64  `json:"offset_ms" jsonschema:"description=Milliseconds since the first message of the flow."`
	Time     string `json:"time"`
	CallID   string `json:"call_id"`
	Src      string `json:"src"`
	Dst      string `json:"dst"`
	Method   string `json:"method"`
	CSeq     string `json:"cseq,omitempty"`
	FromUser string `json:"from_user,omitempty"`
	ToUser   string `json:"to_user,omitempty"`
	SDP      string `json:"sdp,omitempty" jsonschema:"description=Compact SDP media annotation like 'PCMA :17818'."`
	Raw      string `json:"raw,omitempty"`
}

type CallShowResult struct {
	URL      string      `json:"url"`
	CallIDs  []string    `json:"call_ids"`
	Events   []FlowEvent `json:"events"`
	Count    int         `json:"count"`
	Ladder   string      `json:"ladder,omitempty" jsonschema:"description=Plain-text message ladder for human reading."`
	Status   string      `json:"status,omitempty"`
	Caller   string      `json:"caller,omitempty"`
	Callee   string      `json:"callee,omitempty"`
	Duration string      `json:"duration,omitempty"`
}

func (s Service) CallShow(ctx pluginbinding.Context, input CallShowInput) (CallShowResult, error) {
	callIDs := trimmedNonEmpty(input.CallIDs)
	if len(callIDs) == 0 {
		return CallShowResult{}, pluginbinding.Fail("bad_input", "at least one call_id is required")
	}
	client, err := s.client(ctx, input.EndpointRef)
	if err != nil {
		return CallShowResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	from, to, err := input.window("24h")
	if err != nil {
		return CallShowResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	params := SearchParams{From: from, To: to, SmartInput: callIDSmartInput(callIDs), Limit: maxSearchLimit}
	search, err := client.SearchCalls(params)
	if err != nil {
		return CallShowResult{}, pluginbinding.Errorf("homer", "%s", err)
	}
	if len(search.Data) == 0 {
		return CallShowResult{}, pluginbinding.Errorf("not_found", "no messages found for call_ids %s in the window — widen since/until", strings.Join(callIDs, ", "))
	}
	transaction, err := client.GetTransaction(params, search.Data)
	if err != nil {
		return CallShowResult{}, pluginbinding.Errorf("homer", "%s", err)
	}
	events := flowEvents(transaction.Data.Messages, input.IncludeRaw)
	calls := GroupCalls(search.Data, "")
	out := CallShowResult{
		URL:     input.EndpointRef,
		CallIDs: callIDs,
		Events:  events,
		Count:   len(events),
		Ladder:  renderLadder(events),
	}
	if len(calls) > 0 {
		first := calls[len(calls)-1] // GroupCalls sorts newest first; take the oldest as the call start
		out.Status = first.Status
		out.Caller = first.Caller
		out.Callee = first.Callee
		if first.MsgCount > 1 {
			out.Duration = formatDuration(first.Duration)
		}
	}
	return out, nil
}

// callIDSmartInput renders "sid = 'a' OR sid = 'b'".
func callIDSmartInput(callIDs []string) string {
	alternatives := make([]string, 0, len(callIDs))
	for _, id := range callIDs {
		alternatives = append(alternatives, fmt.Sprintf("sid = '%s'", id))
	}
	return BuildSmartInput([][]string{alternatives})
}

// flowEvents converts transaction SIP messages into ordered flow events.
func flowEvents(messages []TransactionMessage, includeRaw bool) []FlowEvent {
	var sips []TransactionMessage
	for _, message := range messages {
		if message.IsSIP() {
			sips = append(sips, message)
		}
	}
	sort.Slice(sips, func(i, j int) bool {
		if sips[i].MicroTS != sips[j].MicroTS {
			return sips[i].MicroTS < sips[j].MicroTS
		}
		return sips[i].CreateDate < sips[j].CreateDate
	})
	var firstMS int64
	events := make([]FlowEvent, 0, len(sips))
	for _, message := range sips {
		when := time.UnixMilli(message.CreateDate)
		if firstMS == 0 {
			firstMS = message.CreateDate
		}
		event := FlowEvent{
			OffsetMS: message.CreateDate - firstMS,
			Time:     when.UTC().Format(time.RFC3339Nano),
			CallID:   message.CallID,
			Src:      message.SrcIP + ":" + strconv.Itoa(message.SrcPort),
			Dst:      message.DstIP + ":" + strconv.Itoa(message.DstPort),
			Method:   firstNonEmpty(message.Method, firstToken(message.Raw)),
			CSeq:     message.CSeq,
			FromUser: message.FromUser,
			ToUser:   message.ToUser,
			SDP:      ExtractSDPMedia(message.Raw),
		}
		if includeRaw {
			event.Raw = message.Raw
		}
		events = append(events, event)
	}
	return events
}

func firstToken(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// renderLadder renders flow events as an aligned plain-text ladder.
func renderLadder(events []FlowEvent) string {
	if len(events) == 0 {
		return ""
	}
	var b strings.Builder
	srcWidth, dstWidth := 0, 0
	for _, event := range events {
		if len(event.Src) > srcWidth {
			srcWidth = len(event.Src)
		}
		if len(event.Dst) > dstWidth {
			dstWidth = len(event.Dst)
		}
	}
	for _, event := range events {
		line := fmt.Sprintf("%8s  %-*s → %-*s  %s", "+"+strconv.FormatInt(event.OffsetMS, 10)+"ms", srcWidth, event.Src, dstWidth, event.Dst, event.Method)
		if event.SDP != "" {
			line += " (" + event.SDP + ")"
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// ---- homer.call.qos ----

type CallQoSInput struct {
	HomerTargetInput
	TimeRangeInput
	CallIDs   []string `json:"call_ids,omitempty" jsonschema:"required,description=One or more SIP Call-IDs."`
	ClockRate int      `json:"clock_rate,omitempty" jsonschema:"description=RTP clock rate in Hz for jitter conversion. Default 8000 (G.711); use 16000 for wideband."`
	LatencyMS int      `json:"latency_ms,omitempty" jsonschema:"description=Assumed one-way network latency in ms for the MOS estimate. Default 20."`
}

type CallQoSResult struct {
	URL     string          `json:"url"`
	CallIDs []string        `json:"call_ids"`
	Streams []StreamMetrics `json:"streams"`
	Count   int             `json:"count"`
	Note    string          `json:"note,omitempty"`
}

func (s Service) CallQoS(ctx pluginbinding.Context, input CallQoSInput) (CallQoSResult, error) {
	callIDs := trimmedNonEmpty(input.CallIDs)
	if len(callIDs) == 0 {
		return CallQoSResult{}, pluginbinding.Fail("bad_input", "at least one call_id is required")
	}
	client, err := s.client(ctx, input.EndpointRef)
	if err != nil {
		return CallQoSResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	from, to, err := input.window("24h")
	if err != nil {
		return CallQoSResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	params := SearchParams{From: from, To: to, SmartInput: callIDSmartInput(callIDs), Limit: maxSearchLimit}
	search, err := client.SearchCalls(params)
	if err != nil {
		return CallQoSResult{}, pluginbinding.Errorf("homer", "%s", err)
	}
	if len(search.Data) == 0 {
		return CallQoSResult{}, pluginbinding.Errorf("not_found", "no messages found for call_ids %s in the window — widen since/until", strings.Join(callIDs, ", "))
	}
	qos, err := client.GetQoS(params, search.Data)
	if err != nil {
		return CallQoSResult{}, pluginbinding.Errorf("homer", "%s", err)
	}
	clockRate := input.ClockRate
	if clockRate <= 0 {
		clockRate = 8000
	}
	latency := float64(input.LatencyMS)
	if latency <= 0 {
		latency = 20
	}
	streams := AggregateStreams(qos, clockRate, latency)
	out := CallQoSResult{URL: input.EndpointRef, CallIDs: callIDs, Streams: streams, Count: len(streams)}
	if len(streams) == 0 {
		out.Note = "no RTCP reports captured for these calls"
	}
	return out, nil
}

// ---- homer.pcap.export ----

type PCAPExportInput struct {
	HomerTargetInput
	TimeRangeInput
	CallIDs []string `json:"call_ids,omitempty" jsonschema:"required,description=One or more SIP Call-IDs to export."`
}

type PCAPExportResult struct {
	URL      string `json:"url"`
	BlobRef  string `json:"blob_ref,omitempty"`
	Path     string `json:"path,omitempty"`
	Filename string `json:"filename"`
	Bytes    int    `json:"bytes"`
}

func (s Service) PCAPExport(ctx pluginbinding.Context, input PCAPExportInput) (PCAPExportResult, error) {
	callIDs := trimmedNonEmpty(input.CallIDs)
	if len(callIDs) == 0 {
		return PCAPExportResult{}, pluginbinding.Fail("bad_input", "at least one call_id is required")
	}
	client, err := s.client(ctx, input.EndpointRef)
	if err != nil {
		return PCAPExportResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	from, to, err := input.window("24h")
	if err != nil {
		return PCAPExportResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	data, err := client.ExportPCAP(SearchParams{From: from, To: to, SmartInput: callIDSmartInput(callIDs), Limit: maxSearchLimit})
	if err != nil {
		return PCAPExportResult{}, pluginbinding.Errorf("homer", "%s", err)
	}
	if len(data) == 0 {
		return PCAPExportResult{}, pluginbinding.Errorf("not_found", "homer returned an empty pcap for call_ids %s", strings.Join(callIDs, ", "))
	}
	filename := "homer-" + sanitizeFilename(callIDs[0]) + ".pcap"
	blob, err := ctx.Host.BlobWrite(pluginbinding.BlobWriteRequest{
		Content:   data,
		MediaType: "application/vnd.tcpdump.pcap",
		Filename:  filename,
		Metadata:  map[string]string{"call_ids": strings.Join(callIDs, ",")},
	})
	if err != nil {
		return PCAPExportResult{}, pluginbinding.Errorf("homer", "store pcap blob: %s", err)
	}
	return PCAPExportResult{URL: input.EndpointRef, BlobRef: blob.Ref, Path: blob.Path, Filename: filename, Bytes: len(data)}, nil
}

func sanitizeFilename(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// ---- homer.alias.list ----

type AliasListInput struct {
	HomerTargetInput
}

type AliasRecord struct {
	IP     string `json:"ip"`
	Port   int    `json:"port,omitempty"`
	Alias  string `json:"alias"`
	Active bool   `json:"active"`
}

type AliasListResult struct {
	URL     string        `json:"url"`
	Aliases []AliasRecord `json:"aliases"`
	Count   int           `json:"count"`
}

func (s Service) AliasList(ctx pluginbinding.Context, input AliasListInput) (AliasListResult, error) {
	client, err := s.client(ctx, input.EndpointRef)
	if err != nil {
		return AliasListResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	aliases, err := client.ListAliases()
	if err != nil {
		return AliasListResult{}, pluginbinding.Errorf("homer", "%s", err)
	}
	records := make([]AliasRecord, 0, len(aliases))
	for _, alias := range aliases {
		records = append(records, AliasRecord{IP: alias.IP, Port: int(alias.Port), Alias: alias.Alias, Active: alias.Status})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Alias < records[j].Alias })
	return AliasListResult{URL: input.EndpointRef, Aliases: records, Count: len(records)}, nil
}

func trimmedNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
