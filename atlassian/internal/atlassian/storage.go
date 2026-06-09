package atlassian

import (
	"bytes"
	"encoding/xml"
	stdhtml "html"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// StorageToMarkdown renders Confluence storage-format XHTML into readable
// Markdown so that callers never have to interpret the raw XML.
//
// It handles the common block and inline constructs Confluence emits
// (paragraphs, headings, lists, tables, blockquotes, rules, links, text
// effects, code/info/note/warning/tip/panel/expand macros, task lists, page
// and user links, images, emoticons, and layouts) and degrades gracefully on
// unknown elements by rendering their text content.
func StorageToMarkdown(storage string) string {
	storage = strings.TrimSpace(storage)
	if storage == "" {
		return ""
	}
	nodes, err := parseStorage(storage)
	if err != nil {
		return storage
	}
	return strings.TrimSpace(renderStorageBlocks(nodes))
}

// MarkdownToStorage converts Markdown into Confluence storage-format XHTML
// ready to embed in a page or comment body. Fenced code blocks become
// Confluence code macros (with language), GFM tables and strikethrough are
// supported, and raw HTML in the source is omitted (use a storage body
// directly when hand-authored macros are needed).
func MarkdownToStorage(markdown string) string {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.Strikethrough),
		goldmark.WithRendererOptions(html.WithXHTML()),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "<p>" + stdhtml.EscapeString(markdown) + "</p>"
	}
	out := buf.String()
	out = storageCodeBlockPattern.ReplaceAllStringFunc(out, func(match string) string {
		groups := storageCodeBlockPattern.FindStringSubmatch(match)
		if groups == nil {
			return match
		}
		return storageCodeMacro(groups[1], stdhtml.UnescapeString(groups[2]))
	})
	out = strings.ReplaceAll(out, "<del>", "<s>")
	out = strings.ReplaceAll(out, "</del>", "</s>")
	return strings.TrimSpace(out)
}

var storageCodeBlockPattern = regexp.MustCompile(`(?s)<pre><code(?: class="language-([^"]*)")?>(.*?)</code></pre>`)

func storageCodeMacro(language, code string) string {
	code = strings.TrimRight(code, "\n")
	// CDATA cannot contain "]]>" — split the sequence across two sections.
	code = strings.ReplaceAll(code, "]]>", "]]]]><![CDATA[>")
	var b strings.Builder
	b.WriteString(`<ac:structured-macro ac:name="code">`)
	if language = strings.TrimSpace(language); language != "" {
		b.WriteString(`<ac:parameter ac:name="language">` + stdhtml.EscapeString(language) + `</ac:parameter>`)
	}
	b.WriteString(`<ac:plain-text-body><![CDATA[` + code + `]]></ac:plain-text-body></ac:structured-macro>`)
	return b.String()
}

// storageNode is one node of a parsed storage-format fragment: either an
// element (name + attrs + kids) or a text node (name == "", text set).
type storageNode struct {
	name  string
	text  string
	attrs map[string]string
	kids  []*storageNode
}

// parseStorage tokenizes a storage-format fragment. Confluence fragments use
// undeclared `ac:`/`ri:` namespace prefixes and HTML entities, so the decoder
// runs in non-strict mode with the HTML entity table and auto-closing rules.
//
// Go's decoder mismatches end tags when a self-closing prefixed element nests
// inside another prefixed element, so element-name prefixes are neutralized
// (`ac:link` → `ac.link`, outside CDATA) before decoding and restored by
// storageName. Dots never appear in real storage element names.
func parseStorage(storage string) ([]*storageNode, error) {
	decoder := xml.NewDecoder(strings.NewReader(neutralizeStoragePrefixes(storage)))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	root := &storageNode{}
	stack := []*storageNode{root}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			node := &storageNode{name: storageName(t.Name), attrs: map[string]string{}}
			for _, attr := range t.Attr {
				node.attrs[storageName(attr.Name)] = attr.Value
			}
			parent := stack[len(stack)-1]
			parent.kids = append(parent.kids, node)
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			parent := stack[len(stack)-1]
			parent.kids = append(parent.kids, &storageNode{text: string(t)})
		}
	}
	return root.kids, nil
}

