package detect_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dakaneye/release-pilot/internal/detect"
)

func TestDetectGo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example"), 0o644)

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
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example"), 0o644)
	os.WriteFile(filepath.Join(dir, ".goreleaser.yaml"), []byte("builds:"), 0o644)

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
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]"), 0o644)

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
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)

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
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example"), 0o644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)

	_, err := detect.Ecosystem(dir, "auto")
	if err == nil {
		t.Fatal("expected error for ambiguous ecosystem")
	}
}

func TestDetectAmbiguousWithOverride(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example"), 0o644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)

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
