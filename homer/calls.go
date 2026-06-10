package homer

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// CallSummary is a grouped view of a SIP call (all messages sharing one
// Call-ID).
type CallSummary struct {
	CallID    string        `json:"call_id"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"-"`
	Caller    string        `json:"caller"`
	Callee    string        `json:"callee"`
	Direction string        `json:"direction,omitempty"` // "IN", "OUT", or ""
	Status    string        `json:"status"`              // answered, busy, cancelled, no answer, failed, ringing
	MsgCount  int           `json:"msg_count"`
	Messages  []CallRecord  `json:"-"`
}

// GroupCalls groups raw SIP messages by Call-ID and produces call summaries,
// newest first. If number is non-empty, direction is detected relative to it.
func GroupCalls(records []CallRecord, number string) []CallSummary {
	groups := map[string][]CallRecord{}
	for _, record := range records {
		groups[record.CallID] = append(groups[record.CallID], record)
	}
	var summaries []CallSummary
	for callID, msgs := range groups {
		sort.Slice(msgs, func(i, j int) bool { return msgs[i].Date < msgs[j].Date })
		summary := CallSummary{CallID: callID, MsgCount: len(msgs), Messages: msgs}
		if len(msgs) > 0 {
			summary.StartTime = time.UnixMilli(msgs[0].Date)
			summary.EndTime = time.UnixMilli(msgs[len(msgs)-1].Date)
			summary.Duration = summary.EndTime.Sub(summary.StartTime)
		}
		// Caller/callee from the first INVITE, falling back to the first message.
		for _, m := range msgs {
			if m.Method == "INVITE" {
				summary.Caller = m.FromUser
				summary.Callee = firstNonEmpty(m.ToUser, m.RuriUser)
				break
			}
		}
		if summary.Caller == "" && len(msgs) > 0 {
			summary.Caller = msgs[0].FromUser
			summary.Callee = firstNonEmpty(msgs[0].ToUser, msgs[0].RuriUser)
		}
		if number != "" {
			summary.Direction = detectDirection(summary.Caller, summary.Callee, number)
		}
		summary.Status = deriveStatus(msgs)
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].StartTime.After(summaries[j].StartTime) })
	return summaries
}

// FetchCalls discovers calls matching the search parameters, returning up to
// maxCalls summaries plus whether discovery was cut off. Homer's search
// returns messages, not calls, so unique Call-IDs are discovered via bounded
// backward pagination (up to maxBatches x batchLimit messages, walking the
// window backwards from the newest message).
func (c *Client) FetchCalls(params SearchParams, number string, maxCalls int) ([]CallSummary, bool, error) {
	const (
		batchLimit = 200 // messages per discovery request (safe for the Homer API)
		maxBatches = 5   // max discovery iterations to avoid runaway requests
	)
	seenCallIDs := map[string]bool{}
	var allDiscovered []CallRecord
	discoverTo := params.To
	exhausted := false

	for batch := 0; batch < maxBatches; batch++ {
		if !discoverTo.After(params.From) {
			exhausted = true
			break
		}
		batchParams := params
		batchParams.To = discoverTo
		batchParams.Limit = batchLimit
		result, err := c.SearchCalls(batchParams)
		if err != nil {
			return nil, false, err
		}
		if len(result.Data) == 0 {
			exhausted = true
			break
		}
		var minTS int64
		for i := range result.Data {
			seenCallIDs[result.Data[i].CallID] = true
			if minTS == 0 || result.Data[i].Date < minTS {
				minTS = result.Data[i].Date
			}
		}
		allDiscovered = append(allDiscovered, result.Data...)
		if len(result.Data) < batchLimit {
			exhausted = true
			break
		}
		if len(seenCallIDs) >= maxCalls {
			break
		}
		// Advance the window to just before the oldest message received.
		discoverTo = time.UnixMilli(minTS).Add(-time.Millisecond)
	}

	if len(allDiscovered) == 0 {
		return nil, false, nil
	}
	calls := GroupCalls(allDiscovered, number)
	truncated := !exhausted || len(calls) > maxCalls
	if len(calls) > maxCalls {
		calls = calls[:maxCalls]
	}
	return calls, truncated, nil
}

