// Package registry exposes the default Fluxplane plugin marketplace.
package registry

import (
	_ "embed"
	"encoding/json"

	"github.com/fluxplane/fluxplane-plugin/manifest"
)

//go:embed marketplace.json
var marketplaceJSON []byte

// MarketplaceJSON returns a copy of the embedded marketplace JSON.
func MarketplaceJSON() []byte {
	return append([]byte(nil), marketplaceJSON...)
}

// Marketplace returns the embedded default marketplace.
func Marketplace() manifest.Marketplace {
	var marketplace manifest.Marketplace
	if err := json.Unmarshal(marketplaceJSON, &marketplace); err != nil {
		panic("fluxplane-plugins: invalid embedded marketplace: " + err.Error())
	}
	return marketplace
}
