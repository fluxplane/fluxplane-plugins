package confluence

import (
	"testing"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestManifestDeclaresSharedAtlassianEnvFallbacks(t *testing.T) {
	manifest := Manifest()
	if manifest.Metadata[pluginbinding.ManifestProtocolKey] != protocol.Version {
		t.Fatalf("protocol metadata = %#v", manifest.Metadata)
	}
	if len(manifest.Auth) != 1 {
		t.Fatalf("auth = %#v", manifest.Auth)
	}
	fields := map[string]core.AuthField{}
	for _, field := range manifest.Auth[0].Fields {
		fields[field.Name] = field
	}
	if got := fields[AuthPurposeAPIToken].Env; len(got) != 2 || got[0] != EnvConfluenceAPIToken || got[1] != EnvAtlassianAPIToken {
		t.Fatalf("token env = %#v", got)
	}
	if got := fields[AuthPurposeCloudID].Env; len(got) != 2 || got[0] != "ATLASSIAN_CLOUD_ID" || got[1] != "CONFLUENCE_CLOUD_ID" {
		t.Fatalf("cloud id env = %#v", got)
	}
	if len(fields) != 2 {
		t.Fatalf("auth fields = %#v", fields)
	}
	if len(manifest.Endpoints) != 1 || manifest.Endpoints[0].Name != EndpointName || len(manifest.Endpoints[0].Env) != 3 {
		t.Fatalf("endpoints = %#v", manifest.Endpoints)
	}
	byEntity := map[string]core.DatasourceSpec{}
	for _, datasource := range manifest.Datasources {
		byEntity[datasource.Entity] = datasource
	}
	if byEntity[EntityPage].Fallback != core.DatasourceFallbackHostIndexFirst {
		t.Fatalf("page fallback = %q", byEntity[EntityPage].Fallback)
	}
}
