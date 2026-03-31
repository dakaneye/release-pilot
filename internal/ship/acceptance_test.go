//go:build acceptance

package ship_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcceptanceNodeRelease(t *testing.T) {
	// Find project root (walk up to go.mod)
	root := projectRoot(t)

	// Build the binary
	binary := filepath.Join(t.TempDir(), "release-pilot")
	build := exec.Command("go", "build", "-o", binary, "./cmd/release-pilot")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}

	// Set up a git repo with package.json
	dir := t.TempDir()
	setupNodeRepo(t, dir)

	// Stub Claude API
	claudeStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": `{"bump":"minor","notes":"## Features\n\n- Add cool feature ([#1](https://github.com/test/repo/pull/1))"}`,
				},
			},
		})
	}))
	defer claudeStub.Close()

	// Stub GitHub API
	ghStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/pulls"):
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"number":    1,
					"title":     "feat: cool feature",
					"body":      "Adds a cool feature",
					"merged_at": "2026-03-15T10:00:00Z",
					"state":     "closed",
				},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{})
		}
	}))
	defer ghStub.Close()

	// Run release-pilot ship --dry-run
	cmd := exec.Command(binary, "ship", "--dry-run")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"ANTHROPIC_API_KEY=test-key",
		"GITHUB_TOKEN=test-token",
		"ANTHROPIC_BASE_URL="+claudeStub.URL,
		"GITHUB_API_URL="+ghStub.URL,
		"RUNNER_TEMP="+t.TempDir(),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ship --dry-run failed: %s\n%s", err, out)
	}

	output := string(out)
	// The version should be bumped from 1.0.0 to 1.1.0 (minor bump)
	if !strings.Contains(output, "v1.1.0") {
		t.Errorf("output should mention new version v1.1.0:\n%s", output)
	}
	if !strings.Contains(output, "dry-run") {
		t.Errorf("output should mention dry-run:\n%s", output)
	}
}

func TestAcceptanceGoReleaseWithTagPrefix(t *testing.T) {
	root := projectRoot(t)

	binary := filepath.Join(t.TempDir(), "release-pilot")
	build := exec.Command("go", "build", "-o", binary, "./cmd/release-pilot")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}

	dir := t.TempDir()
	setupGoMonorepo(t, dir)

	claudeStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": `{"bump":"patch","notes":"## Fixes\n\n- Fix review-code bug"}`,
				},
			},
		})
	}))
	defer claudeStub.Close()

	ghStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/pulls"):
			json.NewEncoder(w).Encode([]map[string]any{})
		default:
			json.NewEncoder(w).Encode(map[string]any{})
		}
	}))
	defer ghStub.Close()

	cmd := exec.Command(binary, "ship", "--dry-run",
		"--tag-prefix", "review-code/",
		"--sub-dir", "review-code/",
	)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"ANTHROPIC_API_KEY=test-key",
		"GITHUB_TOKEN=test-token",
		"ANTHROPIC_BASE_URL="+claudeStub.URL,
		"GITHUB_API_URL="+ghStub.URL,
		"RUNNER_TEMP="+t.TempDir(),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ship --dry-run with prefix failed: %s\n%s", err, out)
	}

	output := string(out)
	// Should pick up review-code/v1.0.0 as latest tag and bump to review-code/v1.0.1
	if !strings.Contains(output, "review-code/v1.0.1") {
		t.Errorf("output should mention prefixed version review-code/v1.0.1:\n%s", output)
	}
	if !strings.Contains(output, "review-code/v1.0.0") {
		t.Errorf("output should mention latest tag review-code/v1.0.0:\n%s", output)
	}
}

func setupGoMonorepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "tag.gpgsign", "false"},
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

	// Create go.mod at root
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/test/repo\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create sub-project directory
	subdir := filepath.Join(dir, "review-code")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "init monorepo")

	// Create tags: an unprefixed one and a prefixed one
	gitRun(t, dir, "tag", "-a", "v0.1.0", "-m", "v0.1.0")
	gitRun(t, dir, "tag", "-a", "review-code/v1.0.0", "-m", "review-code/v1.0.0")

	// Add a change in the sub-project after the tag
	if err := os.WriteFile(filepath.Join(subdir, "fix.go"), []byte("package main\n// fix\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "fix: review-code bug")
}

func setupNodeRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "tag.gpgsign", "false"},
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

	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "name": "test-app",
  "version": "1.0.0"
}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "init")
	gitRun(t, dir, "tag", "v1.0.0")

	// Add a change after the tag
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte("console.log('hello')"), 0o644); err != nil {
		t.Fatalf("write index.js: %v", err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "feat: cool feature")
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s\n%s", strings.Join(args, " "), err, out)
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}
