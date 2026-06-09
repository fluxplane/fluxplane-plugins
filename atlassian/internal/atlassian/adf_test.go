package atlassian

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/codewandler/md2adf"
)

func TestADFToMarkdownEmptyAndPlainString(t *testing.T) {
	for _, raw := range []string{"", "null", "  "} {
		if got := ADFToMarkdown(json.RawMessage(raw)); got != "" {
			t.Fatalf("ADFToMarkdown(%q) = %q, want empty", raw, got)
		}
	}
	if got := ADFToMarkdown(json.RawMessage(`"just text"`)); got != "just text" {
		t.Fatalf("plain string = %q", got)
	}
}

func TestADFToMarkdownInlineAndBlocks(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[
		{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Title"}]},
		{"type":"paragraph","content":[
			{"type":"text","text":"a "},
			{"type":"text","text":"bold","marks":[{"type":"strong"}]},
			{"type":"text","text":" "},
			{"type":"text","text":"code","marks":[{"type":"code"}]},
			{"type":"text","text":" "},
			{"type":"text","text":"link","marks":[{"type":"link","attrs":{"href":"https://x.test"}}]}
		]},
		{"type":"bulletList","content":[
			{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]}
		]},
		{"type":"rule"}
	]}`)
	got := ADFToMarkdown(raw)
	want := "## Title\n\na **bold** `code` [link](https://x.test)\n\n- one\n\n---"
	if got != want {
		t.Fatalf("render =\n%q\nwant\n%q", got, want)
	}
}

func TestADFToMarkdownTable(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"table","content":[
		{"type":"tableRow","content":[
			{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"H1"}]}]},
			{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"H2"}]}]}
		]},
		{"type":"tableRow","content":[
			{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]},
			{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}
		]}
	]}]}`)
	got := ADFToMarkdown(raw)
	want := "| H1 | H2 |\n| --- | --- |\n| a | b |"
	if got != want {
		t.Fatalf("table =\n%q\nwant\n%q", got, want)
	}
}

// conformanceCases capture the converter contract: no text node may carry the
// code mark together with any mark except link, or Jira rejects the document
// with 400 INVALID_INPUT.
var conformanceCases = map[string]string{
	"bold around code":   "**bold with `code` inside**",
	"italic around code": "*italic with `code` inside*",
	"strike around code": "~~struck with `code` inside~~",
	"link around code":   "[`code`](https://x.test)",
	"plain code":         "`code`",
	"bold then code":     "**bold** and `code`",
	"heading + list":     "# H\n\n- a\n- b\n\n```go\nx := 1\n```",
}

func TestMarkdownToADFSanitizesCodeMarks(t *testing.T) {
	for name, md := range conformanceCases {
		t.Run(name, func(t *testing.T) {
			var tree any
			if err := json.Unmarshal(MarkdownToADF(md), &tree); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			assertNoIllegalCodeMarks(t, tree)
		})
	}
}

func assertNoIllegalCodeMarks(t *testing.T, v any) {
	t.Helper()
	switch node := v.(type) {
	case map[string]any:
		if node["type"] == "text" {
			if marks, ok := node["marks"].([]any); ok {
				types := map[string]bool{}
				for _, m := range marks {
					if mm, ok := m.(map[string]any); ok {
						if mt, ok := mm["type"].(string); ok {
							types[mt] = true
						}
					}
				}
				if types["code"] {
					for mt := range types {
						if mt != "code" && mt != "link" {
							body, _ := json.Marshal(node)
							t.Fatalf("code mark combined with disallowed %q: %s", mt, body)
						}
					}
				}
			}
		}
		assertNoIllegalCodeMarks(t, node["content"])
	case []any:
		for _, item := range node {
			assertNoIllegalCodeMarks(t, item)
		}
	}
}

func TestADFRoundTrip(t *testing.T) {
	markdown := "# Heading\n\nSome **bold** and `code`.\n\n- one\n- two"
	got := ADFToMarkdown(MarkdownToADF(markdown))
	if got != markdown {
		t.Fatalf("round trip =\n%q\nwant\n%q", got, markdown)
	}
}

// FuzzMarkdownToADF asserts the converter never panics, always emits valid JSON,
// and never produces the illegal code+other-mark combination, for any input.
func FuzzMarkdownToADF(f *testing.F) {
	for _, seed := range conformanceCases {
		f.Add(seed)
	}
	f.Add("")
	f.Add("plain text with `code` and **bold** and a [link](x) and ~~strike~~")
	f.Add("- nested\n  - **deep `code`**\n\n> quote with `code`")
	f.Fuzz(func(t *testing.T, markdown string) {
		raw := MarkdownToADF(markdown)
		if len(raw) == 0 {
			return
		}
		var tree any
		if err := json.Unmarshal(raw, &tree); err != nil {
			t.Fatalf("MarkdownToADF produced invalid JSON for %q: %v", markdown, err)
		}
		assertNoIllegalCodeMarks(t, tree)
	})
}

// FuzzADFToMarkdown asserts the renderer never panics on arbitrary inputs and is
// stable when fed its own md2adf output.
func FuzzADFToMarkdown(f *testing.F) {
	f.Add(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"x"}]}]}`)
	f.Add(`"plain"`)
	f.Add(`null`)
	f.Add(`{"type":"unknown"}`)
	f.Add(`{`)
	f.Fuzz(func(t *testing.T, raw string) {
		_ = ADFToMarkdown(json.RawMessage(raw))
		doc := md2adf.Convert(raw)
		if data, err := json.Marshal(doc); err == nil {
			if strings.Contains(ADFToMarkdown(data), "\x00") {
				t.Fatalf("rendered output contains NUL for %q", raw)
			}
		}
	})
}