// MergeSearchResults deduplicates two search results by message ID.
func MergeSearchResults(a, b *SearchResult) *SearchResult {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	seen := make(map[float64]bool, len(a.Data))
	for _, record := range a.Data {
		seen[record.ID] = true
	}
	merged := make([]CallRecord, len(a.Data))
	copy(merged, a.Data)
	for _, record := range b.Data {
		if !seen[record.ID] {
			merged = append(merged, record)
			seen[record.ID] = true
		}
	}
	return &SearchResult{Data: merged}
}

// DeriveRoute extracts unique IP hop pairs from a call's messages in order of
// first appearance.
func DeriveRoute(msgs []CallRecord) [][2]string {
	var pairs [][2]string
	seen := map[[2]string]bool{}
	for _, m := range msgs {
		pair := [2]string{m.SourceIP, m.DestIP}
		if !seen[pair] {
			seen[pair] = true
			pairs = append(pairs, pair)
		}
	}
	return pairs
}

// FormatRoute formats IP hop pairs into a compact chain like "A -> B -> C",
// collapsing consecutive hops that share an endpoint.
func FormatRoute(pairs [][2]string) string {
	if len(pairs) == 0 {
		return ""
	}
	chain := []string{pairs[0][0], pairs[0][1]}
	for i := 1; i < len(pairs); i++ {
		if pairs[i][0] == chain[len(chain)-1] {
			chain = append(chain, pairs[i][1])
		} else {
			chain = append(chain, pairs[i][0], pairs[i][1])
		}
	}
	return strings.Join(chain, " → ")
}

// detectDirection determines call direction relative to the given number:
// number matches caller -> OUT, matches callee -> IN.
func detectDirection(caller, callee, number string) string {
	norm := strings.TrimPrefix(number, "+")
	normCaller := strings.TrimPrefix(caller, "+")
	normCallee := strings.TrimPrefix(callee, "+")
	if normCaller != "" && (strings.Contains(normCaller, norm) || strings.Contains(norm, normCaller)) {
		return "OUT"
	}
	if normCallee != "" && (strings.Contains(normCallee, norm) || strings.Contains(norm, normCallee)) {
		return "IN"
	}
	return ""
}

// deriveStatus checks SIP response codes to determine the call outcome.
// Response messages carry numeric strings ("200", "486") in the Method field.
func deriveStatus(msgs []CallRecord) string {
	var highestResponse int
	for _, m := range msgs {
		if code, err := strconv.Atoi(m.Method); err == nil && code >= 100 && code > highestResponse {
			highestResponse = code
		}
		if statusCode := int(m.Status); statusCode >= 100 && statusCode > highestResponse {
			highestResponse = statusCode
		}
	}
	switch {
	case highestResponse >= 200 && highestResponse < 300:
		return "answered"
	case highestResponse == 486:
		return "busy"
	case highestResponse == 487:
		return "cancelled"
	case highestResponse == 408 || highestResponse == 480:
		return "no answer"
	case highestResponse >= 400:
		return "failed"
	case highestResponse >= 100:
		return "ringing"
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// formatDuration renders a duration compactly ("53s", "18m12s", "1h5m").
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return strconv.FormatInt(d.Milliseconds(), 10) + "ms"
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return strconv.Itoa(int(d.Seconds())) + "s"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		if s == 0 {
			return strconv.Itoa(m) + "m"
		}
		return strconv.Itoa(m) + "m" + strconv.Itoa(s) + "s"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return strconv.Itoa(h) + "h"
	}
	return strconv.Itoa(h) + "h" + strconv.Itoa(m) + "m"
}
