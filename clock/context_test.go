package clock

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	fpcontext "github.com/fluxplane/fluxplane-context"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func TestManifestDeclaresClockContextProvider(t *testing.T) {
	manifest := Manifest()
	plugintest.AssertManifestQuality(t, manifest)
	if len(manifest.Context) != 1 {
		t.Fatalf("context providers = %#v", manifest.Context)
	}
	spec := manifest.Context[0]
	if spec.Name != ContextProviderName || spec.DefaultPlacement != fpcontext.PlacementSystem {
		t.Fatalf("context spec = %#v", spec)
	}
	if len(spec.Kinds) != 1 || spec.Kinds[0] != fpcontext.BlockData {
		t.Fatalf("context kinds = %#v", spec.Kinds)
	}
}

func TestContextProviderCachesWithin60s(t *testing.T) {
	base := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)
	clock := &fakeClock{t: base}
	p := newProvider(Config{Now: clock.Now})
	p.startAt = base.Add(-90 * time.Second)

	first, err := p.Build(pluginbinding.Context{Context: context.Background()}, pluginbinding.ContextBuildInput{})
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	if len(first.Blocks) != 1 {
		t.Fatalf("want 1 block, got %d", len(first.Blocks))
	}

	clock.advance(30 * time.Second)
	second, err := p.Build(pluginbinding.Context{Context: context.Background()}, pluginbinding.ContextBuildInput{})
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if second.Blocks[0].Content != first.Blocks[0].Content {
		t.Fatalf("expected cached content, got:\nfirst:  %q\nsecond: %q", first.Blocks[0].Content, second.Blocks[0].Content)
	}

	clock.advance(31 * time.Second)
	third, err := p.Build(pluginbinding.Context{Context: context.Background()}, pluginbinding.ContextBuildInput{})
	if err != nil {
		t.Fatalf("third build: %v", err)
	}
	if third.Blocks[0].Content == first.Blocks[0].Content {
		t.Fatalf("expected refreshed content after 61s; still got %q", third.Blocks[0].Content)
	}
}

func TestContextProviderIncludesUptimeAndTimezone(t *testing.T) {
	start := time.Date(2026, 5, 28, 9, 0, 0, 0, time.UTC)
	now := start.Add(2*time.Hour + 30*time.Minute)
	p := newProvider(Config{Now: func() time.Time { return now }, TZ: "Europe/Berlin"})
	p.startAt = start

	result, err := p.Build(pluginbinding.Context{Context: context.Background()}, pluginbinding.ContextBuildInput{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	block := result.Blocks[0]
	if block.Provider != ContextProviderName || block.Kind != fpcontext.BlockData || block.Placement != fpcontext.PlacementSystem || block.Freshness != fpcontext.FreshnessDynamic {
		t.Fatalf("block metadata = %#v", block)
	}
	for _, want := range []string{"Current time: 2026-05-28T11:30:00Z", "Uptime: 2h30m", "(Europe/Berlin:"} {
		if !strings.Contains(block.Content, want) {
			t.Fatalf("expected %q in content, got %q", want, block.Content)
		}
	}
}

func TestPluginContextBuildPreservesBlockMetadata(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	plugin := NewPlugin(WithNow(func() time.Time { return now }), WithTimezone("UTC"))
	resp := plugin.Handle(protocol.Request{
		Command: protocol.CommandContextBuild,
		Plugin:  PluginName,
		Payload: []byte(`{"query":"now"}`),
	})
	if !resp.OK {
		t.Fatalf("context build failed: %#v", resp.Error)
	}
	var result pluginbinding.ContextBuildResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Blocks) != 1 {
		t.Fatalf("blocks = %#v", result.Blocks)
	}
	block := result.Blocks[0]
	if block.Provider != ContextProviderName || block.MediaType != "text/plain" || block.Placement != fpcontext.PlacementSystem || block.Freshness != fpcontext.FreshnessDynamic {
		t.Fatalf("block = %#v", block)
	}
	if block.Source == nil || block.Source.Plugin != PluginName {
		t.Fatalf("source = %#v", block.Source)
	}
}

func TestFormatAge(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Second, "45s"},
		{2 * time.Minute, "2m0s"},
		{2*time.Minute + 15*time.Second, "2m15s"},
		{3*time.Hour + 5*time.Minute, "3h5m"},
		{49 * time.Hour, "2d1h0m"},
		{-time.Second, "0s"},
	}
	for _, c := range cases {
		if got := formatAge(c.d); got != c.want {
			t.Errorf("formatAge(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }
