package clock

import (
	"fmt"
	"strings"
	"sync"
	"time"

	fpcontext "github.com/fluxplane/fluxplane-context"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

const cacheTTL = 60 * time.Second

type Config struct {
	Now func() time.Time
	TZ  string
}

type provider struct {
	now     func() time.Time
	tz      string
	startAt time.Time

	mu        sync.Mutex
	lastAt    time.Time
	lastBlock fpcontext.Block
}

func newProvider(cfg Config) *provider {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &provider{now: now, tz: cfg.TZ, startAt: now()}
}

func (p *provider) Build(_ pluginbinding.Context, _ pluginbinding.ContextBuildInput) (pluginbinding.ContextBuildResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.now()
	if !p.lastAt.IsZero() && now.Sub(p.lastAt) < cacheTTL && p.lastBlock.ID != "" {
		return pluginbinding.ContextBuildResult{Blocks: []fpcontext.Block{p.lastBlock}}, nil
	}

	p.lastBlock = fpcontext.Block{
		ID:        "now",
		Provider:  ContextProviderName,
		Kind:      fpcontext.BlockData,
		Placement: fpcontext.PlacementSystem,
		Title:     "Time",
		Content:   renderTime(now, p.tz, p.startAt),
		MediaType: "text/plain",
		Priority:  90,
		Freshness: fpcontext.FreshnessDynamic,
	}
	p.lastAt = now
	return pluginbinding.ContextBuildResult{Blocks: []fpcontext.Block{p.lastBlock}}, nil
}

func renderTime(now time.Time, tz string, startAt time.Time) string {
	utc := now.UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString("Current time: ")
	b.WriteString(utc)
	if tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			fmt.Fprintf(&b, " (%s: %s)", tz, now.In(loc).Format(time.RFC3339))
		} else {
			fmt.Fprintf(&b, " (tz %q is unknown)", tz)
		}
	}
	if !startAt.IsZero() {
		fmt.Fprintf(&b, "\nUptime: %s (since %s)", formatAge(now.Sub(startAt)), startAt.UTC().Format(time.RFC3339))
	}
	return b.String()
}

func formatAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	days := int(d / (24 * time.Hour))
	d -= time.Duration(days) * 24 * time.Hour
	hours := int(d / time.Hour)
	d -= time.Duration(hours) * time.Hour
	minutes := int(d / time.Minute)
	d -= time.Duration(minutes) * time.Minute
	seconds := int(d / time.Second)
	switch {
	case days > 0:
		return fmt.Sprintf("%dd%dh%dm", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh%dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}
