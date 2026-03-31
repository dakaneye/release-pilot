package detect_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dakaneye/release-pilot/internal/detect"
)

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectGo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example")

	result, err := detect.Ecosystem(dir, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "go" {
		t.Errorf("expected go, got %s", result.Name)
	}
	if result.HasGoreleaser {
		t.Error("expected no goreleaser")
	}
}

func TestDetectGoWithGoreleaser(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example")
	writeFile(t, filepath.Join(dir, ".goreleaser.yaml"), "builds:")

	result, err := detect.Ecosystem(dir, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "go" {
		t.Errorf("expected go, got %s", result.Name)
	}
	if !result.HasGoreleaser {
		t.Error("expected goreleaser detected")
	}
}

func TestDetectPython(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[project]")

	result, err := detect.Ecosystem(dir, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "python" {
		t.Errorf("expected python, got %s", result.Name)
	}
}

func TestDetectNode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), "{}")

	result, err := detect.Ecosystem(dir, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "node" {
		t.Errorf("expected node, got %s", result.Name)
	}
}

func TestDetectAmbiguousFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example")
	writeFile(t, filepath.Join(dir, "package.json"), "{}")

	_, err := detect.Ecosystem(dir, "auto")
	if err == nil {
		t.Fatal("expected error for ambiguous ecosystem")
	}
}

func TestDetectAmbiguousWithOverride(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example")
	writeFile(t, filepath.Join(dir, "package.json"), "{}")

	result, err := detect.Ecosystem(dir, "go")
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "go" {
		t.Errorf("expected go, got %s", result.Name)
	}
}

func TestDetectNoManifest(t *testing.T) {
	dir := t.TempDir()

	_, err := detect.Ecosystem(dir, "auto")
	if err == nil {
		t.Fatal("expected error for no manifest")
	}
}
