package homer

import (
	"fmt"
	"strings"
)

// RenderLadderSVG draws a call flow as an SVG sequence diagram: one lifeline
// per host (in first-appearance order), one arrow per SIP message labeled with
// method and offset, SDP/media annotations inline, and failure highlighting
// (>=400 responses, BYE, and CANCEL in red; 2xx green; 1xx gray). Multi-leg
// flows (several Call-IDs, e.g. from call.analyze) render on shared lifelines
// with a per-leg prefix on the label.
func RenderLadderSVG(events []FlowEvent) []byte {
	const (
		laneWidth  = 240
		leftMargin = 130
		headerY    = 46
		firstRowY  = 96
		rowHeight  = 30
	)
	lanes := ladderLanes(events)
	laneX := map[string]int{}
	for i, lane := range lanes {
		laneX[lane] = leftMargin + i*laneWidth
	}
	legPrefix := ladderLegPrefixes(events)

	width := leftMargin + len(lanes)*laneWidth + 60
	height := firstRowY + len(events)*rowHeight + 40

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" font-family="monospace" font-size="12">`, width, height)
	b.WriteString(`<defs>` +
		`<marker id="arrow" markerWidth="10" markerHeight="8" refX="9" refY="4" orient="auto"><path d="M0,0 L10,4 L0,8 z" fill="context-stroke"/></marker>` +
		`</defs>`)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="white"/>`, width, height)

	// Lifelines and headers.
	for _, lane := range lanes {
		x := laneX[lane]
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="180" height="24" rx="4" fill="#eef2f7" stroke="#7a8aa0"/>`, x-90, headerY-18)
		fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle" fill="#1d2939">%s</text>`, x, headerY, svgEscape(lane))
		fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#b6c2d2" stroke-dasharray="4,4"/>`, x, headerY+10, x, height-20)
	}

	// Messages.
	for i, event := range events {
		y := firstRowY + i*rowHeight
		x1, ok1 := laneX[event.Src]
		x2, ok2 := laneX[event.Dst]
		if !ok1 || !ok2 {
			continue
		}
		color := ladderColor(event.Method)
		label := fmt.Sprintf("%s%s +%dms", legPrefix[event.CallID], svgEscape(event.Method), event.OffsetMS)
		if event.SDP != "" {
			label += "  [" + svgEscape(event.SDP) + "]"
		}
		mid := (x1 + x2) / 2
		if x1 == x2 {
			// Self-message: short loop to the right.
			fmt.Fprintf(&b, `<path d="M%d,%d h40 v10 h-40" fill="none" stroke="%s" marker-end="url(#arrow)"/>`, x1, y, color)
			fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s">%s</text>`, x1+46, y+8, color, label)
			continue
		}
		fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" marker-end="url(#arrow)"/>`, x1, y, x2, y, color)
		fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle" fill="%s">%s</text>`, mid, y-5, color, label)
	}
	b.WriteString(`</svg>`)
	return []byte(b.String())
}

// ladderLanes returns the unique hosts in first-appearance order.
func ladderLanes(events []FlowEvent) []string {
	var lanes []string
	seen := map[string]bool{}
	for _, event := range events {
		for _, host := range []string{event.Src, event.Dst} {
			if host != "" && !seen[host] {
				seen[host] = true
				lanes = append(lanes, host)
			}
		}
	}
	return lanes
}

// ladderLegPrefixes assigns "L<n>: " label prefixes when the flow spans more
// than one Call-ID (merged legs from call.analyze); single-leg flows get none.
func ladderLegPrefixes(events []FlowEvent) map[string]string {
	order := []string{}
	seen := map[string]bool{}
	for _, event := range events {
		if !seen[event.CallID] {
			seen[event.CallID] = true
			order = append(order, event.CallID)
		}
	}
	prefixes := map[string]string{}
	if len(order) <= 1 {
		for _, id := range order {
			prefixes[id] = ""
		}
		return prefixes
	}
	for i, id := range order {
		prefixes[id] = fmt.Sprintf("L%d: ", i+1)
	}
	return prefixes
}

// ladderColor highlights outcomes: failures and teardown red, success green,
// provisional gray, requests dark.
func ladderColor(method string) string {
	method = strings.TrimSpace(method)
	if code := responseCode(method); code > 0 {
		switch {
		case code >= 400:
			return "#cc2929"
		case code >= 200:
			return "#1e7a3c"
		default:
			return "#7a8aa0"
		}
	}
	switch strings.ToUpper(method) {
	case "BYE", "CANCEL":
		return "#cc2929"
	default:
		return "#1d2939"
	}
}

func responseCode(method string) int {
	if len(method) != 3 {
		return 0
	}
	code := 0
	for _, r := range method {
		if r < '0' || r > '9' {
			return 0
		}
		code = code*10 + int(r-'0')
	}
	return code
}

func svgEscape(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return replacer.Replace(s)
}
