//go:build e2e

// Package e2e contains end-to-end tests that make real Claude API calls.
// Run with: go test ./test/e2e/... -tags e2e -v
//
// Requires:
//   - ANTHROPIC_API_KEY set (real key for Claude API calls)
//   - release-pilot binary built (the test builds it automatically)
//
// These tests are idempotent — they create fresh temp directories each run
// and stub the GitHub API (since test repos aren't on GitHub).
package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestE2EGoApp(t *testing.T) {
	binary := buildBinary(t)
	dir := setupRepo(t, "go-app", "v0.1.0", []change{
		{file: "math.go", content: "package main\n\nfunc Subtract(a, b int) int { return a - b }\n", msg: "feat: add Subtract function"},
		{file: "main.go", content: readFixture(t, "go-app/main.go") + "\n// Fixed off-by-one\n", msg: "fix: correct off-by-one in Add"},
	})

	ghStub := stubGitHubAPI(t, []stubPR{
		{Number: 1, Title: "feat: add Subtract function", Body: "Adds a new Subtract function to the math library for basic arithmetic operations."},
		{Number: 2, Title: "fix: correct off-by-one in Add", Body: "The Add function had an edge case with negative numbers that returned incorrect results."},
	})
	defer ghStub.Close()

	result := runShip(t, binary, dir, ghStub.URL)
	assertContains(t, result, "v0.2.0", "expected minor bump to v0.2.0")
	assertContains(t, result, "dry-run", "expected dry-run marker")
	assertContains(t, result, "Subtract", "expected notes to mention Subtract")

	t.Logf("--- Go App Output ---\n%s", result)
}

func TestE2EPythonApp(t *testing.T) {
	binary := buildBinary(t)
	dir := setupRepo(t, "python-app", "v0.1.0", []change{
		{file: "src/validators.py", content: "def validate_email(email: str) -> bool:\n    return '@' in email and '.' in email.split('@')[1]\n", msg: "feat: add email validation"},
		{file: "src/app.py", content: readFixture(t, "python-app/src/app.py") + "\n\ndef greet_formal(name: str, title: str) -> str:\n    return f\"Good day, {title} {name}!\"\n", msg: "feat: add formal greeting"},
	})

	ghStub := stubGitHubAPI(t, []stubPR{
		{Number: 5, Title: "feat: add email validation", Body: "Adds a simple email validation utility for form input checking."},
		{Number: 6, Title: "feat: add formal greeting", Body: "Adds a `greet_formal` function that accepts a title for more formal interactions."},
	})
	defer ghStub.Close()

	result := runShip(t, binary, dir, ghStub.URL)
	assertContains(t, result, "v0.2.0", "expected minor bump to v0.2.0")
	assertContains(t, result, "email", "expected notes to mention email validation")

	t.Logf("--- Python App Output ---\n%s", result)
}

func TestE2ENodeApp(t *testing.T) {
	binary := buildBinary(t)
	dir := setupRepo(t, "node-app", "v2.0.0", []change{
		{file: "index.js", content: readFixture(t, "node-app/index.js") + "\n\nexport function truncate(str, len) {\n  if (str.length <= len) return str;\n  return str.slice(0, len) + '...';\n}\n", msg: "feat: add truncate utility"},
		{file: "CHANGELOG.md", content: "# Changelog\n\n## Unreleased\n- Updated eslint\n", msg: "chore: update deps"},
	})

	ghStub := stubGitHubAPI(t, []stubPR{
		{Number: 20, Title: "feat: add truncate utility", Body: "Adds a string truncation helper for display text."},
		{Number: 21, Title: "chore(deps): bump eslint from 8.0 to 9.0", Body: "Bumps eslint. Automated by Dependabot."},
	})
	defer ghStub.Close()

	result := runShip(t, binary, dir, ghStub.URL)
	assertContains(t, result, "v2.1.0", "expected minor bump to v2.1.0")
	assertContains(t, result, "truncate", "expected notes to mention truncate")

	t.Logf("--- Node App Output ---\n%s", result)
}

// --- helpers ---

type change struct {
	file    string
	content string
	msg     string
}

type stubPR struct {
	Number int
	Title  string
	Body   string
}

func buildBinary(t *testing.T) string {
	t.Helper()
	root := projectRoot(t)
	binary := filepath.Join(t.TempDir(), "release-pilot")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/release-pilot")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %s\n%s", err, out)
	}
	return binary
}

func projectRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root")
		}
		dir = parent
	}
}

func readFixture(t *testing.T, rel string) string {
	t.Helper()
	root := projectRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "testdata", "repos", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func setupRepo(t *testing.T, fixture string, initialTag string, changes []change) string {
	t.Helper()
	root := projectRoot(t)
	src := filepath.Join(root, "testdata", "repos", fixture)
	dir := t.TempDir()

	// Copy fixture files
	copyDir(t, src, dir)

	// Init git
	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@release-pilot.dev")
	gitRun(t, dir, "config", "user.name", "release-pilot-test")
	gitRun(t, dir, "config", "tag.gpgsign", "false")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	gitRun(t, dir, "remote", "add", "origin", fmt.Sprintf("https://github.com/test/%s.git", fixture))
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "initial commit")
	gitRun(t, dir, "tag", initialTag)

	// Apply changes after the tag
	for _, c := range changes {
		path := filepath.Join(dir, c.file)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(c.content), 0o644)
		gitRun(t, dir, "add", "-A")
		gitRun(t, dir, "commit", "-m", c.msg)
	}

	return dir
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			os.MkdirAll(dstPath, 0o755)
			copyDir(t, srcPath, dstPath)
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				t.Fatal(err)
			}
			os.WriteFile(dstPath, data, 0o644)
		}
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s\n%s", args, err, out)
	}
}

func stubGitHubAPI(t *testing.T, prs []stubPR) *httptest.Server {
	t.Helper()
	mergedAt := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/pulls") {
			var out []map[string]any
			for _, pr := range prs {
				out = append(out, map[string]any{
					"number":    pr.Number,
					"title":     pr.Title,
					"body":      pr.Body,
					"merged_at": mergedAt,
					"state":     "closed",
				})
			}
			json.NewEncoder(w).Encode(out)
			return
		}
		// Default: return empty object for any other endpoint
		json.NewEncoder(w).Encode(map[string]any{})
	}))
}

func runShip(t *testing.T, binary, dir, ghStubURL string) string {
	t.Helper()

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set — skipping E2E test")
	}

	cmd := exec.Command(binary, "ship", "--dry-run")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"ANTHROPIC_API_KEY="+apiKey,
		"GITHUB_TOKEN=fake-token",
		"GITHUB_API_URL="+ghStubURL,
		"RUNNER_TEMP="+t.TempDir(),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release-pilot ship --dry-run failed: %s\n%s", err, out)
	}
	return string(out)
}

func assertContains(t *testing.T, output, substr, msg string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("%s — %q not found in output:\n%s", msg, substr, output)
	}
}