// storageName joins an undeclared namespace prefix back onto the local name
// so dispatch can match "ac:structured-macro", "ri:page", etc. — including
// element prefixes neutralized to dots by neutralizeStoragePrefixes.
func storageName(name xml.Name) string {
	if name.Space != "" {
		return name.Space + ":" + name.Local
	}
	if i := strings.Index(name.Local, "."); i > 0 {
		return name.Local[:i] + ":" + name.Local[i+1:]
	}
	return name.Local
}

var storageElementPrefixPattern = regexp.MustCompile(`<(/?)([A-Za-z][\w-]*):`)
var storageCDATAPattern = regexp.MustCompile(`(?s)<!\[CDATA\[.*?\]\]>`)

// neutralizeStoragePrefixes rewrites element-name namespace prefixes to dotted
// names (`<ac:link>` → `<ac.link>`) everywhere outside CDATA sections, keeping
// attribute prefixes untouched.
func neutralizeStoragePrefixes(storage string) string {
	var b strings.Builder
	last := 0
	for _, loc := range storageCDATAPattern.FindAllStringIndex(storage, -1) {
		b.WriteString(storageElementPrefixPattern.ReplaceAllString(storage[last:loc[0]], "<${1}${2}."))
		b.WriteString(storage[loc[0]:loc[1]])
		last = loc[1]
	}
	b.WriteString(storageElementPrefixPattern.ReplaceAllString(storage[last:], "<${1}${2}."))
	return b.String()
}

func (n *storageNode) attr(key string) string {
	if n == nil || n.attrs == nil {
		return ""
	}
	return strings.TrimSpace(n.attrs[key])
}

// find returns the first direct or nested child element with the given name.
func (n *storageNode) find(name string) *storageNode {
	for _, kid := range n.kids {
		if kid.name == name {
			return kid
		}
		if found := kid.find(name); found != nil {
			return found
		}
	}
	return nil
}

func (n *storageNode) collectText() string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	if n.name == "" {
		b.WriteString(n.text)
	}
	for _, kid := range n.kids {
		b.WriteString(kid.collectText())
	}
	return b.String()
}

// macroParameter returns the value of <ac:parameter ac:name="key"> within a
// structured macro.
func (n *storageNode) macroParameter(key string) string {
	for _, kid := range n.kids {
		if kid.name == "ac:parameter" && kid.attr("ac:name") == key {
			return strings.TrimSpace(kid.collectText())
		}
	}
	return ""
}

var storageBlockElements = map[string]bool{
	"p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "table": true, "blockquote": true, "pre": true, "hr": true,
	"div": true, "section": true,
	"ac:structured-macro": true, "ac:task-list": true,
	"ac:layout": true, "ac:layout-section": true, "ac:layout-cell": true,
	"ac:rich-text-body": true,
}

// renderStorageBlocks renders a sequence of sibling nodes, grouping
// consecutive inline content into paragraphs and joining blocks with blank
// lines.
func renderStorageBlocks(nodes []*storageNode) string {
	var parts []string
	var inlineRun []*storageNode
	flush := func() {
		if len(inlineRun) == 0 {
			return
		}
		if text := strings.TrimSpace(renderStorageInline(inlineRun)); text != "" {
			parts = append(parts, text)
		}
		inlineRun = nil
	}
	for _, node := range nodes {
		if node.name == "" && strings.TrimSpace(node.text) == "" {
			continue
		}
		if storageBlockElements[node.name] || isStorageBlockCard(node) {
			flush()
			if rendered := renderStorageBlock(node); strings.TrimSpace(rendered) != "" {
				parts = append(parts, rendered)
			}
			continue
		}
		inlineRun = append(inlineRun, node)
	}
	flush()
	return strings.Join(parts, "\n\n")
}

// isStorageBlockCard reports whether an <ac:link> is a block-appearance card
// (its own line in the editor) rather than an inline link.
func isStorageBlockCard(n *storageNode) bool {
	return n.name == "ac:link" && n.attr("ac:card-appearance") == "block"
}

