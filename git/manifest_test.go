package git

import (
	"testing"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

func TestManifestDeclaresGitOperations(t *testing.T) {
	manifest := Manifest()
	plugintest.AssertManifestQuality(t, manifest)
	names := map[string]bool{}
	for _, op := range manifest.Operations {
		names[op.Name] = true
	}
	for _, name := range []string{OperationStatus, OperationDiff, OperationAdd, OperationCommit, OperationTag, OperationPush} {
		if !names[name] {
			t.Fatalf("operation %q missing from manifest: %#v", name, manifest.Operations)
		}
	}
	if len(manifest.Auth) != 0 || len(manifest.Datasources) != 0 || len(manifest.Indexes) != 0 {
		t.Fatalf("git manifest should not declare auth/datasources/indexes: %#v", manifest)
	}
	if manifest.Operations[0].Risk != core.OperationRiskLow {
		t.Fatalf("status risk = %q, want low", manifest.Operations[0].Risk)
	}
}
