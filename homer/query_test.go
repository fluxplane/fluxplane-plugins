package homer

import (
	"strings"
	"testing"
)

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "simple from_user",
			input: "from_user = '999%'",
			want:  "data_header.from_user LIKE '999%'",
		},
		{
			name:  "alias ua",
			input: "ua = 'Asterisk%'",
			want:  "data_header.user_agent LIKE 'Asterisk%'",
		},
		{
			name:  "full name user_agent",
			input: "user_agent = 'FPBX%'",
			want:  "data_header.user_agent LIKE 'FPBX%'",
		},
		{
			name:  "AND",
			input: "from_user = '999%' AND to_user = '1234'",
			want:  "data_header.from_user LIKE '999%' AND data_header.to_user = '1234'",
		},
		{
			name:  "OR",
			input: "from_user = '123' OR to_user = '123'",
			want:  "data_header.from_user = '123' OR data_header.to_user = '123'",
		},
		{
			name:  "mixed AND with parenthesized OR",
			input: "from_user = '999%' AND (to_user = '123' OR to_user = '456')",
			want:  "data_header.from_user LIKE '999%' AND (data_header.to_user = '123' OR data_header.to_user = '456')",
		},
		{
			name:  "top-level field method",
			input: "method = 'INVITE'",
			want:  "method = 'INVITE'",
		},
		{
			name:  "top-level field status number",
			input: "status = 200",
			want:  "status = 200",
		},
		{
			name:  "not-equal operator",
			input: "status != 200",
			want:  "status != 200",
		},
		{
			name:  "call_id alias to sid",
			input: "call_id = 'abc123@host'",
			want:  "sid = 'abc123@host'",
		},
		{
			name:  "sid direct",
			input: "sid = 'abc123@host'",
			want:  "sid = 'abc123@host'",
		},
		{
			name:  "complex nested",
			input: "(from_user = '100' OR from_user = '200') AND method = 'INVITE'",
			want:  "(data_header.from_user = '100' OR data_header.from_user = '200') AND method = 'INVITE'",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  "",
		},
		{
			name:    "unknown field",
			input:   "bogus = '123'",
			wantErr: "unknown field",
		},
		{
			name:    "missing value",
			input:   "from_user =",
			wantErr: "expected value",
		},
		{
			name:    "missing operator",
			input:   "from_user '123'",
			wantErr: "expected operator",
		},
		{
			name:    "unterminated string",
			input:   "from_user = '123",
			wantErr: "unterminated string",
		},
		{
			name:    "missing closing paren",
			input:   "(from_user = '123'",
			wantErr: "missing closing parenthesis",
		},
		{
			name:    "unexpected token after expression",
			input:   "from_user = '123' from_user = '456'",
			wantErr: "unexpected token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseQuery(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseQuery(%q)\n  got:  %q\n  want: %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNumberAlternativesCoverPrefixVariants(t *testing.T) {
	got := NumberAlternatives("data_header.from_user", "+49301234567")
	want := []string{
		"data_header.from_user = '49301234567'",
		"data_header.from_user = '+49301234567'",
		"data_header.from_user = '0049301234567'",
	}
	if len(got) != len(want) {
		t.Fatalf("alternatives = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("alternatives[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// 00-prefixed input normalizes to the same canonical variants.
	if normalized := NumberAlternatives("f", "0049301234567"); normalized[0] != "f = '49301234567'" {
		t.Fatalf("00-input alternatives = %#v", normalized)
	}
}

func TestNumberContainsAlternative(t *testing.T) {
	got := NumberContainsAlternative("data_header.to_user", "+4930123")
	if len(got) != 1 || got[0] != "data_header.to_user LIKE '%4930123%'" {
		t.Fatalf("contains alternative = %#v", got)
	}
}

func TestUserPredicateUpgradesWildcardsToLike(t *testing.T) {
	if got := userPredicate("data_header.from_user", "%1234567%"); got != "data_header.from_user LIKE '%1234567%'" {
		t.Fatalf("wildcard predicate = %q", got)
	}
	if got := userPredicate("data_header.from_user", "1234567"); got != "data_header.from_user = '1234567'" {
		t.Fatalf("plain predicate = %q", got)
	}
}

func TestExtractSIPHeaders(t *testing.T) {
	raw := "INVITE sip:123@example.com SIP/2.0\r\nFrom: <sip:a@x>\r\nX-CID: abc-123\r\nx-tenant: lyse\r\n\r\nbody X-CID: not-this"
	got := extractSIPHeaders(raw, []string{"X-CID", "X-Tenant"})
	if got["X-CID"] != "abc-123" || got["X-Tenant"] != "lyse" {
		t.Fatalf("headers = %#v", got)
	}
	if extractSIPHeaders(raw, nil) != nil {
		t.Fatalf("no requested headers should yield nil")
	}
}

func TestRenderLadderSVG(t *testing.T) {
	events := []FlowEvent{
		{OffsetMS: 0, CallID: "leg-a", Src: "10.0.0.1:5060", Dst: "10.0.0.2:5060", Method: "INVITE", SDP: "PCMA :17818"},
		{OffsetMS: 20, CallID: "leg-a", Src: "10.0.0.2:5060", Dst: "10.0.0.1:5060", Method: "180"},
		{OffsetMS: 90, CallID: "leg-b", Src: "10.0.0.2:5060", Dst: "10.0.0.3:5060", Method: "486"},
		{OffsetMS: 120, CallID: "leg-a", Src: "10.0.0.1:5060", Dst: "10.0.0.2:5060", Method: "BYE"},
	}
	svg := string(RenderLadderSVG(events))
	if !strings.HasPrefix(svg, "<svg ") || !strings.HasSuffix(svg, "</svg>") {
		t.Fatalf("not an svg document: %.80s", svg)
	}
	// Three lifelines, one per host.
	if got := strings.Count(svg, "stroke-dasharray"); got != 3 {
		t.Fatalf("lifelines = %d, want 3", got)
	}
	// Failure highlighting on 486 and BYE; success-ish gray on 180.
	if !strings.Contains(svg, ">L2: 486 +90ms<") || !strings.Contains(svg, "#cc2929") {
		t.Fatalf("missing highlighted failure: %s", svg)
	}
	// Multi-leg flows carry per-leg prefixes; SDP annotation rides the label.
	if !strings.Contains(svg, "L1: INVITE +0ms") || !strings.Contains(svg, "[PCMA :17818]") {
		t.Fatalf("missing labels: %s", svg)
	}
}
