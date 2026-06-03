package openapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	auth "github.com/fluxplane/fluxplane-auth"
	sdkmanifest "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

func TestPluginGeneratesOperationsDatasourceAndAuthMethods(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	plugin := newTestPlugin(t, dir, testConfig())
	manifest := plugin.Manifest()
	if len(manifest.Operations) != 2 {
		t.Fatalf("operations = %d, want 2", len(manifest.Operations))
	}
	if !hasOperation(manifest.Operations, "users_get_user") {
		t.Fatalf("users_get_user was not generated: %#v", manifest.Operations)
	}
	if len(manifest.Datasources) == 0 || manifest.Datasources[0].Name != "users_api_docs" {
		t.Fatalf("datasources = %#v, want users_api_docs", manifest.Datasources)
	}
	if len(manifest.Auth) != 1 || manifest.Auth[0].Name != "bearerAuth" || len(manifest.Auth[0].Fields) != 1 {
		t.Fatalf("auth = %#v", manifest.Auth)
	}
}

func TestGeneratedOperationExecutesWithBearerAuth(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	host := &fakeHost{secret: "secret-token", http: pluginbinding.HTTPResponse{
		StatusCode:  200,
		Headers:     map[string][]string{"Content-Type": {"application/json"}},
		ContentType: "application/json",
		Body:        []byte(`{"id":"42","name":"Ada"}`),
	}}
	plugin := newTestPlugin(t, dir, testConfig())
	out := plugintest.RunOK[OperationResult](t, plugin, "users_get_user", map[string]any{
		"path":  map[string]any{"id": "42"},
		"query": map[string]any{"verbose": true},
	}, plugintest.WithHost(host), plugintest.WithInstance("users"))
	if out.Status != 200 {
		t.Fatalf("output = %#v", out)
	}
	if got, want := host.httpReq.URL, "https://api.example.test/v1/users/42?verbose=true"; got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
	if host.httpReq.Auth == nil || host.httpReq.Auth.BearerTokenPurpose != "bearerAuth" {
		t.Fatalf("auth request = %#v", host.httpReq.Auth)
	}
}

func TestDocumentationDatasourceSearchAndGet(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir)
	plugin := newTestPlugin(t, dir, testConfig())
	search := plugintest.DatasourceSearchOK[pluginbinding.DatasourceSearchResult[DocRecord]](t, plugin, map[string]any{
		"datasource": "users_api_docs",
		"entity":     string(OperationEntity),
		"query":      "Get user",
		"limit":      10,
	})
	if search.Count != 1 || search.Records[0].ID != "operation:get_user" {
		t.Fatalf("search = %#v", search)
	}
	get := plugintest.DatasourceGetOK[pluginbinding.DatasourceGetResult[DocRecord]](t, plugin, map[string]any{
		"datasource": "users_api_docs",
		"entity":     string(SchemaEntity),
		"id":         "schema:User",
	})
	if get.Record.ID != "schema:User" {
		t.Fatalf("get = %#v", get)
	}
}

func TestPluginLoadsSpecWithRefSiblingDescription(t *testing.T) {
	dir := t.TempDir()
	spec := `openapi: 3.0.3
info:
  title: Ref Sibling API
  version: "1.0"
paths:
  /apps:
    get:
      operationId: listApps
      responses:
        "200":
          description: OK
components:
  schemas:
    Application:
      $ref: "#/components/schemas/BaseApplication"
      description: Application metadata.
    BaseApplication:
      type: object
      properties:
        id:
          type: string
`
	if err := os.WriteFile(filepath.Join(dir, "ref-sibling.yaml"), []byte(spec), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	plugin := newTestPlugin(t, dir, Config{Specs: []SpecConfig{{
		File:       "ref-sibling.yaml",
		Operations: OperationsConfig{Prefix: "ref"},
		Datasource: DatasourceConfig{Name: "ref_api_docs"},
	}}})
	if !hasOperation(plugin.Manifest().Operations, "ref_list_apps") {
		t.Fatalf("ref_list_apps was not generated")
	}
}

func newTestPlugin(t *testing.T, dir string, cfg Config) *pluginbinding.Plugin {
	t.Helper()
	plugin, err := NewPlugin(context.Background(), cfg, Options{Root: dir, Instance: "users"})
	if err != nil {
		t.Fatalf("NewPlugin: %v", err)
	}
	return plugin
}

func testConfig() Config {
	return Config{Specs: []SpecConfig{{
		File:       "openapi.yaml",
		Operations: OperationsConfig{Prefix: "users", Include: []string{"getUser", "listUsers"}},
		Datasource: DatasourceConfig{Name: "users_api_docs", Index: DatasourceIndexConfig{Enabled: true}},
		Auth: AuthConfig{Schemes: map[string]AuthSchemeConfig{"bearerAuth": {
			Method: "env",
			Kind:   "bearer_token",
			Env:    auth.EnvSpec{Name: "TESTUSER_PASSWORD"},
		}}},
	}}}
}

func writeFixture(t *testing.T, dir string) {
	t.Helper()
	spec := `openapi: 3.0.3
info:
  title: Users API
  version: "1.0"
servers:
  - url: https://api.example.test/v1
security:
  - bearerAuth: []
paths:
  /users:
    get:
      operationId: listUsers
      summary: List users
      responses:
        "200":
          description: User list
  /users/{id}:
    get:
      operationId: getUser
      summary: Get user
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
        - name: verbose
          in: query
          schema:
            type: boolean
      responses:
        "200":
          description: User response
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
  schemas:
    User:
      type: object
      required: [id]
      properties:
        id:
          type: string
        name:
          type: string
`
	if err := os.WriteFile(filepath.Join(dir, "openapi.yaml"), []byte(spec), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func hasOperation(ops []sdkmanifest.OperationSpec, name string) bool {
	for _, op := range ops {
		if op.Name == name {
			return true
		}
	}
	return false
}

type fakeHost struct {
	pluginbinding.HostClient
	secret  string
	http    pluginbinding.HTTPResponse
	httpReq pluginbinding.HTTPRequest
}

func (h *fakeHost) Secret(purpose string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{Purpose: purpose, Value: h.secret}, nil
}

func (h *fakeHost) HTTP(req pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	h.httpReq = req
	if h.http.StatusCode == 0 {
		h.http.StatusCode = 200
	}
	return h.http, nil
}
