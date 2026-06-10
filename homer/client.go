package homer

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// Client speaks the Homer 7.x REST API through the host HTTP capability. All
// requests resolve the registered endpoint ref on the host side; the plugin
// performs no direct network IO. Authentication is a JWT obtained from
// POST /api/v3/auth with credentials resolved ONLY from the host secret store
// (persisted once via `fluxplane-plugin auth connect homer`) — never from
// environment variables at invoke time.
type Client struct {
	EndpointRef string
	Host        pluginbinding.HostClient
	token       string
}

// SearchParams holds the shared query window for Homer API calls.
type SearchParams struct {
	From       time.Time
	To         time.Time
	SmartInput string
	CallID     string
	Limit      int
}

// SearchResult holds the response from a call search.
type SearchResult struct {
	Data []CallRecord `json:"data"`
}

// CallRecord is one raw message record from the Homer search API.
type CallRecord struct {
	ID         float64 `json:"id"`
	Date       int64   `json:"create_date"`
	MicroTS    int64   `json:"micro_ts"`
	Protocol   float64 `json:"protocol"`
	SourceIP   string  `json:"srcIp"`
	SourcePort float64 `json:"srcPort"`
	DestIP     string  `json:"dstIp"`
	DestPort   float64 `json:"dstPort"`
	CallID     string  `json:"sid"`
	Method     string  `json:"method"`
	MethodText string  `json:"method_text"`
	FromUser   string  `json:"from_user"`
	ToUser     string  `json:"to_user"`
	RuriUser   string  `json:"ruri_user"`
	UserAgent  string  `json:"user_agent"`
	CSeq       string  `json:"cseq"`
	Status     float64 `json:"status"`
	AliasSrc   string  `json:"aliasSrc"`
	AliasDst   string  `json:"aliasDst"`
	Table      string  `json:"table"`
}

// TransactionResult holds the response from a call transaction query.
type TransactionResult struct {
	Data struct {
		Messages []TransactionMessage `json:"messages"`
	} `json:"data"`
	Total int `json:"total"`
}

// TransactionMessage is one message with its raw content. The transaction
// endpoint also returns RTCP/RTP messages (profile "5_default"/"35_default")
// which lack SIP fields; filter with IsSIP.
type TransactionMessage struct {
	ID         int             `json:"id"`
	CallID     string          `json:"sid"`
	Method     string          `json:"method,omitempty"`
	SrcIP      string          `json:"srcIp"`
	SrcPort    int             `json:"srcPort"`
	DstIP      string          `json:"dstIp"`
	DstPort    int             `json:"dstPort"`
	CreateDate int64           `json:"create_date"`
	MicroTS    int64           `json:"micro_ts"`
	Raw        string          `json:"raw"`
	FromUser   string          `json:"from_user,omitempty"`
	ToUser     string          `json:"to_user,omitempty"`
	CSeq       string          `json:"cseq,omitempty"`
	Protocol   int             `json:"protocol"`
	Profile    string          `json:"profile,omitempty"`
	DBNode     string          `json:"dbnode"`
	Node       json.RawMessage `json:"node"` // string or []string depending on Homer version
}

// IsSIP reports whether this is a SIP message (not RTCP/RTP/QoS).
func (m TransactionMessage) IsSIP() bool {
	return m.Profile == "" || m.Profile == "1_call" || m.Profile == "1_default" || m.Profile == "1_registration"
}

// Alias is one Homer IP/port alias.
type Alias struct {
	ID        float64 `json:"id"`
	IP        string  `json:"ip"`
	Port      float64 `json:"port"`
	Mask      float64 `json:"mask"`
	Alias     string  `json:"alias"`
	Status    bool    `json:"status"`
	CaptureID string  `json:"captureID"`
}

