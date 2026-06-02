package websearch

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

func TestAggregatorSupportsDirectProviderRuntime(t *testing.T) {
	runtime := &fakeProviderRuntime{providers: []Provider{{
		Name:      "example-provider",
		Plugin:    "example",
		Aliases:   []string{"example"},
		Operation: "example.search",
	}}}
	plugin := NewPluginWithProviderRuntime(runtime)

	providers := plugintest.RunOK[ProviderListResult](t, plugin, OperationProviderList, NoInput{})
	if providers.Count != 1 || providers.Providers[0].Name != "example-provider" {
		t.Fatalf("providers = %#v", providers)
	}

	output := plugintest.RunOK[SearchOutput](t, plugin, OperationSearch, SearchInput{Query: "fluxplane", Providers: []string{"example"}, Max: 2})
	if len(output.Results) != 1 || output.Results[0].Provider != "example-provider" || output.Results[0].Query != "fluxplane" {
		t.Fatalf("output = %#v", output)
	}
	if got := runtime.calls; len(got) != 1 || got[0] != "example-provider:fluxplane:2" {
		t.Fatalf("calls = %#v", got)
	}
}

type fakeProviderRuntime struct {
	providers []Provider
	calls     []string
}

func (f *fakeProviderRuntime) Providers(pluginbinding.Context) ([]Provider, error) {
	return f.providers, nil
}

func (f *fakeProviderRuntime) Search(_ pluginbinding.Context, input ProviderSearchRequest) (ProviderSearchResponse, error) {
	f.calls = append(f.calls, fmt.Sprintf("%s:%s:%d", input.Target.Name, input.Query, input.Max))
	if !strings.Contains(input.Query, "fluxplane") {
		return ProviderSearchResponse{}, fmt.Errorf("unexpected query")
	}
	return ProviderSearchResponse{Set: ResultSet{
		Provider: input.Target.Name,
		Query:    input.Query,
		Results:  []Result{{URL: "https://example.com", Title: "Example"}},
	}}, nil
}
