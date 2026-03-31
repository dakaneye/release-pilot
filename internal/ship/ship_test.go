package ship_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dakaneye/release-pilot/internal/ship"
)

func TestShipMissingAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "fake")

	err := ship.Run(t.TempDir(), ship.Options{})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("error should mention ANTHROPIC_API_KEY: %v", err)
	}
}

func TestShipMissingGitHubToken(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "fake")
	t.Setenv("GITHUB_TOKEN", "")

	err := ship.Run(t.TempDir(), ship.Options{})
	if err == nil {
		t.Fatal("expected error for missing GitHub token")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("error should mention GITHUB_TOKEN: %v", err)
	}
}

func TestShipNoManifest(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "fake")
	t.Setenv("GITHUB_TOKEN", "fake")

	dir := t.TempDir()
	initGitRepo(t, dir)

	err := ship.Run(dir, ship.Options{})
	if err == nil {
		t.Fatal("expected error for no manifest")
	}
	// The error might be from detect step wrapped in pipeline step error
	errStr := err.Error()
	if !strings.Contains(errStr, "ecosystem") && !strings.Contains(errStr, "detect") {
		t.Errorf("error should mention ecosystem detection: %v", err)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgsign", "false"},
		{"git", "remote", "add", "origin", "https://github.com/test/repo.git"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := exec.Command("git", "-C", dir, "add", ".")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	run = exec.Command("git", "-C", dir, "commit", "-m", "init")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
}
