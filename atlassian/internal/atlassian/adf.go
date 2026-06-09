// Package atlassian holds helpers shared by the Jira and Confluence plugins.
package atlassian

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/codewandler/md2adf"
)

// MarkdownToADF converts Markdown into an Atlassian Document Format document,
// repairing mark combinations that Jira's ADF validator rejects. The returned
// JSON is ready to embed directly in a Jira rich-text field (description,
// comment body).
//
// The underlying md2adf converter emits a single text node carrying both the
// code mark and any enclosing formatting marks when inline code is nested inside
// bold/italic/strikethrough (e.g. "**bold with `code` inside**"). ADF forbids
// the code mark alongside any mark other than link, so Jira rejects the whole
// payload with 400 INVALID_INPUT. We post-process the tree to drop the
// incompatible marks, keeping the document valid.
func MarkdownToADF(markdown string) json.RawMessage {
	doc := md2adf.Convert(markdown)
	data, err := json.Marshal(doc)
	if err != nil {
		return nil
	}
	var tree any
	if err := json.Unmarshal(data, &tree); err != nil {
		return data
	}
	sanitizeADFMarks(tree)
	out, err := json.Marshal(tree)
	if err != nil {
		return data
	}
	return out
}

// sanitizeADFMarks walks a decoded ADF tree and repairs every text node so that
// the code mark never coexists with a mark ADF disallows.
func sanitizeADFMarks(v any) {
	switch node := v.(type) {
	case map[string]any:
		if node["type"] == "text" {
			pruneCodeMarks(node)
		}
		sanitizeADFMarks(node["content"])
	case []any:
		for _, item := range node {
			sanitizeADFMarks(item)
		}
	}
}

// pruneCodeMarks drops every mark except code and link from a text node that
// carries the code mark. ADF only permits code to combine with link.
func pruneCodeMarks(node map[string]any) {
	marks, ok := node["marks"].([]any)
	if !ok || len(marks) == 0 {
		return
	}
	hasCode := false
	for _, mark := range marks {
		if mm, ok := mark.(map[string]any); ok && mm["type"] == "code" {
			hasCode = true
			break
		}
	}
	if !hasCode {
		return
	}
	kept := make([]any, 0, len(marks))
	for _, mark := range marks {
		if mm, ok := mark.(map[string]any); ok {
			switch mm["type"] {
			case "code", "link":
				kept = append(kept, mark)
			}
		}
	}
	node["marks"] = kept
}

// adfNode is a generic Atlassian Document Format node. ADF documents are JSON
// trees whose nodes carry a type, optional child content, leaf text, inline
// marks, and type-specific attributes.
type adfNode struct {
	Type    string         `json:"type"`
	Text    string         `json:"text"`
	Content []adfNode      `json:"content"`
	Marks   []adfMark      `json:"marks"`
	Attrs   map[string]any `json:"attrs"`
}

type adfMark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs"`
}

// ADFToMarkdown renders an Atlassian Document Format value into readable
// Markdown so that callers never have to interpret the raw ADF tree.
//
// It handles the common block and inline constructs Jira emits (paragraphs,
// headings, lists, code blocks, blockquotes, panels, rules, tables, links,
// mentions, emoji, and inline cards) and degrades gracefully on unknown nodes
// by rendering their text content. Empty, null, or already-flattened plain
// string values are returned as-is.
func ADFToMarkdown(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// Some Jira fields (REST v2, or values already flattened to text) come
	// back as a plain JSON string rather than an ADF document.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return strings.TrimSpace(s)
		}
	}
	var node adfNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	blocks := node.Content
	if node.Type != "doc" {
		blocks = []adfNode{node}
	}
	return strings.TrimRight(renderBlocks(blocks, 0), "\n")
}

func renderBlocks(nodes []adfNode, depth int) string {
	var parts []string
	for _, n := range nodes {
		rendered := renderBlock(n, depth)
		if strings.TrimSpace(rendered) != "" {
			parts = append(parts, rendered)
		}
	}
	return strings.Join(parts, "\n\n")
}

func renderBlock(n adfNode, depth int) string {
	switch n.Type {
	case "paragraph", "":
		return renderInline(n.Content)
	case "heading":
		level := attrInt(n.Attrs, "level", 1)
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		return strings.Repeat("#", level) + " " + renderInline(n.Content)
	case "bulletList":
		return renderList(n.Content, depth, false)
	case "orderedList":
		return renderList(n.Content, depth, true)
	case "codeBlock":
		return "```" + attrString(n.Attrs, "language") + "\n" + collectText(n.Content) + "\n```"
	case "blockquote":
		return prefixLines(renderBlocks(n.Content, depth), "> ")
	case "panel":
		inner := renderBlocks(n.Content, depth)
		if label := strings.ToUpper(attrString(n.Attrs, "panelType")); label != "" {
			inner = "**" + label + "**\n\n" + inner
		}
		return prefixLines(inner, "> ")
	case "rule":
		return "---"
	case "table":
		return renderTable(n)
	case "mediaSingle", "mediaGroup":
		return renderInline(n.Content)
	default:
		if len(n.Content) > 0 {
			return renderBlocks(n.Content, depth)
		}
		return renderInline([]adfNode{n})
	}
}

