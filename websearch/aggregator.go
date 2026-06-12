package websearch

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	fpdatasource "github.com/fluxplane/fluxplane-datasource"
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

const (
	PluginName            = "websearch"
	PluginVersion         = "0.19.1"
	PluginDescription     = "Generic web search aggregator over provider plugins."
	OperationSearch       = "websearch.search"
	OperationProviderList = "websearch.provider.list"
	DatasourceResults     = "websearch.results"
	ContextProviderName   = "websearch.context"
	ProviderRuntimeName   = "websearch"
	ProviderActionList    = "providers"
	ProviderActionSearch  = "search"
	fanoutConcurrency     = 4
)

type NoInput struct{}

type ProviderRuntime interface {
	Providers(pluginbinding.Context) ([]Provider, error)
	Search(pluginbinding.Context, ProviderSearchRequest) (ProviderSearchResponse, error)
}

type ProviderSearchRequest struct {
	Target Provider `json:"target"`
	Query  string   `json:"query"`
	Max    int      `json:"max,omitempty"`
}

type ProviderSearchResponse struct {
	Set    ResultSet     `json:"set"`
	Errors []SearchError `json:"errors"`
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
		pluginbinding.RegisterOperation(searchSpec(), service.Search),
		pluginbinding.RegisterDatasourceSearch(aggregatorDatasourceSpec(), service.DatasourceSearch),
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
		Aliases:     []string{"web", "websearch"},
		Operations: []core.OperationSpec{
			providerListSpec(),
			searchSpec(),
		},
		Datasources: []core.DatasourceSpec{
			aggregatorDatasourceSpec(),
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

func (HostProviderRuntime) Search(ctx pluginbinding.Context, input ProviderSearchRequest) (ProviderSearchResponse, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return ProviderSearchResponse{}, err
	}
	resp, err := ctx.Host.CapabilityCall(pluginbinding.ProviderCallRequest{
		Provider: ProviderRuntimeName,
		Action:   ProviderActionSearch,
		Payload:  raw,
	})
	if err != nil {
		return ProviderSearchResponse{}, err
	}
	var out ProviderSearchResponse
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		return ProviderSearchResponse{}, err
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
	content := "Websearch aggregates provider plugins through provider discovery."
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
			Title:    "Websearch context",
			Content:  content,
			Priority: 30,
			Metadata: map[string]string{
				"providers": strings.Join(names, ","),
				"operation": OperationSearch,
			},
		}},
	}, nil
}

func (s Service) Search(ctx pluginbinding.Context, input SearchInput) (SearchOutput, error) {
	output := s.run(ctx, input)
	if len(output.Results) == 0 {
		return output, pluginbinding.Fail("web_search_failed", firstSearchError(output, "web search returned no results"))
	}
	return output, nil
}

func (s Service) DatasourceSearch(ctx pluginbinding.Context, input SearchInput) (DatasourceSearchResult, error) {
	output := s.run(ctx, input)
	result := ToDatasourceSearchResult(ctx.DatasourceSource(), input, output)
	if len(result.Records) == 0 {
		return result, pluginbinding.Fail("web_search_failed", firstSearchError(output, "web search returned no results"))
	}
	return result, nil
}

