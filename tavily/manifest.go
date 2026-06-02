package tavily

import (
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/websearch"
)

const (
	PluginName        = "tavily"
	PluginVersion     = "0.18.2"
	PluginDescription = "Tavily web search provider."

	AuthMethodAPIKey  = "api_key"
	AuthPurposeAPIKey = "api_key"
	EnvTavilyAPIKey   = "TAVILY_API_KEY"

	OperationSearch = "tavily.search"

	DatasourceWebSearch = "tavily.web_search"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return websearch.ProviderManifestSpec(providerSpec())
}

func providerSpec() websearch.ProviderSpec {
	return websearch.ProviderSpec{
		Name:                  PluginName,
		Version:               PluginVersion,
		Description:           PluginDescription,
		Aliases:               []string{PluginName},
		Operation:             OperationSearch,
		Datasource:            DatasourceWebSearch,
		OperationDescription:  "Search the web with Tavily.",
		DatasourceDescription: "Tavily web search results.",
		Auth: []core.AuthMethod{pluginbinding.BearerAuth(
			AuthMethodAPIKey,
			"Tavily API key resolved by dex secret broker.",
			pluginbinding.AuthField(AuthPurposeAPIKey, "Tavily API key", true, true, EnvTavilyAPIKey),
		)},
		SecretPurposes: []string{AuthPurposeAPIKey},
	}
}
