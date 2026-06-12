package vision

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

const (
	PluginName            = "vision"
	PluginVersion         = "0.19.1"
	PluginDescription     = "Generic image understanding aggregator over vision provider plugins."
	OperationAnalyze      = "vision.analyze"
	OperationProviderList = "vision.provider.list"
	ContextProviderName   = "vision.context"
	ProviderRuntimeName   = "vision"
	ProviderActionList    = "providers"
	ProviderActionAnalyze = "analyze"
	fanoutConcurrency     = 3
)

type NoInput struct{}

type ProviderRuntime interface {
	Providers(pluginbinding.Context) ([]Provider, error)
	Analyze(pluginbinding.Context, ProviderAnalyzeRequest) (ProviderAnalyzeResponse, error)
}

type ProviderAnalyzeRequest struct {
	Target Provider     `json:"target"`
	Input  AnalyzeInput `json:"input"`
}

type ProviderAnalyzeResponse struct {
	Result AnalysisResult `json:"result"`
	Errors []AnalyzeError `json:"errors"`
}

type HostProviderRuntime struct{}

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithProviderRuntime(HostProviderRuntime{})
}

func NewPluginWithProviderRuntime(runtime ProviderRuntime) *pluginbinding.Plugin {
	if runtime == nil {
		runtime = HostProviderRuntime{}
	}
	service := Service{Runtime: runtime}
	return pluginbinding.Define(ManifestSpec(),
		pluginbinding.RegisterOperation(providerListSpec(), service.Providers),
		pluginbinding.RegisterOperation(analyzeSpec(), service.Analyze),
		pluginbinding.RegisterContextProvider(contextSpec(), service.Context),
	)
}

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(ManifestSpec())
}

func ManifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"image-vision", "vision"},
		Operations: []core.OperationSpec{
			providerListSpec(),
			analyzeSpec(),
		},
		Context: []core.ContextSpec{
			contextSpec(),
		},
		Metadata: map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
	}
}

func (HostProviderRuntime) Providers(ctx pluginbinding.Context) ([]Provider, error) {
	raw, err := json.Marshal(NoInput{})
	if err != nil {
		return nil, err
	}
	resp, err := ctx.Host.CapabilityCall(pluginbinding.ProviderCallRequest{
		Provider: ProviderRuntimeName,
		Action:   ProviderActionList,
		Payload:  raw,
	})
	if err != nil {
		return nil, err
	}
	var out ProviderListResult
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return nil, err
	}
	return out.Providers, nil
}

func (HostProviderRuntime) Analyze(ctx pluginbinding.Context, input ProviderAnalyzeRequest) (ProviderAnalyzeResponse, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return ProviderAnalyzeResponse{}, err
	}
	resp, err := ctx.Host.CapabilityCall(pluginbinding.ProviderCallRequest{
		Provider: ProviderRuntimeName,
		Action:   ProviderActionAnalyze,
		Payload:  raw,
	})
	if err != nil {
		return ProviderAnalyzeResponse{}, err
	}
	var out ProviderAnalyzeResponse
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return ProviderAnalyzeResponse{}, err
	}
	return out, nil
}

type Service struct {
	Runtime ProviderRuntime
}

func (s Service) runtime() ProviderRuntime {
	if s.Runtime == nil {
		return HostProviderRuntime{}
	}
	return s.Runtime
}

func (s Service) Providers(ctx pluginbinding.Context, _ NoInput) (ProviderListResult, error) {
	providers, err := s.runtime().Providers(ctx)
	if err != nil {
		return ProviderListResult{}, err
	}
	return ProviderListResult{Providers: providers, Count: len(providers)}, nil
}

func (s Service) Context(ctx pluginbinding.Context, input pluginbinding.ContextBuildInput) (pluginbinding.ContextBuildResult, error) {
	providers, err := s.runtime().Providers(ctx)
	if err != nil {
		providers = nil
	}
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = append(names, provider.Name)
	}
	content := "Vision aggregates image-understanding provider plugins through provider discovery."
	if len(names) > 0 {
		content += " Available providers: " + strings.Join(names, ", ") + "."
	} else {
		content += " No provider plugins are currently available."
	}
	if query := strings.TrimSpace(input.Query); query != "" {
		content += " Query: " + query + "."
	}
	return pluginbinding.ContextBuildResult{
		Blocks: []core.ContextBlock{{
			ID:       ContextProviderName,
			Kind:     pluginbinding.ContextKindText,
			Title:    "Vision context",
			Content:  content,
			Priority: 30,
			Metadata: map[string]string{
				"providers": strings.Join(names, ","),
				"operation": OperationAnalyze,
			},
		}},
	}, nil
}

