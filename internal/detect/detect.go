package detect

import (
	"fmt"
	"os"
	"path/filepath"
)

// Result holds the detected ecosystem and related metadata.
type Result struct {
	Name          string // "go", "python", "node"
	HasGoreleaser bool
	ManifestPath  string // path to the manifest file (empty for Go)
}

var manifests = map[string]string{
	"go.mod":         "go",
	"pyproject.toml": "python",
	"package.json":   "node",
}

// Ecosystem detects the project ecosystem from manifest files in dir.
// Pass override as "auto" to detect automatically, or a specific ecosystem name
// (e.g. "go") to bypass detection.
func Ecosystem(dir string, override string) (Result, error) {
	if override != "auto" && override != "" {
		return buildResult(dir, override)
	}

	var found []string
	for file, eco := range manifests {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			found = append(found, eco)
		}
	}

	if len(found) == 0 {
		return Result{}, fmt.Errorf("no ecosystem detected: expected go.mod, pyproject.toml, or package.json in %s", dir)
	}
	if len(found) > 1 {
		return Result{}, fmt.Errorf("multiple ecosystems detected (%v): set 'ecosystem' in .release-pilot.yaml", found)
	}

	return buildResult(dir, found[0])
}

func buildResult(dir string, eco string) (Result, error) {
	r := Result{Name: eco}

	switch eco {
	case "go":
		for _, name := range []string{".goreleaser.yaml", ".goreleaser.yml"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				r.HasGoreleaser = true
				break
			}
		}
	case "python":
		r.ManifestPath = filepath.Join(dir, "pyproject.toml")
	case "node":
		r.ManifestPath = filepath.Join(dir, "package.json")
	default:
		return Result{}, fmt.Errorf("unknown ecosystem: %s", eco)
	}

	return r, nil
}
