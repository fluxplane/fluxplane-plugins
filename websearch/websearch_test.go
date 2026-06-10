package websearch

import (
	"encoding/json"
	"testing"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

func TestNormalizeProvidersTreatsNamesAsOpaque(t *testing.T) {
	providers := NormalizeProviders([]string{" Tavily ", "ddg", "custom-provider", "ddg"})
	want := []string{"tavily", "ddg", "custom-provider"}
	if len(providers) != len(want) {
		t.Fatalf("providers = %#v", providers)
	}
	for i := range want {
		if providers[i] != want[i] {
			t.Fatalf("providers = %#v", providers)
		}
	}
}

func TestNormalizeResultURLRejectsUnsafeValues(t *testing.T) {
	tests := map[string]string{
		" https://example.com/path?q=1 ": "https://example.com/path?q=1",
		"http://example.com":             "http://example.com",
		"https://user:token@example.com": "",
		"javascript:alert(1)":            "",
		"https://example.com\nX: y":      "",
		"https:///missing-host":          "",
		"/relative/path":                 "",
	}
	for input, want := range tests {
		if got := NormalizeResultURL(input); got != want {
			t.Fatalf("NormalizeResultURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRecordsDropsUnsafeURLs(t *testing.T) {
	records := Records(pluginbinding.DatasourceSource{Plugin: "test"}, []ResultSet{{
		Provider: "provider",
		Query:    "query",
		Results: []Result{
			{URL: "https://example.com", Title: "Safe"},
			{URL: "javascript:alert(1)", Title: "Script"},
			{URL: "https://example.com\nHeader: value", Title: "Control"},
		},
	}})
	if len(records) != 1 || records[0].URL != "https://example.com" {
		t.Fatalf("records = %#v", records)
	}
}

func TestProviderOperationSpecCarriesInputExample(t *testing.T) {
	spec := ProviderOperationSpec(ProviderSpec{Name: "example", Operation: "example.search"})
	var schema struct {
		Examples []map[string]any `json:"examples"`
	}
	if err := json.Unmarshal(spec.Input, &schema); err != nil {
		t.Fatalf("decode input schema: %v", err)
	}
	if len(schema.Examples) == 0 || schema.Examples[0]["query"] == "" {
		t.Fatalf("examples = %#v, want a runnable query example", schema.Examples)
	}
}