func renderList(items []adfNode, depth int, ordered bool) string {
	var lines []string
	number := 0
	for _, item := range items {
		if item.Type != "listItem" {
			continue
		}
		number++
		marker := "- "
		if ordered {
			marker = fmt.Sprintf("%d. ", number)
		}
		indent := strings.Repeat(" ", len(marker))
		content := renderBlocks(item.Content, depth+1)
		for j, line := range strings.Split(content, "\n") {
			switch {
			case j == 0:
				lines = append(lines, marker+line)
			case line == "":
				lines = append(lines, "")
			default:
				lines = append(lines, indent+line)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func renderInline(nodes []adfNode) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.Type {
		case "text":
			b.WriteString(applyMarks(n.Text, n.Marks))
		case "hardBreak":
			b.WriteString("\n")
		case "mention":
			name := strings.TrimSpace(attrString(n.Attrs, "text"))
			if name == "" {
				name = "@" + attrString(n.Attrs, "id")
			}
			if !strings.HasPrefix(name, "@") {
				name = "@" + name
			}
			b.WriteString(name)
		case "emoji":
			if text := attrString(n.Attrs, "text"); text != "" {
				b.WriteString(text)
			} else {
				b.WriteString(attrString(n.Attrs, "shortName"))
			}
		case "inlineCard", "blockCard":
			b.WriteString(attrString(n.Attrs, "url"))
		case "date":
			b.WriteString(attrString(n.Attrs, "timestamp"))
		case "status":
			if text := attrString(n.Attrs, "text"); text != "" {
				b.WriteString("[" + text + "]")
			}
		default:
			if len(n.Content) > 0 {
				b.WriteString(renderInline(n.Content))
			} else if n.Text != "" {
				b.WriteString(n.Text)
			}
		}
	}
	return b.String()
}

func applyMarks(text string, marks []adfMark) string {
	link := ""
	has := map[string]bool{}
	for _, m := range marks {
		has[m.Type] = true
		if m.Type == "link" {
			link = attrString(m.Attrs, "href")
		}
	}
	if has["code"] {
		text = "`" + text + "`"
	}
	if has["strike"] {
		text = "~~" + text + "~~"
	}
	if has["em"] {
		text = "*" + text + "*"
	}
	if has["strong"] {
		text = "**" + text + "**"
	}
	if link != "" {
		text = "[" + text + "](" + link + ")"
	}
	return text
}

func renderTable(n adfNode) string {
	var rows [][]string
	for _, row := range n.Content {
		if row.Type != "tableRow" {
			continue
		}
		var cells []string
		for _, cell := range row.Content {
			text := strings.TrimSpace(strings.ReplaceAll(renderBlocks(cell.Content, 0), "\n", " "))
			cells = append(cells, strings.ReplaceAll(text, "|", "\\|"))
		}
		rows = append(rows, cells)
	}
	if len(rows) == 0 {
		return ""
	}
	width := 0
	for _, r := range rows {
		if len(r) > width {
			width = len(r)
		}
	}
	pad := func(r []string) []string {
		for len(r) < width {
			r = append(r, "")
		}
		return r
	}
	var b strings.Builder
	b.WriteString("| " + strings.Join(pad(rows[0]), " | ") + " |\n")
	seps := make([]string, width)
	for i := range seps {
		seps[i] = "---"
	}
	b.WriteString("| " + strings.Join(seps, " | ") + " |")
	for _, r := range rows[1:] {
		b.WriteString("\n| " + strings.Join(pad(r), " | ") + " |")
	}
	return b.String()
}

func collectText(nodes []adfNode) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(n.Text)
		if len(n.Content) > 0 {
			b.WriteString(collectText(n.Content))
		}
	}
	return b.String()
}

func prefixLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = strings.TrimRight(prefix, " ")
		} else {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

func attrString(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	value, ok := attrs[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func attrInt(attrs map[string]any, key string, fallback int) int {
	if attrs == nil {
		return fallback
	}
	switch n := attrs[key].(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		if v, err := n.Int64(); err == nil {
			return int(v)
		}
	}
	return fallback
}
