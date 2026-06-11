package homer

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// CallAnalyzeInput drives multi-leg call analysis: a seed call is located,
// candidate legs are found by fanning out on the involved numbers, and legs
// are confirmed by a shared correlation header value plus temporal overlap.
type CallAnalyzeInput struct {
	HomerTargetInput
	TimeRangeInput
	CallID            string   `json:"call_id,omitempty" jsonschema:"description=Seed Call-ID. Provide this or from_user+to_user."`
	FromUser          string   `json:"from_user,omitempty" jsonschema:"description=Seed caller (used with to_user when call_id is unknown)."`
	ToUser            string   `json:"to_user,omitempty" jsonschema:"description=Seed callee (used with from_user when call_id is unknown)."`
	CorrelationHeader string   `json:"correlation_header,omitempty" jsonschema:"required,description=SIP header whose value ties the legs of one logical call together (e.g. X-CID)."`
	Numbers           []string `json:"numbers,omitempty" jsonschema:"description=Extra numbers (agents\\, extensions) to widen the leg search; legs involving them are included even without the correlation header."`
	Headers           []string `json:"headers,omitempty" jsonschema:"description=Additional SIP headers to extract from each leg's INVITE for the report."`
	Limit             int      `json:"limit,omitempty" jsonschema:"description=Maximum candidate legs from the fan-out. Default 50\\, max 200."`
}

// CallLeg is one confirmed leg of a multi-leg call.
type CallLeg struct {
	CallID      string            `json:"call_id"`
	Seed        bool              `json:"seed,omitempty"`
	StartTime   string            `json:"start_time"`
	Duration    string            `json:"duration,omitempty"`
	From        string            `json:"from"`
	To          string            `json:"to"`
	Status      string            `json:"status,omitempty"`
	Route       string            `json:"route,omitempty"`
	Correlation string            `json:"correlation,omitempty" jsonschema:"description=The correlation header value found on this leg's INVITE."`
	Headers     map[string]string `json:"headers,omitempty"`
	MatchedBy   string            `json:"matched_by,omitempty" jsonschema:"description=seed\\, correlation\\, or number."`
}

type CallAnalyzeResult struct {
	URL               string      `json:"url"`
	SeedCallID        string      `json:"seed_call_id"`
	CorrelationHeader string      `json:"correlation_header"`
	CorrelationValues []string    `json:"correlation_values,omitempty"`
	Legs              []CallLeg   `json:"legs"`
	LegCount          int         `json:"leg_count"`
	Events            []FlowEvent `json:"events,omitempty"`
	EventCount        int         `json:"event_count"`
	Ladder            string      `json:"ladder,omitempty"`
}

