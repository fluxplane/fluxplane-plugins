package registry

import "testing"

func TestMarketplaceLoadsEmbeddedCatalog(t *testing.T) {
	marketplace := Marketplace()
	if marketplace.Version != "1" {
		t.Fatalf("version = %q", marketplace.Version)
	}
	if len(marketplace.Plugins) == 0 {
		t.Fatal("plugins is empty")
	}
	for _, plugin := range marketplace.Plugins {
		if plugin.Name == "gitlab" {
			if plugin.Binary != "fluxplane-plugin-gitlab" {
				t.Fatalf("gitlab binary = %q", plugin.Binary)
			}
			return
		}
	}
	t.Fatal("gitlab marketplace entry missing")
}
