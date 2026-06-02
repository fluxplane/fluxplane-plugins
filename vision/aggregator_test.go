package vision

import (
	"fmt"
	"testing"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

func TestAggregatorSupportsDirectProviderRuntime(t *testing.T) {
	runtime := &fakeProviderRuntime{providers: []Provider{{
		Name:      "example-provider",
		Plugin:    "example",
		Aliases:   []string{"example"},
		Operation: "example.vision.analyze",
	}}}
	plugin := NewPluginWithProviderRuntime(runtime)

	providers := plugintest.RunOK[ProviderListResult](t, plugin, OperationProviderList, NoInput{})
	if providers.Count != 1 || providers.Providers[0].Name != "example-provider" {
		t.Fatalf("providers = %#v", providers)
	}

	output := plugintest.RunOK[AnalyzeOutput](t, plugin, OperationAnalyze, AnalyzeInput{
		Prompt:    "read it",
		Providers: []string{"example"},
		Model:     "fallback",
		Models:    map[string]string{"example-provider": "direct-model"},
		Images:    []ImageInput{{URL: "https://example.com/image.png"}},
	})
	if len(output.Results) != 1 || output.Results[0].Provider != "example-provider" || output.Results[0].Text != "analysis: read it" {
		t.Fatalf("output = %#v", output)
	}
	if got := runtime.calls; len(got) != 1 || got[0] != "example-provider:direct-model" {
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

func (f *fakeProviderRuntime) Analyze(_ pluginbinding.Context, input ProviderAnalyzeRequest) (ProviderAnalyzeResponse, error) {
	f.calls = append(f.calls, fmt.Sprintf("%s:%s", input.Target.Name, input.Input.Model))
	return ProviderAnalyzeResponse{Result: AnalysisResult{
		Provider: input.Target.Name,
		Model:    input.Input.Model,
		Text:     "analysis: " + NormalizePrompt(input.Input.Prompt),
	}}, nil
}
