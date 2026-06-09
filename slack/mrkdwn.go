package slack

import (
	"regexp"
	"strings"
)

// Slack messages use "mrkdwn", which differs from standard Markdown an agent
// expects: bold is *single asterisks*, italic is _underscores_, strike is
// ~single tildes~, links are <url|text>, mentions are <@U…>/<#C…|name>, and
// &,<,> are HTML-escaped. These converters translate both ways so the agent
// reads and writes ordinary Markdown by default while Slack still renders it.

type textFormat string

const (
	textFormatMarkdown textFormat = "markdown"
	textFormatMrkdwn   textFormat = "mrkdwn"
	textFormatBoth     textFormat = "both"
)

func parseTextFormat(value string) textFormat {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(textFormatMrkdwn):
		return textFormatMrkdwn
	case string(textFormatBoth):
		return textFormatBoth
	default:
		return textFormatMarkdown
	}
}

var (
	mrkdwnLinkRE    = regexp.MustCompile(`<(https?://[^|>]+)\|([^>]+)>`)
	mrkdwnBareURLRE = regexp.MustCompile(`<(https?://[^|>]+)>`)
	mrkdwnUserRE    = regexp.MustCompile(`<@([UW][A-Z0-9]+)(\|([^>]+))?>`)
	mrkdwnChannelRE = regexp.MustCompile(`<#(C[A-Z0-9]+)(\|([^>]+))?>`)
	mrkdwnSubteamRE = regexp.MustCompile(`<!subteam\^[A-Z0-9]+(\|([^>]+))?>`)
	mrkdwnSpecialRE = regexp.MustCompile(`<!(here|channel|everyone)>`)
)

// MrkdwnToMarkdown converts Slack mrkdwn into standard Markdown for agent
// consumption: links/mentions become readable, HTML entities are decoded, and
// bold/italic/strike are normalized. Inline and fenced code are preserved.
func MrkdwnToMarkdown(text string) string {
	if text == "" {
		return ""
	}
	text = mrkdwnLinkRE.ReplaceAllString(text, "[$2]($1)")
	text = mrkdwnBareURLRE.ReplaceAllString(text, "$1")
	text = mrkdwnUserRE.ReplaceAllStringFunc(text, func(m string) string {
		sub := mrkdwnUserRE.FindStringSubmatch(m)
		if sub[3] != "" {
			return "@" + sub[3]
		}
		return "@" + sub[1]
	})
	text = mrkdwnChannelRE.ReplaceAllStringFunc(text, func(m string) string {
		sub := mrkdwnChannelRE.FindStringSubmatch(m)
		if sub[3] != "" {
			return "#" + sub[3]
		}
		return "#" + sub[1]
	})
	text = mrkdwnSubteamRE.ReplaceAllStringFunc(text, func(m string) string {
		sub := mrkdwnSubteamRE.FindStringSubmatch(m)
		if sub[2] != "" {
			return "@" + sub[2]
		}
		return "@group"
	})
	text = mrkdwnSpecialRE.ReplaceAllString(text, "@$1")

	text = mapOutsideCode(text, func(seg string) string {
		seg = normalizeMrkdwnEmphasis(seg)
		return seg
	})

	// Decode HTML entities last so a literal "&lt;" in code is left untouched
	// only inside code (handled by mapOutsideCode); outside code we decode.
	text = mapOutsideCode(text, func(seg string) string {
		seg = strings.ReplaceAll(seg, "&lt;", "<")
		seg = strings.ReplaceAll(seg, "&gt;", ">")
		seg = strings.ReplaceAll(seg, "&amp;", "&")
		return seg
	})
	return text
}

var (
	mrkdwnBoldRE   = regexp.MustCompile(`\*([^*\n]+)\*`)
	mrkdwnStrikeRE = regexp.MustCompile(`~([^~\n]+)~`)
	mrkdwnItalicRE = regexp.MustCompile(`\b_([^_\n]+)_\b`)
)

func normalizeMrkdwnEmphasis(seg string) string {
	seg = mrkdwnBoldRE.ReplaceAllString(seg, "**$1**")
	seg = mrkdwnStrikeRE.ReplaceAllString(seg, "~~$1~~")
	seg = mrkdwnItalicRE.ReplaceAllString(seg, "*$1*")
	return seg
}

var (
	mdBoldRE   = regexp.MustCompile(`\*\*([^*\n]+)\*\*|__([^_\n]+)__`)
	mdStrikeRE = regexp.MustCompile(`~~([^~\n]+)~~`)
	mdLinkRE   = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)]+)\)`)
)

// MarkdownToMrkdwn converts standard Markdown (what an agent writes) into Slack
// mrkdwn so it renders correctly: **bold**→*bold*, ~~s~~→~s~, [t](u)→<u|t>, and
// &,<,> escaped. Inline/fenced code is preserved verbatim.
func MarkdownToMrkdwn(text string) string {
	if text == "" {
		return ""
	}
	return mapOutsideCode(text, func(seg string) string {
		seg = strings.ReplaceAll(seg, "&", "&amp;")
		seg = strings.ReplaceAll(seg, "<", "&lt;")
		seg = strings.ReplaceAll(seg, ">", "&gt;")
		// Links first (they introduce the only literal <…> we want to keep).
		seg = mdLinkRE.ReplaceAllString(seg, "<$2|$1>")
		// Protect bold as a sentinel so the italic pass (single *) doesn't
		// re-match the *bold* we'd otherwise produce.
		seg = mdBoldRE.ReplaceAllStringFunc(seg, func(m string) string {
			sub := mdBoldRE.FindStringSubmatch(m)
			inner := sub[1]
			if inner == "" {
				inner = sub[2]
			}
			return "\x00" + inner + "\x00"
		})
		seg = mdStrikeRE.ReplaceAllString(seg, "~$1~")
		seg = mdItalicStarRE.ReplaceAllString(seg, "_${1}_")
		// Restore bold as Slack single-asterisk bold.
		seg = strings.ReplaceAll(seg, "\x00", "*")
		return seg
	})
}

var mdItalicStarRE = regexp.MustCompile(`\*([^*\n]+)\*`)

// mapOutsideCode applies fn to the parts of text that are NOT inside inline
// (`…`) or fenced (```…```) code spans, leaving code untouched.
func mapOutsideCode(text string, fn func(string) string) string {
	var b strings.Builder
	i := 0
	for i < len(text) {
		if strings.HasPrefix(text[i:], "```") {
			end := strings.Index(text[i+3:], "```")
			if end >= 0 {
				b.WriteString(text[i : i+3+end+3])
				i += 3 + end + 3
				continue
			}
		}
		if text[i] == '`' {
			end := strings.IndexByte(text[i+1:], '`')
			if end >= 0 {
				b.WriteString(text[i : i+1+end+1])
				i += 1 + end + 1
				continue
			}
		}
		// Accumulate a run of non-code text up to the next backtick.
		next := strings.IndexByte(text[i:], '`')
		if next < 0 {
			b.WriteString(fn(text[i:]))
			break
		}
		b.WriteString(fn(text[i : i+next]))
		i += next
	}
	return b.String()
}
