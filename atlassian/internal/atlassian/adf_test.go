package atlassian

import (
	"encoding/json"
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

func TestADFToMarkdownInlineMarks(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","text":"a "},
		{"type":"text","text":"bold","marks":[{"type":"strong"}]},
		{"type":"text","text":" "},
		{"type":"text","text":"italic","marks":[{"type":"em"}]},
		{"type":"text","text":" "},
		{"type":"text","text":"code","marks":[{"type":"code"}]},
		{"type":"text","text":" "},
		{"type":"text","text":"link","marks":[{"type":"link","attrs":{"href":"https://x.test"}}]}
	]}]}`)
	got := ADFToMarkdown(raw)
	want := "a **bold** *italic* `code` [link](https://x.test)"
	if got != want {
		t.Fatalf("inline = %q, want %q", got, want)
	}
}

func TestADFToMarkdownBlocks(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[
		{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Title"}]},
		{"type":"bulletList","content":[
			{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]},
			{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"two"}]}]}
		]},
		{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"x := 1"}]},
		{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"quoted"}]}]},
		{"type":"rule"}
	]}`)
	got := ADFToMarkdown(raw)
	want := "## Title\n\n- one\n- two\n\n```go\nx := 1\n```\n\n> quoted\n\n---"
	if got != want {
		t.Fatalf("blocks =\n%q\nwant\n%q", got, want)
	}
}

func TestADFToMarkdownMentionsAndCards(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"mention","attrs":{"text":"@Ada","id":"acct-1"}},
		{"type":"text","text":" see "},
		{"type":"inlineCard","attrs":{"url":"https://card.test/1"}}
	]}]}`)
	got := ADFToMarkdown(raw)
	want := "@Ada see https://card.test/1"
	if got != want {
		t.Fatalf("mentions/cards = %q, want %q", got, want)
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

// TestADFRoundTrip confirms Markdown survives md2adf.Convert -> ADFToMarkdown
// for the common constructs both halves support.
func TestADFRoundTrip(t *testing.T) {
	markdown := "# Heading\n\nSome **bold** and `code`.\n\n- one\n- two"
	doc := md2adf.Convert(markdown)
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := ADFToMarkdown(raw)
	want := "# Heading\n\nSome **bold** and `code`.\n\n- one\n- two"
	if got != want {
		t.Fatalf("round trip =\n%q\nwant\n%q", got, want)
	}
}
