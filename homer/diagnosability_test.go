package homer

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

func TestQueryFieldDescriptionListsAllFields(t *testing.T) {
	manifest := Manifest()
	for _, op := range manifest.Operations {
		if op.Name != OperationSearch {
			continue
		}
		var schema struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(op.Input, &schema); err != nil {
			t.Fatal(err)
		}
		desc := schema.Properties["query"].Description
		for _, field := range []string{"call_id", "cseq", "from_user", "method", "ruri_user", "sid", "status", "to_user", "ua", "user_agent"} {
			if !strings.Contains(desc, field) {
				t.Fatalf("query description truncated, missing %s: %q", field, desc)
			}
		}
		return
	}
	t.Fatal("search op not found")
}

func TestSearchEchoesInterpretedQuery(t *testing.T) {
	server := fakeHomer(t)
	defer server.Close()
	host := newHomerTestHost(server.URL)
	plugin := NewPluginWithService(Service{})
	out := plugintest.RunOK[SearchResultOutput](t, plugin, OperationSearch, map[string]any{
		"endpoint_ref": "homer-test",
		"number":       "493012345",
		"since":        "2h",
	}, plugintest.WithHost(host))
	if out.Query.From == "" || out.Query.To == "" {
		t.Fatalf("query echo missing window: %#v", out.Query)
	}
	if !strings.Contains(out.Query.SmartInput, "493012345") {
		t.Fatalf("query echo missing effective filters: %#v", out.Query)
	}
	if out.Query.Limit != 200 {
		t.Fatalf("query echo limit = %d", out.Query.Limit)
	}
}