func renderStorageBlock(n *storageNode) string {
	switch n.name {
	case "p":
		return strings.TrimSpace(renderStorageInline(n.kids))
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level, _ := strconv.Atoi(strings.TrimPrefix(n.name, "h"))
		return strings.Repeat("#", level) + " " + strings.TrimSpace(renderStorageInline(n.kids))
	case "ul":
		return renderStorageList(n, false)
	case "ol":
		return renderStorageList(n, true)
	case "blockquote":
		return prefixLines(renderStorageBlocks(n.kids), "> ")
	case "pre":
		return "```\n" + strings.TrimRight(n.collectText(), "\n") + "\n```"
	case "hr":
		return "---"
	case "table":
		return renderStorageTable(n)
	case "ac:structured-macro":
		return renderStorageMacro(n)
	case "ac:task-list":
		return renderStorageTaskList(n)
	case "ac:link":
		return renderStorageLink(n)
	case "div", "section", "ac:layout", "ac:layout-section", "ac:layout-cell", "ac:rich-text-body":
		return renderStorageBlocks(n.kids)
	default:
		return renderStorageBlocks(n.kids)
	}
}

func renderStorageMacro(n *storageNode) string {
	name := strings.ToLower(n.attr("ac:name"))
	switch name {
	case "code":
		body := ""
		if plain := n.find("ac:plain-text-body"); plain != nil {
			body = plain.collectText()
		}
		return "```" + n.macroParameter("language") + "\n" + strings.TrimRight(body, "\n") + "\n```"
	case "info", "note", "warning", "tip", "panel", "expand":
		inner := ""
		if rich := n.find("ac:rich-text-body"); rich != nil {
			inner = renderStorageBlocks(rich.kids)
		}
		label := strings.ToUpper(name)
		if title := n.macroParameter("title"); title != "" {
			label += ": " + title
		}
		return prefixLines("**"+label+"**\n\n"+inner, "> ")
	case "status":
		if title := n.macroParameter("title"); title != "" {
			return "[" + title + "]"
		}
		return ""
	case "toc", "children", "anchor":
		return ""
	default:
		if rich := n.find("ac:rich-text-body"); rich != nil {
			return renderStorageBlocks(rich.kids)
		}
		if plain := n.find("ac:plain-text-body"); plain != nil {
			return strings.TrimSpace(plain.collectText())
		}
		return ""
	}
}

func renderStorageTaskList(n *storageNode) string {
	var lines []string
	for _, task := range n.kids {
		if task.name != "ac:task" {
			continue
		}
		marker := "- [ ] "
		if status := task.find("ac:task-status"); status != nil && strings.TrimSpace(status.collectText()) == "complete" {
			marker = "- [x] "
		}
		body := ""
		if taskBody := task.find("ac:task-body"); taskBody != nil {
			body = strings.TrimSpace(renderStorageInline(taskBody.kids))
		}
		lines = append(lines, marker+body)
	}
	return strings.Join(lines, "\n")
}

