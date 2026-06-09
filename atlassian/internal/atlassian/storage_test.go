package atlassian

import (
	"strings"
	"testing"
)

func TestStorageToMarkdownRendersCommonBlocks(t *testing.T) {
	storage := `<h2>Deploy</h2>` +
		`<p>Use <strong>caution</strong> with <em>prod</em> and <code>kubectl</code>. See <a href="https://example.com/docs">docs</a>.</p>` +
		`<ul><li>one</li><li>two<ul><li>nested</li></ul></li></ul>` +
		`<ol><li>first</li><li>second</li></ol>` +
		`<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">go</ac:parameter>` +
		`<ac:plain-text-body><![CDATA[fmt.Println("hi")]]></ac:plain-text-body></ac:structured-macro>` +
		`<ac:structured-macro ac:name="info"><ac:rich-text-body><p>Heads up</p></ac:rich-text-body></ac:structured-macro>` +
		`<table><tbody><tr><th>Name</th><th>Value</th></tr><tr><td>a</td><td>1</td></tr></tbody></table>` +
		`<hr/>`

	want := "## Deploy\n\n" +
		"Use **caution** with *prod* and `kubectl`. See [docs](https://example.com/docs).\n\n" +
		"- one\n- two\n  - nested\n\n" +
		"1. first\n2. second\n\n" +
		"```go\nfmt.Println(\"hi\")\n```\n\n" +
		"> **INFO**\n>\n> Heads up\n\n" +
		"| Name | Value |\n| --- | --- |\n| a | 1 |\n\n" +
		"---"

	if got := StorageToMarkdown(storage); got != want {
		t.Fatalf("StorageToMarkdown:\n got: %q\nwant: %q", got, want)
	}
}

func TestStorageToMarkdownRendersConfluenceSpecifics(t *testing.T) {
	storage := `<p>Ping <ac:link><ri:user ri:account-id="acct-1"/></ac:link> about ` +
		`<ac:link><ri:page ri:content-title="Runbook"/></ac:link>&nbsp;today <ac:emoticon ac:name="smile" ac:emoji-fallback="🙂"/></p>` +
		`<ac:task-list><ac:task><ac:task-status>complete</ac:task-status><ac:task-body>done thing</ac:task-body></ac:task>` +
		`<ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body>open thing</ac:task-body></ac:task></ac:task-list>` +
		`<p><ac:image ac:alt="chart"><ri:attachment ri:filename="chart.png"/></ac:image></p>`

	got := StorageToMarkdown(storage)
	want := "Ping @acct-1 about Runbook today 🙂\n\n" +
		"- [x] done thing\n- [ ] open thing\n\n" +
		"![chart](chart.png)"
	if got != want {
		t.Fatalf("StorageToMarkdown:\n got: %q\nwant: %q", got, want)
	}
}

func TestStorageToMarkdownSeparatesBlockLinkCards(t *testing.T) {
	storage := `<ac:layout><ac:layout-section ac:type="two_equal"><ac:layout-cell>` +
		`<ac:link ac:card-appearance="block"><ri:page ri:content-title="Onboarding customers"/><ac:link-body>Onboarding customers</ac:link-body></ac:link>` +
		`<ac:link ac:card-appearance="block"><ri:page ri:content-title="Emergency processes"/><ac:link-body>Emergency processes</ac:link-body></ac:link>` +
		`</ac:layout-cell></ac:layout-section></ac:layout>`

	want := "Onboarding customers\n\nEmergency processes"
	if got := StorageToMarkdown(storage); got != want {
		t.Fatalf("block cards:\n got: %q\nwant: %q", got, want)
	}
}

func TestStorageToMarkdownDegradesGracefully(t *testing.T) {
	if got := StorageToMarkdown(""); got != "" {
		t.Fatalf("empty storage = %q", got)
	}
	if got := StorageToMarkdown("plain text only"); got != "plain text only" {
		t.Fatalf("plain text = %q", got)
	}
}

func TestMarkdownToStorageProducesStorageXHTML(t *testing.T) {
	markdown := "## Deploy\n\nUse **caution** with `kubectl` — see [docs](https://example.com/docs).\n\n" +
		"- one\n- two\n\n```go\nfmt.Println(\"hi\")\n```\n\n~~dropped~~"

	got := MarkdownToStorage(markdown)
	for _, want := range []string{
		"<h2>Deploy</h2>",
		"<strong>caution</strong>",
		"<code>kubectl</code>",
		`<a href="https://example.com/docs">docs</a>`,
		"<ul>",
		"<li>one</li>",
		`<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">go</ac:parameter><ac:plain-text-body><![CDATA[fmt.Println("hi")]]></ac:plain-text-body></ac:structured-macro>`,
		"<s>dropped</s>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("MarkdownToStorage missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<pre>") || strings.Contains(got, "<del>") {
		t.Fatalf("MarkdownToStorage left raw HTML constructs:\n%s", got)
	}
}

func TestMarkdownToStorageEscapesCDATAEnd(t *testing.T) {
	got := MarkdownToStorage("```\ndata]]>more\n```")
	if !strings.Contains(got, "]]]]><![CDATA[>") {
		t.Fatalf("CDATA end sequence not escaped:\n%s", got)
	}
}

func TestStorageRoundTripKeepsStructure(t *testing.T) {
	markdown := "# Title\n\nSome **bold** and *italic* text.\n\n- a\n- b"
	storage := MarkdownToStorage(markdown)
	back := StorageToMarkdown(storage)
	if back != markdown {
		t.Fatalf("round trip:\n got: %q\nwant: %q", back, markdown)
	}
}
