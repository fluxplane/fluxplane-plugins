package slack

import "testing"

func TestMrkdwnToMarkdown(t *testing.T) {
	cases := map[string]string{
		"*bold* and _italic_ and ~struck~": "**bold** and *italic* and ~~struck~~",
		"see <https://x.test|the docs>":    "see [the docs](https://x.test)",
		"raw <https://x.test>":             "raw https://x.test",
		"hi <@U123|ada> in <#C1|general>":  "hi @ada in #general",
		"hi <@U123> ping <!here>":          "hi @U123 ping @here",
		"a &lt;tag&gt; &amp; more":         "a <tag> & more",
		"keep `*literal*` code":            "keep `*literal*` code",
		"```\n*not bold* <a|b>\n```":       "```\n*not bold* <a|b>\n```",
	}
	for in, want := range cases {
		if got := MrkdwnToMarkdown(in); got != want {
			t.Errorf("MrkdwnToMarkdown(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMarkdownToMrkdwn(t *testing.T) {
	cases := map[string]string{
		"**bold** and *italic* and ~~struck~~": "*bold* and _italic_ and ~struck~",
		"[the docs](https://x.test)":           "<https://x.test|the docs>",
		"a <tag> & more":                       "a &lt;tag&gt; &amp; more",
		"run `make <x>` now":                   "run `make <x>` now",
		"__bold__ too":                         "*bold* too",
	}
	for in, want := range cases {
		if got := MarkdownToMrkdwn(in); got != want {
			t.Errorf("MarkdownToMrkdwn(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTextFormat(t *testing.T) {
	if parseTextFormat("") != textFormatMarkdown || parseTextFormat("MRKDWN") != textFormatMrkdwn || parseTextFormat("both") != textFormatBoth {
		t.Fatal("parseTextFormat mapping wrong")
	}
}