func (s Service) run(ctx pluginbinding.Context, input SearchInput) SearchOutput {
	queries := NormalizeQueries(input)
	if len(queries) == 0 {
		return SearchOutput{Errors: []SearchError{{Message: "at least one query is required"}}, Results: []ResultSet{}}
	}
	available, err := s.runtime().Providers(ctx)
	if err != nil {
		return SearchOutput{Errors: []SearchError{{Message: err.Error()}}, Results: []ResultSet{}}
	}
	providers, errors := SelectProviders(available, input.Providers)
	output := SearchOutput{Errors: errors, Results: []ResultSet{}}
	if len(providers) == 0 {
		if len(output.Errors) == 0 {
			output.Errors = append(output.Errors, SearchError{Message: "no web search provider is available"})
		}
		return output
	}
	max := NormalizeMax(input)
	type job struct {
		index    int
		query    string
		provider Provider
	}
	type jobResult struct {
		index  int
		set    ResultSet
		errors []SearchError
		err    error
	}
	var jobs []job
	for _, query := range queries {
		for _, provider := range providers {
			jobs = append(jobs, job{index: len(jobs), query: query, provider: provider})
		}
	}
	results := make([]jobResult, len(jobs))
	sem := make(chan struct{}, fanoutConcurrency)
	var wg sync.WaitGroup
	for _, current := range jobs {
		wg.Add(1)
		go func(current job) {
			defer wg.Done()
			sem <- struct{}{}
			resp, err := s.runtime().Search(ctx, ProviderSearchRequest{Target: current.provider, Query: current.query, Max: max})
			<-sem
			results[current.index] = jobResult{index: current.index, set: resp.Set, errors: resp.Errors, err: err}
		}(current)
	}
	wg.Wait()
	for _, result := range results {
		provider := jobs[result.index].provider
		query := jobs[result.index].query
		for _, searchErr := range result.errors {
			if strings.TrimSpace(searchErr.Provider) == "" {
				searchErr.Provider = provider.Name
			}
			if strings.TrimSpace(searchErr.Query) == "" {
				searchErr.Query = query
			}
			output.Errors = append(output.Errors, searchErr)
		}
		if result.err != nil {
			if len(result.errors) == 0 {
				output.Errors = append(output.Errors, SearchError{Provider: provider.Name, Query: query, Message: result.err.Error()})
			}
			continue
		}
		if !HasResults(result.set) {
			output.Errors = append(output.Errors, SearchError{Provider: provider.Name, Query: query, Message: "provider returned no results"})
			continue
		}
		set := result.set
		if strings.TrimSpace(set.Provider) == "" {
			set.Provider = provider.Name
		}
		if strings.TrimSpace(set.Query) == "" {
			set.Query = query
		}
		output.Results = append(output.Results, set)
	}
	return output
}

func providerListSpec() core.OperationSpec {
	return readProviderOperation[NoInput, ProviderListResult](OperationProviderList, "List available web search provider plugins.")
}

func searchSpec() core.OperationSpec {
	return readProviderOperation[SearchInput, SearchOutput](OperationSearch, "Search the web through provider plugins.")
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

func aggregatorDatasourceSpec() core.DatasourceSpec {
	spec := DatasourceSpec(DatasourceResults, "Aggregated web search results.")
	spec.Access = []fpdatasource.Access{fpdatasource.AccessProvider}
	return spec
}

func contextSpec() core.ContextSpec {
	return pluginbinding.ContextSpec(ContextProviderName, "Web search provider context.", pluginbinding.ContextKindText, pluginbinding.ContextKindReference)
}

func firstSearchError(output SearchOutput, fallback string) string {
	if len(output.Errors) > 0 && strings.TrimSpace(output.Errors[0].Message) != "" {
		return output.Errors[0].Message
	}
	return fallback
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func MetadataString(metadata map[string]any, key string) string {
	if value, ok := metadata[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

// DecodeSearchOutput decodes a provider operation's raw result payload into
// the typed search output. Callers holding a protocol envelope decode here,
// then map with ProviderSearchFromOperationOutput.
func DecodeSearchOutput(raw []byte) (SearchOutput, error) {
	var output SearchOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return SearchOutput{}, err
	}
	return output, nil
}

// ProviderSearchFromOperationOutput maps a provider's typed search output to
// its single result set, defaulting the provider name and query.
func ProviderSearchFromOperationOutput(provider Provider, query string, output SearchOutput) (ProviderSearchResponse, error) {
	if len(output.Results) == 0 {
		return ProviderSearchResponse{Errors: output.Errors}, fmt.Errorf("%s", firstSearchError(output, "provider returned no results"))
	}
	set := output.Results[0]
	if strings.TrimSpace(set.Provider) == "" {
		set.Provider = provider.Name
	}
	if strings.TrimSpace(set.Query) == "" {
		set.Query = query
	}
	return ProviderSearchResponse{Set: set, Errors: output.Errors}, nil
}
