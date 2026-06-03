package registry_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginsDoNotImportCoreOrDex(t *testing.T) {
	forbidden := []string{
		"github.com/fluxplane/fluxplane-" + "core",
		"github.com/fluxplane/fluxplane-" + "dex",
	}
	var bad []string
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			pathValue := strings.Trim(imported.Path.Value, "\"")
			for _, prefix := range forbidden {
				if strings.HasPrefix(pathValue, prefix) {
					bad = append(bad, path+" imports forbidden module "+pathValue)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk plugin repo: %v", err)
	}
	if len(bad) > 0 {
		t.Fatalf("plugin implementation boundary violations:\n%s", strings.Join(bad, "\n"))
	}
}

func TestPluginsGoModDoesNotDependOnCoreOrDex(t *testing.T) {
	var bad []string
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if filepath.Base(path) != "go.mod" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(raw)
		for _, forbidden := range []string{
			"github.com/fluxplane/fluxplane-" + "core",
			"github.com/fluxplane/fluxplane-" + "dex",
		} {
			if strings.Contains(content, forbidden) {
				bad = append(bad, path+" contains forbidden dependency "+forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk plugin repo: %v", err)
	}
	if len(bad) > 0 {
		t.Fatalf("plugin go.mod boundary violations:\n%s", strings.Join(bad, "\n"))
	}
}