// CallAnalyze finds the legs of a multi-leg call:
//  1. locate the seed call (by call_id, or unambiguously by from/to),
//  2. fan out by the seed caller and any extra numbers (±30m margin),
//  3. extract the correlation header from every candidate INVITE,
//  4. keep correlation groups that temporally overlap the seed
//     (legs spawn within seconds of the external INVITE),
//  5. additionally keep fan-out legs that involve an extra number even
//     without the header (explicit user intent).
func (s Service) CallAnalyze(ctx pluginbinding.Context, input CallAnalyzeInput) (CallAnalyzeResult, error) {
	header := strings.TrimSpace(input.CorrelationHeader)
	if header == "" {
		return CallAnalyzeResult{}, pluginbinding.Fail("bad_input", "correlation_header is required (the SIP header that ties call legs together)")
	}
	seedByID := strings.TrimSpace(input.CallID) != ""
	if !seedByID && (strings.TrimSpace(input.FromUser) == "" || strings.TrimSpace(input.ToUser) == "") {
		return CallAnalyzeResult{}, pluginbinding.Fail("bad_input", "provide call_id, or from_user and to_user")
	}
	client, err := s.client(ctx, input.EndpointRef)
	if err != nil {
		return CallAnalyzeResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	from, to, err := input.window("6h")
	if err != nil {
		return CallAnalyzeResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}

	// Step 1: locate the seed call.
	seedParams := SearchParams{From: from, To: to, Limit: defaultSearchLimit}
	if seedByID {
		seedParams.CallID = strings.TrimSpace(input.CallID)
	} else {
		criteria := [][]string{
			NumberAlternatives("data_header.from_user", input.FromUser),
			NumberAlternatives("data_header.to_user", input.ToUser),
		}
		seedParams.SmartInput = BuildSmartInput(criteria)
	}
	seedResult, err := client.SearchCalls(seedParams)
	if err != nil {
		return CallAnalyzeResult{}, pluginbinding.Errorf("homer", "seed search: %s", err)
	}
	seedCalls := GroupCalls(seedResult.Data, "")
	if len(seedCalls) == 0 {
		return CallAnalyzeResult{}, pluginbinding.Errorf("not_found", "no seed call found — widen since/until or check the call_id")
	}
	if !seedByID && len(seedCalls) > 1 {
		candidates := make([]string, 0, len(seedCalls))
		for _, call := range seedCalls {
			candidates = append(candidates, fmt.Sprintf("%s %s %s->%s", call.StartTime.UTC().Format(time.RFC3339), call.CallID, call.Caller, call.Callee))
		}
		return CallAnalyzeResult{}, pluginbinding.Errorf("ambiguous", "found %d calls matching from/to; re-run with one call_id: %s", len(seedCalls), strings.Join(candidates, " | "))
	}
	seedCall := seedCalls[len(seedCalls)-1] // oldest matching group when several windows of the same sid

	// Step 2: fan out by seed caller + extra numbers around the seed window.
	margin := 30 * time.Minute
	fanParams := SearchParams{From: seedCall.StartTime.Add(-margin), To: seedCall.EndTime.Add(margin)}
	var fanAlternatives []string
	fanAlternatives = append(fanAlternatives, NumberAlternatives("data_header.from_user", seedCall.Caller)...)
	for _, number := range trimmedNonEmpty(input.Numbers) {
		fanAlternatives = append(fanAlternatives, NumberAlternatives("data_header.from_user", number)...)
		fanAlternatives = append(fanAlternatives, NumberAlternatives("data_header.to_user", number)...)
	}
	if len(fanAlternatives) > 0 {
		fanParams.SmartInput = BuildSmartInput([][]string{fanAlternatives})
	}
	limit := clampLimit(input.Limit, defaultCallLimit, maxCallLimit)
	fanCalls, _, err := client.FetchCalls(fanParams, "", limit)
	if err != nil {
		return CallAnalyzeResult{}, pluginbinding.Errorf("homer", "fan-out search: %s", err)
	}
	var fanRecords []CallRecord
	for _, call := range fanCalls {
		fanRecords = append(fanRecords, call.Messages...)
	}
	merged := MergeSearchResults(&SearchResult{Data: fanRecords}, seedResult)

	// Step 3: extract the correlation header from all candidate INVITEs.
	transaction, err := client.GetTransaction(fanParams, merged.Data)
	if err != nil {
		return CallAnalyzeResult{}, pluginbinding.Errorf("homer", "candidate transaction: %s", err)
	}
	extraHeaders := trimmedNonEmpty(input.Headers)
	callIDCorrelation := map[string]string{}        // callID -> correlation value
	valueCallIDs := map[string]map[string]bool{}    // correlation value -> callIDs
	callIDHeaders := map[string]map[string]string{} // callID -> extra header values
	inviteRawByCallID := map[string]string{}        // first INVITE per leg
	for _, message := range transaction.Data.Messages {
		if !message.IsSIP() || message.Raw == "" || !strings.HasPrefix(message.Raw, "INVITE ") {
			continue
		}
		if _, seen := inviteRawByCallID[message.CallID]; !seen {
			inviteRawByCallID[message.CallID] = message.Raw
		}
		if value := ExtractSIPHeader(message.Raw, header); value != "" {
			callIDCorrelation[message.CallID] = value
			if valueCallIDs[value] == nil {
				valueCallIDs[value] = map[string]bool{}
			}
			valueCallIDs[value][message.CallID] = true
		}
		for _, extra := range extraHeaders {
			if value := ExtractSIPHeader(message.Raw, extra); value != "" {
				if callIDHeaders[message.CallID] == nil {
					callIDHeaders[message.CallID] = map[string]string{}
				}
				callIDHeaders[message.CallID][extra] = value
			}
		}
	}

	// Step 4: keep correlation groups that temporally overlap the seed.
	allCandidates := GroupCalls(merged.Data, "")
	candidateByCallID := map[string]CallSummary{}
	for _, call := range allCandidates {
		candidateByCallID[call.CallID] = call
	}
	matched := map[string]string{seedCall.CallID: "seed"}
	var correlationValues []string
	for value, callIDs := range valueCallIDs {
		overlaps := false
		for callID := range callIDs {
			if call, ok := candidateByCallID[callID]; ok {
				if call.StartTime.After(seedCall.StartTime.Add(-5*time.Second)) && call.StartTime.Before(seedCall.StartTime.Add(30*time.Second)) {
					overlaps = true
					break
				}
			}
		}
		if !overlaps {
			continue
		}
		correlationValues = append(correlationValues, value)
		for callID := range callIDs {
			if _, ok := matched[callID]; !ok {
				matched[callID] = "correlation"
			}
		}
	}
	sort.Strings(correlationValues)

	// Step 5: include fan-out legs that involve an extra number even without
	// the correlation header — the number expresses explicit user intent.
	extraNumberSet := map[string]bool{}
	for _, number := range trimmedNonEmpty(input.Numbers) {
		extraNumberSet[strings.TrimPrefix(number, "+")] = true
	}
	if len(extraNumberSet) > 0 {
		for _, call := range allCandidates {
			if _, ok := matched[call.CallID]; ok {
				continue
			}
			if extraNumberSet[strings.TrimPrefix(call.Caller, "+")] || extraNumberSet[strings.TrimPrefix(call.Callee, "+")] {
				matched[call.CallID] = "number"
			}
		}
	}

	// Build the leg report (ordered by start time) and the merged flow.
	var legs []CallLeg
	for callID, matchedBy := range matched {
		call, ok := candidateByCallID[callID]
		if !ok {
			continue
		}
		leg := CallLeg{
			CallID:      callID,
			Seed:        callID == seedCall.CallID,
			StartTime:   call.StartTime.UTC().Format(time.RFC3339),
			From:        call.Caller,
			To:          call.Callee,
			Status:      call.Status,
			Route:       FormatRoute(DeriveRoute(call.Messages)),
			Correlation: callIDCorrelation[callID],
			Headers:     callIDHeaders[callID],
			MatchedBy:   matchedBy,
		}
		if call.MsgCount > 1 {
			leg.Duration = formatDuration(call.Duration)
		}
		legs = append(legs, leg)
	}
	sort.Slice(legs, func(i, j int) bool { return legs[i].StartTime < legs[j].StartTime })

	// Merged flow across all legs comes from the already-fetched transaction,
	// filtered to the matched Call-IDs.
	var matchedMessages []TransactionMessage
	for _, message := range transaction.Data.Messages {
		if _, ok := matched[message.CallID]; ok {
			matchedMessages = append(matchedMessages, message)
		}
	}
	events := flowEvents(matchedMessages, false, nil)

	return CallAnalyzeResult{
		URL:               input.EndpointRef,
		SeedCallID:        seedCall.CallID,
		CorrelationHeader: header,
		CorrelationValues: correlationValues,
		Legs:              legs,
		LegCount:          len(legs),
		Events:            events,
		EventCount:        len(events),
		Ladder:            renderLadder(events),
	}, nil
}