// login authenticates against /api/v3/auth and caches the JWT for the rest of
// the invocation. Credentials come exclusively from the host secret store.
func (c *Client) login() error {
	if c.token != "" {
		return nil
	}
	if c.Host == nil {
		return fmt.Errorf("host client is unavailable")
	}
	username, err := c.Host.Secret(SecretPurposeUsername)
	if err != nil {
		return fmt.Errorf("homer username secret is not connected — run: fluxplane-plugin auth connect homer")
	}
	password, err := c.Host.Secret(SecretPurposePassword)
	if err != nil {
		return fmt.Errorf("homer password secret is not connected — run: fluxplane-plugin auth connect homer")
	}
	payload, err := json.Marshal(map[string]string{
		"username": strings.TrimSpace(username.Value),
		"password": strings.TrimSpace(password.Value),
	})
	if err != nil {
		return err
	}
	resp, err := c.do("POST", "/api/v3/auth", payload, "")
	if err != nil {
		return err
	}
	var auth struct {
		Token   string `json:"token"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp, &auth); err != nil {
		return fmt.Errorf("decode auth response: %w", err)
	}
	if auth.Token == "" {
		return fmt.Errorf("homer authentication returned no token: %s", auth.Message)
	}
	c.token = auth.Token
	return nil
}

// Check probes the unauthenticated agent check endpoint.
func (c *Client) Check() error {
	_, err := c.do("GET", "/api/v3/agent/check", nil, "")
	return err
}

// SearchCalls searches for SIP messages matching the given parameters.
func (c *Client) SearchCalls(params SearchParams) (*SearchResult, error) {
	body, err := c.authRequest("POST", "/api/v3/search/call/data", buildSearchPayload(params))
	if err != nil {
		return nil, fmt.Errorf("search calls: %w", err)
	}
	var result SearchResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode search result: %w", err)
	}
	return &result, nil
}

// GetTransaction fetches full message details (including raw SIP bodies) for
// the calls in searchData (from a prior SearchCalls).
func (c *Client) GetTransaction(params SearchParams, searchData []CallRecord) (*TransactionResult, error) {
	body, err := c.authRequest("POST", "/api/v3/call/transaction", buildTransactionPayload(params, searchData))
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}
	var result TransactionResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode transaction result: %w", err)
	}
	return &result, nil
}

// GetQoS fetches RTCP quality-of-service reports for the given call records.
func (c *Client) GetQoS(params SearchParams, searchData []CallRecord) (*QoSResult, error) {
	body, err := c.authRequest("POST", "/api/v3/call/report/qos", buildTransactionPayload(params, searchData))
	if err != nil {
		return nil, fmt.Errorf("get qos: %w", err)
	}
	var result QoSResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode qos result: %w", err)
	}
	return &result, nil
}

// ExportPCAP exports the matching call messages as raw PCAP bytes.
func (c *Client) ExportPCAP(params SearchParams) ([]byte, error) {
	body, err := c.authRequest("POST", "/api/v3/export/call/messages/pcap", buildSearchPayload(params))
	if err != nil {
		return nil, fmt.Errorf("export pcap: %w", err)
	}
	return body, nil
}

// ListAliases returns the configured IP/port aliases.
func (c *Client) ListAliases() ([]Alias, error) {
	body, err := c.authRequest("GET", "/api/v3/alias", nil)
	if err != nil {
		return nil, fmt.Errorf("list aliases: %w", err)
	}
	var resp struct {
		Data []Alias `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode aliases: %w", err)
	}
	return resp.Data, nil
}

func (c *Client) authRequest(method, path string, payload any) ([]byte, error) {
	if err := c.login(); err != nil {
		return nil, err
	}
	var body []byte
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = data
	}
	return c.do(method, path, body, c.token)
}