func (s Service) Analyze(ctx pluginbinding.Context, input AnalyzeInput) (AnalyzeOutput, error) {
	output := s.run(ctx, input)
	if len(output.Results) == 0 {
		return output, pluginbinding.Fail("vision_failed", firstAnalyzeError(output, "vision analysis returned no results"))
	}
	return output, nil
}

func (s Service) run(ctx pluginbinding.Context, input AnalyzeInput) AnalyzeOutput {
	if err := ValidateImages(input.Images); err != nil {
		return AnalyzeOutput{Errors: []AnalyzeError{{Message: err.Error()}}, Results: []AnalysisResult{}}
	}
	available, err := s.runtime().Providers(ctx)
	if err != nil {
		return AnalyzeOutput{Errors: []AnalyzeError{{Message: err.Error()}}, Results: []AnalysisResult{}}
	}
	providers, errors := SelectProviders(available, input.Providers)
	output := AnalyzeOutput{Errors: errors, Results: []AnalysisResult{}}
	if len(providers) == 0 {
		if len(output.Errors) == 0 {
			output.Errors = append(output.Errors, AnalyzeError{Message: "no vision provider is available"})
		}
		return output
	}
	type jobResult struct {
		index  int
		result AnalysisResult
		errors []AnalyzeError
		err    error
	}
	results := make([]jobResult, len(providers))
	sem := make(chan struct{}, fanoutConcurrency)
	var wg sync.WaitGroup
	for i, provider := range providers {
		wg.Add(1)
		go func(index int, provider Provider) {
			defer wg.Done()
			sem <- struct{}{}
			providerInput := input
			providerInput.Model = ModelForProvider(input, provider)
			resp, err := s.runtime().Analyze(ctx, ProviderAnalyzeRequest{Target: provider, Input: providerInput})
			<-sem
			results[index] = jobResult{index: index, result: resp.Result, errors: resp.Errors, err: err}
		}(i, provider)
	}
	wg.Wait()
	for _, result := range results {
		provider := providers[result.index]
		for _, analysisErr := range result.errors {
			if strings.TrimSpace(analysisErr.Provider) == "" {
				analysisErr.Provider = provider.Name
			}
			output.Errors = append(output.Errors, analysisErr)
		}
		if result.err != nil {
			if len(result.errors) == 0 {
				output.Errors = append(output.Errors, AnalyzeError{Provider: provider.Name, Message: result.err.Error()})
			}
			continue
		}
		if strings.TrimSpace(result.result.Text) == "" {
			output.Errors = append(output.Errors, AnalyzeError{Provider: provider.Name, Message: "provider returned no analysis text"})
			continue
		}
		if strings.TrimSpace(result.result.Provider) == "" {
			result.result.Provider = provider.Name
		}
		output.Results = append(output.Results, result.result)
	}
	return output
}

func providerListSpec() core.OperationSpec {
	return readProviderOperation[NoInput, ProviderListResult](OperationProviderList, "List available image vision provider plugins.")
}

func analyzeSpec() core.OperationSpec {
	return readProviderOperation[AnalyzeInput, AnalyzeOutput](OperationAnalyze, "Analyze one or more images through vision provider plugins.")
}

func readProviderOperation[I any, O any](name, description string) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[I, O](
		name,
		description,
		pluginbinding.ReadOnly(),
		pluginbinding.Compact(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	)
}

func contextSpec() core.ContextSpec {
	return pluginbinding.ContextSpec(ContextProviderName, "Image vision provider context.", pluginbinding.ContextKindText, pluginbinding.ContextKindReference)
}

func firstAnalyzeError(output AnalyzeOutput, fallback string) string {
	if len(output.Errors) > 0 && strings.TrimSpace(output.Errors[0].Message) != "" {
		return output.Errors[0].Message
	}
	return fallback
}

// DecodeAnalyzeOutput decodes a provider operation's raw result payload into
// the typed analyze output. Callers holding a protocol envelope decode here,
// then map with ProviderAnalyzeFromOperationOutput.
func DecodeAnalyzeOutput(raw []byte) (AnalyzeOutput, error) {
	var output AnalyzeOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return AnalyzeOutput{}, err
	}
	return output, nil
}

// ProviderAnalyzeFromOperationOutput maps a provider's typed analyze output to
// its single result, defaulting the provider name.
func ProviderAnalyzeFromOperationOutput(provider Provider, output AnalyzeOutput) (ProviderAnalyzeResponse, error) {
	if len(output.Results) == 0 {
		return ProviderAnalyzeResponse{Errors: output.Errors}, fmt.Errorf("%s", firstAnalyzeError(output, "provider returned no results"))
	}
	result := output.Results[0]
	if strings.TrimSpace(result.Provider) == "" {
		result.Provider = provider.Name
	}
	return ProviderAnalyzeResponse{Result: result, Errors: output.Errors}, nil
}