func renderStorageList(n *storageNode, ordered bool) string {
	var lines []string
	number := 0
	for _, item := range n.kids {
		if item.name != "li" {
			continue
		}
		number++
		marker := "- "
		if ordered {
			marker = strconv.Itoa(number) + ". "
		}
		indent := strings.Repeat(" ", len(marker))
		content := renderStorageListItem(item)
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

// renderStorageListItem joins a list item's blocks tightly (single newline) so
// nested lists sit directly under their parent line.
func renderStorageListItem(item *storageNode) string {
	blocks := renderStorageBlocks(item.kids)
	return strings.ReplaceAll(blocks, "\n\n", "\n")
}

func renderStorageTable(n *storageNode) string {
	var rows [][]string
	var walkRows func(nodes []*storageNode)
	walkRows = func(nodes []*storageNode) {
		for _, node := range nodes {
			switch node.name {
			case "tr":
				var cells []string
				for _, cell := range node.kids {
					if cell.name != "td" && cell.name != "th" {
						continue
					}
					text := strings.TrimSpace(strings.ReplaceAll(renderStorageBlocks(cell.kids), "\n", " "))
					cells = append(cells, strings.ReplaceAll(text, "|", "\\|"))
				}
				rows = append(rows, cells)
			case "thead", "tbody", "tfoot":
				walkRows(node.kids)
			}
		}
	}
	walkRows(n.kids)
	if len(rows) == 0 {
		return ""
	}
	width := 0
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}
	pad := func(row []string) []string {
		for len(row) < width {
			row = append(row, "")
		}
		return row
	}
	var b strings.Builder
	b.WriteString("| " + strings.Join(pad(rows[0]), " | ") + " |\n")
	separators := make([]string, width)
	for i := range separators {
		separators[i] = "---"
	}
	b.WriteString("| " + strings.Join(separators, " | ") + " |")
	for _, row := range rows[1:] {
		b.WriteString("\n| " + strings.Join(pad(row), " | ") + " |")
	}
	return b.String()
}

func renderStorageInline(nodes []*storageNode) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.name {
		case "":
			b.WriteString(collapseStorageWhitespace(n.text))
		case "strong", "b":
			b.WriteString("**" + strings.TrimSpace(renderStorageInline(n.kids)) + "**")
		case "em", "i":
			b.WriteString("*" + strings.TrimSpace(renderStorageInline(n.kids)) + "*")
		case "s", "del", "strike":
			b.WriteString("~~" + strings.TrimSpace(renderStorageInline(n.kids)) + "~~")
		case "code":
			b.WriteString("`" + n.collectText() + "`")
		case "a":
			text := strings.TrimSpace(renderStorageInline(n.kids))
			href := n.attr("href")
			switch {
			case href == "":
				b.WriteString(text)
			case text == "" || text == href:
				b.WriteString(href)
			default:
				b.WriteString("[" + text + "](" + href + ")")
			}
		case "br":
			b.WriteString("\n")
		case "time":
			b.WriteString(n.attr("datetime"))
		case "ac:link":
			b.WriteString(renderStorageLink(n))
		case "ac:image":
			b.WriteString(renderStorageImage(n))
		case "ac:emoticon":
			if fallback := n.attr("ac:emoji-fallback"); fallback != "" {
				b.WriteString(fallback)
			} else if name := n.attr("ac:name"); name != "" {
				b.WriteString(":" + name + ":")
			}
		case "ac:placeholder":
			// editor-only hint, not content
		case "span", "u", "sub", "sup", "ac:inline-comment-marker":
			b.WriteString(renderStorageInline(n.kids))
		default:
			if len(n.kids) > 0 {
				b.WriteString(renderStorageInline(n.kids))
			}
		}
	}
	return b.String()
}

// renderStorageLink renders <ac:link> targets: page links by title, user
// links as @account-id, attachment links by filename — preferring an explicit
// link body when present.
func renderStorageLink(n *storageNode) string {
	body := ""
	if plain := n.find("ac:plain-text-link-body"); plain != nil {
		body = strings.TrimSpace(plain.collectText())
	} else if rich := n.find("ac:link-body"); rich != nil {
		body = strings.TrimSpace(renderStorageInline(rich.kids))
	}
	if page := n.find("ri:page"); page != nil {
		if body != "" {
			return body
		}
		return page.attr("ri:content-title")
	}
	if user := n.find("ri:user"); user != nil {
		if body != "" {
			return body
		}
		if id := firstNonEmptyStorage(user.attr("ri:account-id"), user.attr("ri:userkey")); id != "" {
			return "@" + id
		}
		return "@unknown"
	}
	if attachment := n.find("ri:attachment"); attachment != nil {
		if body != "" {
			return body
		}
		return attachment.attr("ri:filename")
	}
	return body
}

func renderStorageImage(n *storageNode) string {
	alt := firstNonEmptyStorage(n.attr("ac:alt"), n.attr("ac:title"))
	if attachment := n.find("ri:attachment"); attachment != nil {
		return "![" + alt + "](" + attachment.attr("ri:filename") + ")"
	}
	if remote := n.find("ri:url"); remote != nil {
		return "![" + alt + "](" + remote.attr("ri:value") + ")"
	}
	return ""
}

// collapseStorageWhitespace folds the insignificant newlines and indentation
// that pretty-printed storage XML carries inside text nodes.
func collapseStorageWhitespace(text string) string {
	if strings.TrimSpace(text) == "" {
		if text == "" {
			return ""
		}
		return " "
	}
	return storageWhitespacePattern.ReplaceAllString(text, " ")
}

var storageWhitespacePattern = regexp.MustCompile(`[ \t]*\n[ \t]*`)

func firstNonEmptyStorage(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