func (c *Client) do(method, path string, body []byte, token string) ([]byte, error) {
	if strings.TrimSpace(c.EndpointRef) == "" {
		return nil, fmt.Errorf("homer endpoint_ref is required")
	}
	headers := map[string]string{}
	if len(body) > 0 {
		headers["Content-Type"] = "application/json"
	}
	if token != "" {
		// The host only injects Authorization when the plugin leaves it unset,
		// so the invocation-scoped JWT is passed explicitly here.
		headers["Authorization"] = "Bearer " + token
	}
	resp, err := c.Host.HTTP(pluginbinding.HTTPRequest{
		EndpointRef: strings.TrimSpace(c.EndpointRef),
		Path:        path,
		Method:      method,
		Headers:     headers,
		Body:        body,
		TimeoutMS:   30000,
		MaxBytes:    64 * 1024 * 1024,
		UserAgent:   "fluxplane-plugin-homer/" + PluginVersion,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("homer returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(resp.Body)))
	}
	return resp.Body, nil
}

// buildSearchPayload constructs the Homer search API request body. The CallID,
// when set, is merged into smartinput (the named "sid" filter does not work in
// Homer 7).
func buildSearchPayload(params SearchParams) map[string]any {
	limit := params.Limit
	if limit <= 0 {
		limit = 200
	}
	filters := []map[string]any{
		{"name": "limit", "value": fmt.Sprintf("%d", limit), "type": "string", "hepid": 1},
	}
	smartInput := params.SmartInput
	if params.CallID != "" {
		sidExpr := fmt.Sprintf("sid = '%s'", params.CallID)
		if smartInput != "" {
			smartInput = sidExpr + " AND " + smartInput
		} else {
			smartInput = sidExpr
		}
	}
	if smartInput != "" {
		filters = append(filters, map[string]any{
			"name": "smartinput", "value": smartInput, "type": "string", "hepid": 1,
		})
	}
	return map[string]any{
		"config": map[string]any{
			"protocol_id":      map[string]any{"name": "SIP", "value": 1},
			"protocol_profile": map[string]any{"name": "call", "value": "call"},
		},
		"param": map[string]any{
			"transaction": map[string]any{},
			"limit":       limit,
			"search": map[string]any{
				"1_call": filters,
			},
			"location": map[string]any{},
			"timezone": homerTimezone(),
		},
		"timestamp": map[string]any{
			"from": params.From.UnixMilli(),
			"to":   params.To.UnixMilli(),
		},
	}
}

// buildTransactionPayload constructs the shared request body used by the
// transaction and QoS endpoints from a prior search's records.
func buildTransactionPayload(params SearchParams, searchData []CallRecord) map[string]any {
	callIDs := map[string]bool{}
	var firstID float64
	for _, record := range searchData {
		callIDs[record.CallID] = true
		if firstID == 0 {
			firstID = record.ID
		}
	}
	callIDList := make([]string, 0, len(callIDs))
	for id := range callIDs {
		callIDList = append(callIDList, id)
	}
	return map[string]any{
		"param": map[string]any{
			"search": map[string]any{
				"1_call": map[string]any{
					"id":     firstID,
					"callid": callIDList,
					"uuid":   []any{},
				},
			},
			"location": map[string]any{
				"node": []string{"local"},
			},
			"timezone": homerTimezone(),
			"transaction": map[string]any{
				"call":         true,
				"registration": false,
				"rest":         false,
			},
		},
		"timestamp": map[string]any{
			"from": params.From.UnixMilli(),
			"to":   params.To.UnixMilli(),
		},
	}
}

// homerTimezone reports the local UTC offset in Homer's convention (negative
// of the offset, in minutes).
func homerTimezone() map[string]any {
	_, offsetSec := time.Now().Zone()
	return map[string]any{"name": "Local", "value": -(offsetSec / 60)}
}

// FormatUserAgent compacts raw SIP User-Agent strings for display:
// "Asterisk PBX 11.13.1~dfsg-2+deb8u4" -> "Asterisk 11.13.1",
// "FPBX-15.0.16.75(16.13.0)" -> "FPBX 16.13.0".
func FormatUserAgent(ua string) string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return ""
	}
	if strings.Contains(strings.ToLower(ua), "asterisk") {
		for i := 0; i < len(ua); i++ {
			if ua[i] >= '0' && ua[i] <= '9' {
				end := i
				for end < len(ua) && (ua[end] >= '0' && ua[end] <= '9' || ua[end] == '.') {
					end++
				}
				version := strings.TrimRight(ua[i:end], ".")
				if strings.Contains(version, ".") {
					return "Asterisk " + version
				}
			}
		}
	}
	if strings.HasPrefix(ua, "FPBX") {
		if open := strings.Index(ua, "("); open >= 0 {
			if close := strings.Index(ua[open:], ")"); close >= 0 {
				return "FPBX " + ua[open+1:open+close]
			}
		}
	}
	return ua
}
