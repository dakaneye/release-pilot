package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dakaneye/release-pilot/internal/config"
)

func TestDefaults(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ecosystem != "auto" {
		t.Errorf("expected ecosystem=auto, got %s", cfg.Ecosystem)
	}
	if cfg.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model=claude-sonnet-4-6, got %s", cfg.Model)
	}
	if cfg.Notes.IncludeDiffs {
		t.Error("expected include-diffs=false by default")
	}
	if cfg.GitHub.Draft {
		t.Error("expected draft=false by default")
	}
	if cfg.GitHub.Prerelease {
		t.Error("expected prerelease=false by default")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".release-pilot.yaml")
	err := os.WriteFile(cfgPath, []byte(`
ecosystem: python
model: claude-opus-4-6
notes:
  include-diffs: true
github:
  draft: true
  prerelease: true
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ecosystem != "python" {
		t.Errorf("expected ecosystem=python, got %s", cfg.Ecosystem)
	}
	if cfg.Model != "claude-opus-4-6" {
		t.Errorf("expected model=claude-opus-4-6, got %s", cfg.Model)
	}
	if !cfg.Notes.IncludeDiffs {
		t.Error("expected include-diffs=true")
	}
	if !cfg.GitHub.Draft {
		t.Error("expected draft=true")
	}
}

func TestEnvVarOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".release-pilot.yaml")
	err := os.WriteFile(cfgPath, []byte(`model: claude-sonnet-4-6`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("RELEASE_PILOT_MODEL", "claude-haiku-4-5")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "claude-haiku-4-5" {
		t.Errorf("expected model=claude-haiku-4-5, got %s", cfg.Model)
	}
}

func TestMissingFileUsesDefaults(t *testing.T) {
	cfg, err := config.Load("/nonexistent/.release-pilot.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ecosystem != "auto" {
		t.Errorf("expected defaults when file missing, got ecosystem=%s", cfg.Ecosystem)
	}
}

func TestInvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".release-pilot.yaml")
	if err := os.WriteFile(cfgPath, []byte(":\n  :\n    : [invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
