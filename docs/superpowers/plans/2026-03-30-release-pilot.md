# release-pilot v0.1.0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI that orchestrates release workflows (version bump, AI release notes, GitHub release, cosign signing) across Go, Python, and Node.js ecosystems.

**Architecture:** Pipeline of 5 idempotent steps (detect, bump, notes, release, sign) behind a single `ship` command. Bump and notes share one Claude API call. State file tracks progress for idempotent re-runs. Config loaded from YAML file with env var and flag overrides.

**Tech Stack:** Go 1.22+, cobra, github.com/anthropics/anthropic-sdk-go, github.com/google/go-github/v76, gopkg.in/yaml.v3

---

## File Structure

```
release-pilot/
  cmd/
    release-pilot/
      main.go                    # entry point, cobra root command
  internal/
    config/
      config.go                  # config loading, merging, defaults
      config_test.go
    detect/
      detect.go                  # ecosystem detection from manifest files
      detect_test.go
    git/
      git.go                     # git tag listing, commit log, tag creation, push
      git_test.go
    github/
      client.go                  # GitHub API: list merged PRs, create/edit releases
      client_test.go
    version/
      version.go                 # semver parsing, manifest read/write per ecosystem
      version_test.go
    claude/
      claude.go                  # prompt construction, API call, response parsing
      claude_test.go
      prompt.go                  # system and user prompt templates
      prompt_test.go
    pipeline/
      state.go                   # state file read/write for idempotency
      state_test.go
      pipeline.go                # step orchestration
      pipeline_test.go
    sign/
      sign.go                    # cosign keyless signing invocation
      sign_test.go
    ship/
      ship.go                    # ship command wiring (connects CLI flags to pipeline)
      ship_test.go
  action/
    action.yml                   # simple mode GitHub Action
    setup/
      action.yml                 # advanced mode (install-only)
    install.sh                   # binary installer script
  testdata/
    go-repo/                     # fixture: minimal Go repo
    python-repo/                 # fixture: minimal Python repo
    node-repo/                   # fixture: minimal Node repo
    claude-response.json         # fixture: realistic Claude API response
  .goreleaser.yaml               # for releasing release-pilot itself
  .release-pilot.yaml            # dogfood: release-pilot's own config
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/release-pilot/main.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/samueldacanay/dev/personal/release-pilot
go mod init github.com/dakaneye/release-pilot
```

- [ ] **Step 2: Install cobra**

```bash
go get github.com/spf13/cobra@latest
```

- [ ] **Step 3: Write main.go with root command**

Create `cmd/release-pilot/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "release-pilot",
		Short: "Orchestrate releases with AI-powered release notes",
	}

	root.AddCommand(versionCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}
}
```

- [ ] **Step 4: Verify it compiles and runs**

```bash
go run ./cmd/release-pilot version
```

Expected: `dev`

- [ ] **Step 5: Initialize git repo and commit**

```bash
git init
git add go.mod go.sum cmd/
git commit -m "feat: scaffold project with cobra root command"
```

---

### Task 2: Config Loading

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write tests for config loading**

Create `internal/config/config_test.go`:

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dakaneye/release-pilot/internal/config"
)

func TestDefaults(t *testing.T) {
	cfg := config.Load("")
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

	cfg := config.Load(cfgPath)
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
	cfg := config.Load(cfgPath)
	if cfg.Model != "claude-haiku-4-5" {
		t.Errorf("expected model=claude-haiku-4-5, got %s", cfg.Model)
	}
}

func TestMissingFileUsesDefaults(t *testing.T) {
	cfg := config.Load("/nonexistent/.release-pilot.yaml")
	if cfg.Ecosystem != "auto" {
		t.Errorf("expected defaults when file missing, got ecosystem=%s", cfg.Ecosystem)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/...
```

Expected: compilation error — `config` package doesn't exist yet.

- [ ] **Step 3: Implement config loading**

Create `internal/config/config.go`:

```go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Ecosystem string       `yaml:"ecosystem"`
	Model     string       `yaml:"model"`
	Notes     NotesConfig  `yaml:"notes"`
	GitHub    GitHubConfig `yaml:"github"`
}

type NotesConfig struct {
	IncludeDiffs bool `yaml:"include-diffs"`
}

type GitHubConfig struct {
	Draft      bool `yaml:"draft"`
	Prerelease bool `yaml:"prerelease"`
}

func defaults() Config {
	return Config{
		Ecosystem: "auto",
		Model:     "claude-sonnet-4-6",
	}
}

func Load(path string) Config {
	cfg := defaults()

	if path == "" {
		path = ".release-pilot.yaml"
	}

	data, err := os.ReadFile(path)
	if err == nil {
		_ = yaml.Unmarshal(data, &cfg)
	}

	if env := os.Getenv("RELEASE_PILOT_MODEL"); env != "" {
		cfg.Model = env
	}

	return cfg
}
```

- [ ] **Step 4: Install yaml dependency and run tests**

```bash
go get gopkg.in/yaml.v3
go test ./internal/config/... -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat(config): add config loading with file, env var, and defaults"
```

---

### Task 3: Ecosystem Detection

**Files:**
- Create: `internal/detect/detect.go`
- Create: `internal/detect/detect_test.go`

- [ ] **Step 1: Write tests for ecosystem detection**

Create `internal/detect/detect_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/detect/...
```

Expected: compilation error.

- [ ] **Step 3: Implement ecosystem detection**

Create `internal/detect/detect.go`:

```go
package detect

import (
	"fmt"
	"os"
	"path/filepath"
)

type Result struct {
	Name           string // "go", "python", "node"
	HasGoreleaser  bool
	ManifestPath   string // path to the manifest file (empty for Go)
}

var manifests = map[string]string{
	"go.mod":          "go",
	"pyproject.toml":  "python",
	"package.json":    "node",
}

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
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/detect/... -v
```

Expected: all 7 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/detect/
git commit -m "feat(detect): add ecosystem detection from manifest files"
```

---

### Task 4: Git Operations

**Files:**
- Create: `internal/git/git.go`
- Create: `internal/git/git_test.go`

- [ ] **Step 1: Write tests for git operations**

Create `internal/git/git_test.go`:

```go
package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dakaneye/release-pilot/internal/git"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0o644)
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "initial")
	return dir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %s\n%s", name, args, err, out)
	}
}

func TestLatestTag(t *testing.T) {
	dir := initRepo(t)
	run(t, dir, "git", "tag", "v0.1.0")
	run(t, dir, "git", "tag", "v0.2.0")

	tag, err := git.LatestTag(dir)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v0.2.0" {
		t.Errorf("expected v0.2.0, got %s", tag)
	}
}

func TestLatestTagNoTags(t *testing.T) {
	dir := initRepo(t)

	_, err := git.LatestTag(dir)
	if err == nil {
		t.Fatal("expected error when no tags exist")
	}
}

func TestCommitsSince(t *testing.T) {
	dir := initRepo(t)
	run(t, dir, "git", "tag", "v0.1.0")

	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644)
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "feat: add feature A")

	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b"), 0o644)
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "fix: fix bug B")

	commits, err := git.CommitsSince(dir, "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
}

func TestCreateTag(t *testing.T) {
	dir := initRepo(t)

	err := git.CreateTag(dir, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	tag, err := git.LatestTag(dir)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", tag)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/git/...
```

Expected: compilation error.

- [ ] **Step 3: Implement git operations**

Create `internal/git/git.go`:

```go
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

type Commit struct {
	Hash    string
	Subject string
}

func LatestTag(dir string) (string, error) {
	out, err := runGit(dir, "describe", "--tags", "--abbrev=0")
	if err != nil {
		return "", fmt.Errorf("no tags found: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func CommitsSince(dir string, tag string) ([]Commit, error) {
	out, err := runGit(dir, "log", tag+"..HEAD", "--pretty=format:%H %s")
	if err != nil {
		return nil, fmt.Errorf("git log since %s: %w", tag, err)
	}

	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	var commits []Commit
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		commits = append(commits, Commit{Hash: parts[0], Subject: parts[1]})
	}
	return commits, nil
}

func CreateTag(dir string, tag string) error {
	_, err := runGit(dir, "tag", tag)
	if err != nil {
		return fmt.Errorf("create tag %s: %w", tag, err)
	}
	return nil
}

func PushTag(dir string, tag string) error {
	_, err := runGit(dir, "push", "origin", tag)
	if err != nil {
		return fmt.Errorf("push tag %s: %w", tag, err)
	}
	return nil
}

func CommitAll(dir string, message string) error {
	if _, err := runGit(dir, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if _, err := runGit(dir, "commit", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

func Push(dir string) error {
	_, err := runGit(dir, "push")
	if err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/git/... -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat(git): add tag listing, commit log, tag creation"
```

---

### Task 5: Version Parsing and Manifest Updates

**Files:**
- Create: `internal/version/version.go`
- Create: `internal/version/version_test.go`

- [ ] **Step 1: Write tests for version parsing and bumping**

Create `internal/version/version_test.go`:

```go
package version_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dakaneye/release-pilot/internal/version"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		tag     string
		major   int
		minor   int
		patch   int
		wantErr bool
	}{
		{"v1.2.3", 1, 2, 3, false},
		{"1.2.3", 1, 2, 3, false},
		{"v0.0.1", 0, 0, 1, false},
		{"invalid", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			v, err := version.ParseTag(tt.tag)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if v.Major != tt.major || v.Minor != tt.minor || v.Patch != tt.patch {
				t.Errorf("expected %d.%d.%d, got %d.%d.%d", tt.major, tt.minor, tt.patch, v.Major, v.Minor, v.Patch)
			}
		})
	}
}

func TestBump(t *testing.T) {
	v := version.Semver{Major: 1, Minor: 2, Patch: 3}

	if got := v.Bump("major"); got.String() != "1.2.3" || got.Tag() != "v1.2.3" {
		// let me redo this
	}

	major := v.Bump("major")
	if major.String() != "2.0.0" {
		t.Errorf("major bump: expected 2.0.0, got %s", major.String())
	}

	minor := v.Bump("minor")
	if minor.String() != "1.3.0" {
		t.Errorf("minor bump: expected 1.3.0, got %s", minor.String())
	}

	patch := v.Bump("patch")
	if patch.String() != "1.2.4" {
		t.Errorf("patch bump: expected 1.2.4, got %s", patch.String())
	}
}

func TestBumpTag(t *testing.T) {
	v := version.Semver{Major: 1, Minor: 2, Patch: 3}
	bumped := v.Bump("minor")
	if bumped.Tag() != "v1.3.0" {
		t.Errorf("expected v1.3.0, got %s", bumped.Tag())
	}
}

func TestUpdatePackageJSON(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "package.json")
	os.WriteFile(pkg, []byte(`{
  "name": "my-app",
  "version": "1.0.0",
  "description": "test"
}`), 0o644)

	err := version.UpdateManifest(pkg, "node", "1.1.0")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(pkg)
	content := string(data)
	if !contains(content, `"version": "1.1.0"`) {
		t.Errorf("expected version 1.1.0 in:\n%s", content)
	}
	if !contains(content, `"name": "my-app"`) {
		t.Error("other fields should be preserved")
	}
}

func TestUpdatePyprojectTOML(t *testing.T) {
	dir := t.TempDir()
	pyproject := filepath.Join(dir, "pyproject.toml")
	os.WriteFile(pyproject, []byte(`[project]
name = "my-app"
version = "1.0.0"
`), 0o644)

	err := version.UpdateManifest(pyproject, "python", "1.1.0")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(pyproject)
	content := string(data)
	if !contains(content, `version = "1.1.0"`) {
		t.Errorf("expected version 1.1.0 in:\n%s", content)
	}
}

func TestUpdatePackageJSONWithLockfile(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "package.json")
	lock := filepath.Join(dir, "package-lock.json")
	os.WriteFile(pkg, []byte(`{
  "name": "my-app",
  "version": "1.0.0"
}`), 0o644)
	os.WriteFile(lock, []byte(`{
  "name": "my-app",
  "version": "1.0.0",
  "lockfileVersion": 3
}`), 0o644)

	err := version.UpdateManifest(pkg, "node", "1.1.0")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(lock)
	content := string(data)
	if !contains(content, `"version": "1.1.0"`) {
		t.Errorf("expected version 1.1.0 in lock file:\n%s", content)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/version/...
```

Expected: compilation error.

- [ ] **Step 3: Implement version parsing and manifest updates**

Create `internal/version/version.go`:

```go
package version

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Semver struct {
	Major int
	Minor int
	Patch int
}

func (v Semver) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Semver) Tag() string {
	return "v" + v.String()
}

func (v Semver) Bump(level string) Semver {
	switch level {
	case "major":
		return Semver{Major: v.Major + 1}
	case "minor":
		return Semver{Major: v.Major, Minor: v.Minor + 1}
	case "patch":
		return Semver{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
	default:
		return v
	}
}

var tagPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

func ParseTag(tag string) (Semver, error) {
	matches := tagPattern.FindStringSubmatch(tag)
	if matches == nil {
		return Semver{}, fmt.Errorf("invalid semver tag: %s", tag)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return Semver{Major: major, Minor: minor, Patch: patch}, nil
}

func UpdateManifest(path string, ecosystem string, newVersion string) error {
	switch ecosystem {
	case "node":
		return updateJSON(path, newVersion)
	case "python":
		return updateTOML(path, newVersion)
	default:
		return nil // Go uses tags only
	}
}

func updateJSON(path string, newVersion string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	obj["version"] = newVersion

	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}

	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	// Update lockfile if present
	dir := filepath.Dir(path)
	lockPath := filepath.Join(dir, "package-lock.json")
	if lockData, err := os.ReadFile(lockPath); err == nil {
		var lockObj map[string]any
		if err := json.Unmarshal(lockData, &lockObj); err == nil {
			lockObj["version"] = newVersion
			if lockOut, err := json.MarshalIndent(lockObj, "", "  "); err == nil {
				os.WriteFile(lockPath, append(lockOut, '\n'), 0o644)
			}
		}
	}

	return nil
}

func updateTOML(path string, newVersion string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	// Simple regex replacement for version field in pyproject.toml
	// Handles both [project].version and [tool.poetry].version
	re := regexp.MustCompile(`(?m)^(version\s*=\s*")([^"]+)(")`)
	content := string(data)

	if !re.MatchString(content) {
		return fmt.Errorf("no version field found in %s", path)
	}

	updated := re.ReplaceAllString(content, "${1}"+newVersion+"${3}")

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// ReadVersion reads the current version from a manifest file.
func ReadVersion(path string, ecosystem string) (string, error) {
	switch ecosystem {
	case "node":
		return readJSONVersion(path)
	case "python":
		return readTOMLVersion(path)
	default:
		return "", fmt.Errorf("go versions come from git tags, not manifests")
	}
}

func readJSONVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	v, ok := obj["version"].(string)
	if !ok {
		return "", fmt.Errorf("no version field in %s", path)
	}
	return v, nil
}

func readTOMLVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	re := regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(string(data))
	if matches == nil {
		return "", fmt.Errorf("no version field in %s", path)
	}
	return matches[1], nil
}

// Unused import guard
var _ = strings.TrimSpace
```

Wait — that last line is dead code. Remove it. The `strings` import isn't needed.

Create `internal/version/version.go` (corrected — no `strings` import):

```go
package version

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

type Semver struct {
	Major int
	Minor int
	Patch int
}

func (v Semver) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Semver) Tag() string {
	return "v" + v.String()
}

func (v Semver) Bump(level string) Semver {
	switch level {
	case "major":
		return Semver{Major: v.Major + 1}
	case "minor":
		return Semver{Major: v.Major, Minor: v.Minor + 1}
	case "patch":
		return Semver{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
	default:
		return v
	}
}

var tagPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

func ParseTag(tag string) (Semver, error) {
	matches := tagPattern.FindStringSubmatch(tag)
	if matches == nil {
		return Semver{}, fmt.Errorf("invalid semver tag: %s", tag)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return Semver{Major: major, Minor: minor, Patch: patch}, nil
}

func UpdateManifest(path string, ecosystem string, newVersion string) error {
	switch ecosystem {
	case "node":
		return updateJSON(path, newVersion)
	case "python":
		return updateTOML(path, newVersion)
	default:
		return nil
	}
}

func updateJSON(path string, newVersion string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	obj["version"] = newVersion

	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}

	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	lockPath := filepath.Join(dir, "package-lock.json")
	if lockData, err := os.ReadFile(lockPath); err == nil {
		var lockObj map[string]any
		if err := json.Unmarshal(lockData, &lockObj); err == nil {
			lockObj["version"] = newVersion
			if lockOut, err := json.MarshalIndent(lockObj, "", "  "); err == nil {
				os.WriteFile(lockPath, append(lockOut, '\n'), 0o644)
			}
		}
	}

	return nil
}

func updateTOML(path string, newVersion string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	re := regexp.MustCompile(`(?m)^(version\s*=\s*")([^"]+)(")`)
	content := string(data)

	if !re.MatchString(content) {
		return fmt.Errorf("no version field found in %s", path)
	}

	updated := re.ReplaceAllString(content, "${1}"+newVersion+"${3}")

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func ReadVersion(path string, ecosystem string) (string, error) {
	switch ecosystem {
	case "node":
		return readJSONVersion(path)
	case "python":
		return readTOMLVersion(path)
	default:
		return "", fmt.Errorf("go versions come from git tags, not manifests")
	}
}

func readJSONVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	v, ok := obj["version"].(string)
	if !ok {
		return "", fmt.Errorf("no version field in %s", path)
	}
	return v, nil
}

func readTOMLVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	re := regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(string(data))
	if matches == nil {
		return "", fmt.Errorf("no version field in %s", path)
	}
	return matches[1], nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/version/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/version/
git commit -m "feat(version): add semver parsing, bumping, and manifest updates"
```

---

### Task 6: GitHub API Client

**Files:**
- Create: `internal/github/client.go`
- Create: `internal/github/client_test.go`

- [ ] **Step 1: Write tests with HTTP stub server**

Create `internal/github/client_test.go`:

```go
package github_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gh "github.com/dakaneye/release-pilot/internal/github"
)

func TestListMergedPRs(t *testing.T) {
	mergedAt := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	prs := []map[string]any{
		{
			"number":    1,
			"title":     "feat: add feature A",
			"body":      "This adds feature A",
			"merged_at": mergedAt.Format(time.RFC3339),
			"state":     "closed",
		},
		{
			"number":    2,
			"title":     "closed but not merged",
			"body":      "This was closed",
			"merged_at": nil,
			"state":     "closed",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/pulls" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(prs)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := gh.NewClient("fake-token", srv.URL)
	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	result, err := client.MergedPRsSince("owner", "repo", since)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 merged PR, got %d", len(result))
	}
	if result[0].Number != 1 {
		t.Errorf("expected PR #1, got #%d", result[0].Number)
	}
	if result[0].Title != "feat: add feature A" {
		t.Errorf("unexpected title: %s", result[0].Title)
	}
}

func TestCreateRelease(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/releases" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&received)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"id":       123,
				"html_url": "https://github.com/owner/repo/releases/tag/v1.0.0",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := gh.NewClient("fake-token", srv.URL)
	url, err := client.CreateRelease("owner", "repo", gh.ReleaseParams{
		Tag:        "v1.0.0",
		Name:       "v1.0.0",
		Body:       "## What's new\n- Feature A",
		Draft:      false,
		Prerelease: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://github.com/owner/repo/releases/tag/v1.0.0" {
		t.Errorf("unexpected URL: %s", url)
	}
	if received["tag_name"] != "v1.0.0" {
		t.Errorf("expected tag v1.0.0, got %v", received["tag_name"])
	}
}

func TestEditReleaseBody(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/releases/123" && r.Method == "PATCH" {
			json.NewDecoder(r.Body).Decode(&received)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": 123})
			return
		}
		if r.URL.Path == "/repos/owner/repo/releases/tags/v1.0.0" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": 123})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := gh.NewClient("fake-token", srv.URL)
	err := client.EditReleaseBody("owner", "repo", "v1.0.0", "Updated notes")
	if err != nil {
		t.Fatal(err)
	}
	if received["body"] != "Updated notes" {
		t.Errorf("expected updated body, got %v", received["body"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/github/...
```

Expected: compilation error.

- [ ] **Step 3: Implement GitHub client**

Create `internal/github/client.go`:

```go
package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

type PR struct {
	Number int
	Title  string
	Body   string
}

type ReleaseParams struct {
	Tag        string
	Name       string
	Body       string
	Draft      bool
	Prerelease bool
}

func NewClient(token string, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &Client{
		token:   token,
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) MergedPRsSince(owner, repo string, since time.Time) ([]PR, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?state=closed&sort=updated&direction=desc&per_page=100", c.baseURL, owner, repo)

	body, err := c.get(url)
	if err != nil {
		return nil, fmt.Errorf("list PRs: %w", err)
	}

	var raw []struct {
		Number   int        `json:"number"`
		Title    string     `json:"title"`
		Body     string     `json:"body"`
		MergedAt *time.Time `json:"merged_at"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse PR response: %w", err)
	}

	var prs []PR
	for _, r := range raw {
		if r.MergedAt != nil && r.MergedAt.After(since) {
			prs = append(prs, PR{Number: r.Number, Title: r.Title, Body: r.Body})
		}
	}
	return prs, nil
}

func (c *Client) CreateRelease(owner, repo string, params ReleaseParams) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases", c.baseURL, owner, repo)

	payload := map[string]any{
		"tag_name":   params.Tag,
		"name":       params.Name,
		"body":       params.Body,
		"draft":      params.Draft,
		"prerelease": params.Prerelease,
	}

	body, err := c.post(url, payload)
	if err != nil {
		return "", fmt.Errorf("create release: %w", err)
	}

	var result struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse release response: %w", err)
	}
	return result.HTMLURL, nil
}

func (c *Client) EditReleaseBody(owner, repo, tag, newBody string) error {
	// First, get the release by tag
	getURL := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", c.baseURL, owner, repo, tag)
	body, err := c.get(getURL)
	if err != nil {
		return fmt.Errorf("get release by tag %s: %w", tag, err)
	}

	var release struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return fmt.Errorf("parse release: %w", err)
	}

	// Then patch the body
	patchURL := fmt.Sprintf("%s/repos/%s/%s/releases/%d", c.baseURL, owner, repo, release.ID)
	_, err = c.patch(patchURL, map[string]any{"body": newBody})
	if err != nil {
		return fmt.Errorf("edit release body: %w", err)
	}
	return nil
}

func (c *Client) get(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) post(url string, payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) patch(url string, payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitHub API %s %s returned %d: %s", req.Method, req.URL.Path, resp.StatusCode, string(body))
	}

	return body, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/github/... -v
```

Expected: all 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/github/
git commit -m "feat(github): add PR listing, release creation, and release editing"
```

---

### Task 7: Claude API Integration — Prompt Construction

**Files:**
- Create: `internal/claude/prompt.go`
- Create: `internal/claude/prompt_test.go`
- Create: `testdata/claude-response.json`

- [ ] **Step 1: Write tests for prompt construction**

Create `internal/claude/prompt_test.go`:

```go
package claude_test

import (
	"strings"
	"testing"

	"github.com/dakaneye/release-pilot/internal/claude"
	gh "github.com/dakaneye/release-pilot/internal/github"
	"github.com/dakaneye/release-pilot/internal/git"
)

func TestBuildPromptIncludesPRs(t *testing.T) {
	input := claude.PromptInput{
		RepoOwner:  "owner",
		RepoName:   "repo",
		CurrentTag: "v1.0.0",
		PRs: []gh.PR{
			{Number: 10, Title: "feat: add search", Body: "Adds full-text search to the API"},
			{Number: 11, Title: "fix: null pointer in auth", Body: "Fixes crash when token is nil"},
		},
		Commits: []git.Commit{
			{Hash: "abc123", Subject: "feat: add search"},
			{Hash: "def456", Subject: "fix: null pointer in auth"},
			{Hash: "ghi789", Subject: "chore: update deps"},
		},
	}

	prompt := claude.BuildUserPrompt(input)

	if !strings.Contains(prompt, "#10") {
		t.Error("prompt should contain PR number #10")
	}
	if !strings.Contains(prompt, "feat: add search") {
		t.Error("prompt should contain PR title")
	}
	if !strings.Contains(prompt, "Adds full-text search") {
		t.Error("prompt should contain PR body")
	}
	if !strings.Contains(prompt, "chore: update deps") {
		t.Error("prompt should contain commit messages")
	}
	if !strings.Contains(prompt, "v1.0.0") {
		t.Error("prompt should contain current version")
	}
}

func TestBuildPromptNoPRs(t *testing.T) {
	input := claude.PromptInput{
		RepoOwner:  "owner",
		RepoName:   "repo",
		CurrentTag: "v0.1.0",
		Commits: []git.Commit{
			{Hash: "abc123", Subject: "feat: initial release"},
		},
	}

	prompt := claude.BuildUserPrompt(input)

	if !strings.Contains(prompt, "No pull requests") {
		t.Error("prompt should note absence of PRs")
	}
	if !strings.Contains(prompt, "feat: initial release") {
		t.Error("prompt should still contain commits")
	}
}

func TestSystemPromptIsStable(t *testing.T) {
	prompt := claude.SystemPrompt()
	if !strings.Contains(prompt, "semver") {
		t.Error("system prompt should mention semver")
	}
	if !strings.Contains(prompt, "JSON") {
		t.Error("system prompt should mention JSON output format")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/claude/...
```

Expected: compilation error.

- [ ] **Step 3: Implement prompt construction**

Create `internal/claude/prompt.go`:

```go
package claude

import (
	"fmt"
	"strings"

	gh "github.com/dakaneye/release-pilot/internal/github"
	"github.com/dakaneye/release-pilot/internal/git"
)

type PromptInput struct {
	RepoOwner  string
	RepoName   string
	CurrentTag string
	PRs        []gh.PR
	Commits    []git.Commit
	Diffs      string // optional, empty unless include-diffs is true
}

func SystemPrompt() string {
	return `You are a release engineer analyzing changes for a software release.

Your job:
1. Determine the appropriate semver bump level (major, minor, or patch) based on the changes.
2. Write human-readable release notes in markdown.

Rules for semver bump:
- major: breaking changes to public API, removed features, incompatible changes
- minor: new features, new capabilities, non-breaking additions
- patch: bug fixes, documentation, dependency updates, internal refactoring

Rules for release notes:
- Group changes under these headings (omit empty groups): Breaking Changes, Features, Fixes, Other
- Write for end users: explain what changed and why it matters, not implementation details
- Reference PR numbers as links: [#N](https://github.com/{owner}/{repo}/pull/N)
- Be concise: one line per change unless it needs more context
- If there are breaking changes, put them first with clear migration guidance

Respond with ONLY a JSON object in this exact format:
{
  "bump": "major" | "minor" | "patch",
  "notes": "markdown release notes"
}`
}

func BuildUserPrompt(input PromptInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Repository: %s/%s\n", input.RepoOwner, input.RepoName)
	fmt.Fprintf(&b, "Current version: %s\n\n", input.CurrentTag)

	if len(input.PRs) > 0 {
		b.WriteString("## Merged Pull Requests\n\n")
		for _, pr := range input.PRs {
			fmt.Fprintf(&b, "### #%d: %s\n", pr.Number, pr.Title)
			if pr.Body != "" {
				fmt.Fprintf(&b, "%s\n", pr.Body)
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString("No pull requests found since last release.\n\n")
	}

	if len(input.Commits) > 0 {
		b.WriteString("## Commits\n\n")
		for _, c := range input.Commits {
			fmt.Fprintf(&b, "- %s (%s)\n", c.Subject, c.Hash[:7])
		}
		b.WriteString("\n")
	}

	if input.Diffs != "" {
		b.WriteString("## Diffs\n\n")
		b.WriteString(input.Diffs)
		b.WriteString("\n")
	}

	return b.String()
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/claude/... -v
```

Expected: all 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/claude/
git commit -m "feat(claude): add prompt construction for bump and release notes"
```

---

### Task 8: Claude API Integration — API Call and Response Parsing

**Files:**
- Create: `internal/claude/claude.go`
- Modify: `internal/claude/prompt_test.go` (add response parsing tests)
- Create: `testdata/claude-response.json`

- [ ] **Step 1: Create test fixture**

Create `testdata/claude-response.json`:

```json
{
  "bump": "minor",
  "notes": "## Features\n\n- Add full-text search to the API ([#10](https://github.com/owner/repo/pull/10))\n\n## Fixes\n\n- Fix null pointer crash in auth when token is nil ([#11](https://github.com/owner/repo/pull/11))\n\n## Other\n\n- Update dependencies"
}
```

- [ ] **Step 2: Write tests for API call and response parsing**

Add to `internal/claude/prompt_test.go` (or create a new `internal/claude/claude_test.go`):

Create `internal/claude/claude_test.go`:

```go
package claude_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dakaneye/release-pilot/internal/claude"
	gh "github.com/dakaneye/release-pilot/internal/github"
	"github.com/dakaneye/release-pilot/internal/git"
)

func TestAnalyze(t *testing.T) {
	responseJSON := `{"bump": "minor", "notes": "## Features\n\n- Search added"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

		// Verify request structure
		if req["model"] == nil {
			t.Error("expected model in request")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": responseJSON,
				},
			},
		})
	}))
	defer srv.Close()

	client := claude.NewClient("fake-key", "claude-sonnet-4-6", srv.URL)
	input := claude.PromptInput{
		RepoOwner:  "owner",
		RepoName:   "repo",
		CurrentTag: "v1.0.0",
		PRs: []gh.PR{
			{Number: 10, Title: "feat: add search", Body: "Adds search"},
		},
		Commits: []git.Commit{
			{Hash: "abc1234", Subject: "feat: add search"},
		},
	}

	result, err := client.Analyze(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Bump != "minor" {
		t.Errorf("expected minor, got %s", result.Bump)
	}
	if result.Notes == "" {
		t.Error("expected non-empty notes")
	}
}

func TestAnalyzeInvalidJSON(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// First call returns garbage
			json.NewEncoder(w).Encode(map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "not json at all"},
				},
			})
		} else {
			// Retry returns valid response
			json.NewEncoder(w).Encode(map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": `{"bump": "patch", "notes": "## Fixes\n\n- Bug fix"}`},
				},
			})
		}
	}))
	defer srv.Close()

	client := claude.NewClient("fake-key", "claude-sonnet-4-6", srv.URL)
	input := claude.PromptInput{
		RepoOwner:  "owner",
		RepoName:   "repo",
		CurrentTag: "v1.0.0",
		Commits:    []git.Commit{{Hash: "abc1234", Subject: "fix: bug"}},
	}

	result, err := client.Analyze(input)
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (retry), got %d", callCount)
	}
	if result.Bump != "patch" {
		t.Errorf("expected patch, got %s", result.Bump)
	}
}

func TestAnalyzeBothRetriesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "garbage"},
			},
		})
	}))
	defer srv.Close()

	client := claude.NewClient("fake-key", "claude-sonnet-4-6", srv.URL)
	input := claude.PromptInput{
		RepoOwner:  "owner",
		RepoName:   "repo",
		CurrentTag: "v1.0.0",
		Commits:    []git.Commit{{Hash: "abc1234", Subject: "fix: bug"}},
	}

	_, err := client.Analyze(input)
	if err == nil {
		t.Fatal("expected error after both retries fail")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/claude/...
```

Expected: compilation error — `NewClient`, `Analyze` don't exist.

- [ ] **Step 4: Implement Claude client**

Create `internal/claude/claude.go`:

```go
package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

type AnalysisResult struct {
	Bump  string `json:"bump"`
	Notes string `json:"notes"`
}

func NewClient(apiKey, model, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &Client{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *Client) Analyze(input PromptInput) (AnalysisResult, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		result, err := c.callAPI(input)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return AnalysisResult{}, fmt.Errorf("claude analysis failed after retry: %w", lastErr)
}

func (c *Client) callAPI(input PromptInput) (AnalysisResult, error) {
	reqBody := map[string]any{
		"model":      c.model,
		"max_tokens": 4096,
		"system":     SystemPrompt(),
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": BuildUserPrompt(input),
			},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("API call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return AnalysisResult{}, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return AnalysisResult{}, fmt.Errorf("parse API response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return AnalysisResult{}, fmt.Errorf("empty response from Claude")
	}

	var result AnalysisResult
	text := apiResp.Content[0].Text
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return AnalysisResult{}, fmt.Errorf("parse Claude output as JSON (raw: %s): %w", text, err)
	}

	if result.Bump != "major" && result.Bump != "minor" && result.Bump != "patch" {
		return AnalysisResult{}, fmt.Errorf("invalid bump level: %s", result.Bump)
	}

	return result, nil
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/claude/... -v
```

Expected: all 6 tests pass (3 from prompt_test.go, 3 from claude_test.go).

- [ ] **Step 6: Commit**

```bash
git add internal/claude/ testdata/
git commit -m "feat(claude): add API client with retry and response parsing"
```

---

### Task 9: Pipeline State Tracking

**Files:**
- Create: `internal/pipeline/state.go`
- Create: `internal/pipeline/state_test.go`

- [ ] **Step 1: Write tests for state tracking**

Create `internal/pipeline/state_test.go`:

```go
package pipeline_test

import (
	"path/filepath"
	"testing"

	"github.com/dakaneye/release-pilot/internal/pipeline"
)

func TestNewState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".release-pilot-state.json")

	state, err := pipeline.LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.IsCompleted("detect") {
		t.Error("detect should not be completed in fresh state")
	}
}

func TestCompleteStep(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".release-pilot-state.json")

	state, _ := pipeline.LoadState(path)
	state.Complete("detect")
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}

	// Reload and verify persistence
	reloaded, err := pipeline.LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.IsCompleted("detect") {
		t.Error("detect should be completed after reload")
	}
	if reloaded.IsCompleted("bump") {
		t.Error("bump should not be completed")
	}
}

func TestStateStoresData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".release-pilot-state.json")

	state, _ := pipeline.LoadState(path)
	state.Set("tag", "v1.2.0")
	state.Set("notes", "## Features\n- stuff")
	state.Save()

	reloaded, _ := pipeline.LoadState(path)
	if reloaded.Get("tag") != "v1.2.0" {
		t.Errorf("expected v1.2.0, got %s", reloaded.Get("tag"))
	}
	if reloaded.Get("notes") == "" {
		t.Error("expected notes to be stored")
	}
}

func TestReset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".release-pilot-state.json")

	state, _ := pipeline.LoadState(path)
	state.Complete("detect")
	state.Set("tag", "v1.0.0")
	state.Save()

	state.Reset()
	state.Save()

	reloaded, _ := pipeline.LoadState(path)
	if reloaded.IsCompleted("detect") {
		t.Error("detect should not be completed after reset")
	}
	if reloaded.Get("tag") != "" {
		t.Error("tag should be empty after reset")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/pipeline/...
```

Expected: compilation error.

- [ ] **Step 3: Implement state tracking**

Create `internal/pipeline/state.go`:

```go
package pipeline

import (
	"encoding/json"
	"os"
)

type State struct {
	Steps map[string]bool   `json:"steps"`
	Data  map[string]string `json:"data"`
	path  string
}

func LoadState(path string) (*State, error) {
	s := &State{
		Steps: make(map[string]bool),
		Data:  make(map[string]string),
		path:  path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return s, nil // fresh state
	}

	if err := json.Unmarshal(data, s); err != nil {
		return s, nil // corrupt state, start fresh
	}

	return s, nil
}

func (s *State) IsCompleted(step string) bool {
	return s.Steps[step]
}

func (s *State) Complete(step string) {
	s.Steps[step] = true
}

func (s *State) Set(key, value string) {
	s.Data[key] = value
}

func (s *State) Get(key string) string {
	return s.Data[key]
}

func (s *State) Reset() {
	s.Steps = make(map[string]bool)
	s.Data = make(map[string]string)
}

func (s *State) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/pipeline/... -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/
git commit -m "feat(pipeline): add state tracking for idempotent step execution"
```

---

### Task 10: Pipeline Orchestration

**Files:**
- Create: `internal/pipeline/pipeline.go`
- Create: `internal/pipeline/pipeline_test.go`

- [ ] **Step 1: Write tests for pipeline orchestration**

Create `internal/pipeline/pipeline_test.go`:

```go
package pipeline_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/dakaneye/release-pilot/internal/pipeline"
)

func TestPipelineRunsAllSteps(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".release-pilot-state.json")

	var executed []string
	makeStep := func(name string) pipeline.Step {
		return pipeline.Step{
			Name: name,
			Run: func(ctx *pipeline.Context) error {
				executed = append(executed, name)
				return nil
			},
		}
	}

	p := pipeline.New(statePath, []pipeline.Step{
		makeStep("detect"),
		makeStep("bump"),
		makeStep("notes"),
		makeStep("release"),
		makeStep("sign"),
	})

	if err := p.Run(false); err != nil {
		t.Fatal(err)
	}

	expected := []string{"detect", "bump", "notes", "release", "sign"}
	if len(executed) != len(expected) {
		t.Fatalf("expected %d steps, got %d", len(expected), len(executed))
	}
	for i, name := range expected {
		if executed[i] != name {
			t.Errorf("step %d: expected %s, got %s", i, name, executed[i])
		}
	}
}

func TestPipelineSkipsCompletedSteps(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".release-pilot-state.json")

	// Pre-complete the detect step
	state, _ := pipeline.LoadState(statePath)
	state.Complete("detect")
	state.Save()

	var executed []string
	makeStep := func(name string) pipeline.Step {
		return pipeline.Step{
			Name: name,
			Run: func(ctx *pipeline.Context) error {
				executed = append(executed, name)
				return nil
			},
		}
	}

	p := pipeline.New(statePath, []pipeline.Step{
		makeStep("detect"),
		makeStep("bump"),
	})

	if err := p.Run(false); err != nil {
		t.Fatal(err)
	}

	if len(executed) != 1 || executed[0] != "bump" {
		t.Errorf("expected only bump to run, got %v", executed)
	}
}

func TestPipelineForceResetsState(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".release-pilot-state.json")

	state, _ := pipeline.LoadState(statePath)
	state.Complete("detect")
	state.Save()

	var executed []string
	makeStep := func(name string) pipeline.Step {
		return pipeline.Step{
			Name: name,
			Run: func(ctx *pipeline.Context) error {
				executed = append(executed, name)
				return nil
			},
		}
	}

	p := pipeline.New(statePath, []pipeline.Step{
		makeStep("detect"),
		makeStep("bump"),
	})

	if err := p.Run(true); err != nil { // force=true
		t.Fatal(err)
	}

	if len(executed) != 2 {
		t.Errorf("expected 2 steps with force, got %d: %v", len(executed), executed)
	}
}

func TestPipelineStopsOnError(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".release-pilot-state.json")

	var executed []string
	p := pipeline.New(statePath, []pipeline.Step{
		{Name: "detect", Run: func(ctx *pipeline.Context) error {
			executed = append(executed, "detect")
			return nil
		}},
		{Name: "bump", Run: func(ctx *pipeline.Context) error {
			executed = append(executed, "bump")
			return fmt.Errorf("API key missing")
		}},
		{Name: "notes", Run: func(ctx *pipeline.Context) error {
			executed = append(executed, "notes")
			return nil
		}},
	})

	err := p.Run(false)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(executed) != 2 {
		t.Errorf("expected 2 steps before error, got %d: %v", len(executed), executed)
	}

	// Verify detect was saved as completed
	state, _ := pipeline.LoadState(statePath)
	if !state.IsCompleted("detect") {
		t.Error("detect should be completed")
	}
	if state.IsCompleted("bump") {
		t.Error("bump should not be completed (it failed)")
	}
}

func TestPipelineRunSingleStep(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".release-pilot-state.json")

	var executed []string
	makeStep := func(name string) pipeline.Step {
		return pipeline.Step{
			Name: name,
			Run: func(ctx *pipeline.Context) error {
				executed = append(executed, name)
				return nil
			},
		}
	}

	p := pipeline.New(statePath, []pipeline.Step{
		makeStep("detect"),
		makeStep("bump"),
		makeStep("notes"),
	})

	if err := p.RunStep("bump", false); err != nil {
		t.Fatal(err)
	}

	if len(executed) != 1 || executed[0] != "bump" {
		t.Errorf("expected only bump, got %v", executed)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/pipeline/...
```

Expected: compilation error — `Pipeline`, `Step`, `Context`, `New`, `Run`, `RunStep` don't exist.

- [ ] **Step 3: Implement pipeline orchestration**

Update `internal/pipeline/pipeline.go`:

```go
package pipeline

import (
	"fmt"
	"log"
)

type Context struct {
	State *State
}

type Step struct {
	Name string
	Run  func(ctx *Context) error
}

type Pipeline struct {
	statePath string
	steps     []Step
}

func New(statePath string, steps []Step) *Pipeline {
	return &Pipeline{
		statePath: statePath,
		steps:     steps,
	}
}

func (p *Pipeline) Run(force bool) error {
	state, err := LoadState(p.statePath)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	if force {
		state.Reset()
		if err := state.Save(); err != nil {
			return fmt.Errorf("reset state: %w", err)
		}
	}

	ctx := &Context{State: state}

	for _, step := range p.steps {
		if state.IsCompleted(step.Name) {
			log.Printf("skipping completed step: %s", step.Name)
			continue
		}

		log.Printf("running step: %s", step.Name)
		if err := step.Run(ctx); err != nil {
			return fmt.Errorf("step %s: %w", step.Name, err)
		}

		state.Complete(step.Name)
		if err := state.Save(); err != nil {
			return fmt.Errorf("save state after %s: %w", step.Name, err)
		}
	}

	return nil
}

func (p *Pipeline) RunStep(name string, force bool) error {
	state, err := LoadState(p.statePath)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	if force {
		delete(state.Steps, name)
	}

	ctx := &Context{State: state}

	for _, step := range p.steps {
		if step.Name == name {
			if state.IsCompleted(step.Name) {
				log.Printf("skipping completed step: %s", step.Name)
				return nil
			}

			log.Printf("running step: %s", step.Name)
			if err := step.Run(ctx); err != nil {
				return fmt.Errorf("step %s: %w", step.Name, err)
			}

			state.Complete(step.Name)
			return state.Save()
		}
	}

	return fmt.Errorf("unknown step: %s", name)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/pipeline/... -v
```

Expected: all 9 tests pass (4 state + 5 pipeline).

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/
git commit -m "feat(pipeline): add step orchestration with idempotency and single-step mode"
```

---

### Task 11: Cosign Signing

**Files:**
- Create: `internal/sign/sign.go`
- Create: `internal/sign/sign_test.go`

- [ ] **Step 1: Write tests for signing**

Create `internal/sign/sign_test.go`:

```go
package sign_test

import (
	"testing"

	"github.com/dakaneye/release-pilot/internal/sign"
)

func TestBuildCosignArgs(t *testing.T) {
	args := sign.CosignArgs("v1.0.0", "owner", "repo")

	// Should use keyless signing
	found := false
	for _, arg := range args {
		if arg == "--yes" {
			found = true
		}
	}
	if !found {
		t.Error("expected --yes flag for non-interactive keyless signing")
	}
}

func TestSignDisabledReturnsNil(t *testing.T) {
	err := sign.Run(false, "v1.0.0", "owner", "repo")
	if err != nil {
		t.Errorf("expected nil when signing disabled, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/sign/...
```

Expected: compilation error.

- [ ] **Step 3: Implement signing**

Create `internal/sign/sign.go`:

```go
package sign

import (
	"fmt"
	"os/exec"
)

func CosignArgs(tag, owner, repo string) []string {
	return []string{
		"sign",
		"--yes", // non-interactive, required for keyless in CI
		fmt.Sprintf("ghcr.io/%s/%s:%s", owner, repo, tag),
	}
}

func Run(enabled bool, tag, owner, repo string) error {
	if !enabled {
		return nil
	}

	args := CosignArgs(tag, owner, repo)
	cmd := exec.Command("cosign", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cosign sign: %s\n%s", err, string(out))
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/sign/... -v
```

Expected: both tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/sign/
git commit -m "feat(sign): add keyless cosign signing"
```

---

### Task 12: Ship Command — Wire Everything Together

**Files:**
- Create: `internal/ship/ship.go`
- Modify: `cmd/release-pilot/main.go`

- [ ] **Step 1: Implement ship command wiring**

Create `internal/ship/ship.go`:

```go
package ship

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/dakaneye/release-pilot/internal/claude"
	"github.com/dakaneye/release-pilot/internal/config"
	"github.com/dakaneye/release-pilot/internal/detect"
	"github.com/dakaneye/release-pilot/internal/git"
	gh "github.com/dakaneye/release-pilot/internal/github"
	"github.com/dakaneye/release-pilot/internal/pipeline"
	"github.com/dakaneye/release-pilot/internal/sign"
	"github.com/dakaneye/release-pilot/internal/version"
)

type Options struct {
	Step        string
	DryRun      bool
	Sign        bool
	VersionOver string // manual version override
	Force       bool
	ConfigPath  string
}

func Run(dir string, opts Options) error {
	cfg := config.Load(opts.ConfigPath)

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY environment variable is required.\nSet it in your CI secrets or export it locally.")
	}

	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		return fmt.Errorf("GITHUB_TOKEN environment variable is required.\nIn GitHub Actions this is provided automatically. Locally, set it to a personal access token.")
	}

	// Determine state file location
	stateDir := os.Getenv("RUNNER_TEMP")
	if stateDir == "" {
		stateDir = os.TempDir()
	}
	statePath := filepath.Join(stateDir, ".release-pilot-state.json")

	claudeClient := claude.NewClient(apiKey, cfg.Model, "")
	ghClient := gh.NewClient(ghToken, "")

	// Shared state across steps
	var eco detect.Result
	var currentVer version.Semver
	var newVer version.Semver
	var analysis claude.AnalysisResult
	var repoOwner, repoName string

	// Parse owner/repo from git remote
	repoOwner, repoName, err := parseRemote(dir)
	if err != nil {
		return fmt.Errorf("parse git remote: %w", err)
	}

	steps := []pipeline.Step{
		{
			Name: "detect",
			Run: func(ctx *pipeline.Context) error {
				var err error
				eco, err = detect.Ecosystem(dir, cfg.Ecosystem)
				if err != nil {
					return err
				}
				log.Printf("detected ecosystem: %s (goreleaser: %v)", eco.Name, eco.HasGoreleaser)
				ctx.State.Set("ecosystem", eco.Name)
				return nil
			},
		},
		{
			Name: "bump",
			Run: func(ctx *pipeline.Context) error {
				tag, err := git.LatestTag(dir)
				if err != nil {
					return fmt.Errorf("find latest tag: %w\nIf this is the first release, create an initial tag: git tag v0.0.0", err)
				}

				currentVer, err = version.ParseTag(tag)
				if err != nil {
					return fmt.Errorf("parse tag %s: %w", tag, err)
				}

				if opts.VersionOver != "" {
					newVer, err = version.ParseTag(opts.VersionOver)
					if err != nil {
						return fmt.Errorf("parse version override %s: %w", opts.VersionOver, err)
					}
				}

				// Gather context for Claude
				tagTime, err := git.TagTimestamp(dir, tag)
				if err != nil {
					return fmt.Errorf("get tag timestamp: %w", err)
				}

				prs, err := ghClient.MergedPRsSince(repoOwner, repoName, tagTime)
				if err != nil {
					log.Printf("warning: could not fetch PRs: %v", err)
				}

				commits, err := git.CommitsSince(dir, tag)
				if err != nil {
					return fmt.Errorf("list commits since %s: %w", tag, err)
				}

				if len(prs) == 0 && len(commits) == 0 {
					return fmt.Errorf("no changes since %s — nothing to release", tag)
				}

				promptInput := claude.PromptInput{
					RepoOwner:  repoOwner,
					RepoName:   repoName,
					CurrentTag: tag,
					PRs:        prs,
					Commits:    commits,
				}

				if cfg.Notes.IncludeDiffs {
					diffs, err := git.DiffSince(dir, tag)
					if err != nil {
						log.Printf("warning: could not get diffs: %v", err)
					} else {
						promptInput.Diffs = diffs
					}
				}

				analysis, err = claudeClient.Analyze(promptInput)
				if err != nil {
					return fmt.Errorf("claude analysis: %w", err)
				}

				if opts.VersionOver == "" {
					newVer = currentVer.Bump(analysis.Bump)
				}

				log.Printf("version: %s -> %s (bump: %s)", currentVer.Tag(), newVer.Tag(), analysis.Bump)

				// Update manifest for Python/Node
				if eco.ManifestPath != "" {
					if err := version.UpdateManifest(eco.ManifestPath, eco.Name, newVer.String()); err != nil {
						return fmt.Errorf("update manifest: %w", err)
					}
				}

				ctx.State.Set("tag", newVer.Tag())
				ctx.State.Set("bump", analysis.Bump)
				ctx.State.Set("notes", analysis.Notes)

				return nil
			},
		},
		{
			Name: "notes",
			Run: func(ctx *pipeline.Context) error {
				// Notes are generated in the bump step (same API call).
				// This step exists for --step targeting and state tracking.
				notes := ctx.State.Get("notes")
				if notes == "" {
					return fmt.Errorf("no release notes found in state — run bump step first")
				}
				log.Printf("release notes generated (%d chars)", len(notes))
				return nil
			},
		},
		{
			Name: "release",
			Run: func(ctx *pipeline.Context) error {
				tag := ctx.State.Get("tag")
				notes := ctx.State.Get("notes")

				if opts.DryRun {
					log.Printf("[dry-run] would create release %s", tag)
					log.Printf("[dry-run] release notes:\n%s", notes)
					return nil
				}

				// Commit version changes for Python/Node
				if eco.ManifestPath != "" {
					if err := git.CommitAll(dir, fmt.Sprintf("release: %s", tag)); err != nil {
						return fmt.Errorf("commit version bump: %w", err)
					}
					if err := git.Push(dir); err != nil {
						return fmt.Errorf("push version commit: %w", err)
					}
				}

				// Create and push tag
				if err := git.CreateTag(dir, tag); err != nil {
					return fmt.Errorf("create tag: %w", err)
				}
				if err := git.PushTag(dir, tag); err != nil {
					return fmt.Errorf("push tag: %w", err)
				}

				if eco.HasGoreleaser {
					// Run goreleaser, then patch release notes
					if err := runGoreleaser(dir); err != nil {
						return fmt.Errorf("goreleaser: %w", err)
					}
					if err := ghClient.EditReleaseBody(repoOwner, repoName, tag, notes); err != nil {
						return fmt.Errorf("patch release notes: %w", err)
					}
				} else {
					url, err := ghClient.CreateRelease(repoOwner, repoName, gh.ReleaseParams{
						Tag:        tag,
						Name:       tag,
						Body:       notes,
						Draft:      cfg.GitHub.Draft,
						Prerelease: cfg.GitHub.Prerelease,
					})
					if err != nil {
						return fmt.Errorf("create release: %w", err)
					}
					ctx.State.Set("release-url", url)
					log.Printf("release created: %s", url)
				}

				return nil
			},
		},
		{
			Name: "sign",
			Run: func(ctx *pipeline.Context) error {
				tag := ctx.State.Get("tag")
				if opts.DryRun {
					if opts.Sign {
						log.Printf("[dry-run] would sign release artifacts for %s", tag)
					}
					return nil
				}
				return sign.Run(opts.Sign, tag, repoOwner, repoName)
			},
		},
	}

	p := pipeline.New(statePath, steps)

	if opts.Step != "" {
		return p.RunStep(opts.Step, opts.Force)
	}
	return p.Run(opts.Force)
}

func parseRemote(dir string) (string, string, error) {
	out, err := git.RemoteURL(dir)
	if err != nil {
		return "", "", err
	}

	// Parse owner/repo from remote URL
	// Handles: git@github.com:owner/repo.git, https://github.com/owner/repo.git
	return parseOwnerRepo(out)
}

func parseOwnerRepo(remote string) (string, string, error) {
	// Handle SSH: git@github.com:owner/repo.git
	if idx := len("git@github.com:"); len(remote) > idx && remote[:idx] == "git@github.com:" {
		path := remote[idx:]
		path = trimSuffix(path, ".git")
		parts := splitOnce(path, "/")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("cannot parse remote: %s", remote)
		}
		return parts[0], parts[1], nil
	}

	// Handle HTTPS: https://github.com/owner/repo.git
	prefix := "https://github.com/"
	if len(remote) > len(prefix) && remote[:len(prefix)] == prefix {
		path := remote[len(prefix):]
		path = trimSuffix(path, ".git")
		parts := splitOnce(path, "/")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("cannot parse remote: %s", remote)
		}
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("unsupported remote format: %s", remote)
}

func trimSuffix(s, suffix string) string {
	if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func splitOnce(s, sep string) []string {
	for i := 0; i < len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return []string{s[:i], s[i+len(sep):]}
		}
	}
	return []string{s}
}

func runGoreleaser(dir string) error {
	cmd := exec.Command("goreleaser", "release", "--clean")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

Wait — `runGoreleaser` uses `exec` and `os` but they're not imported. Let me fix the imports:

```go
import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dakaneye/release-pilot/internal/claude"
	"github.com/dakaneye/release-pilot/internal/config"
	"github.com/dakaneye/release-pilot/internal/detect"
	"github.com/dakaneye/release-pilot/internal/git"
	gh "github.com/dakaneye/release-pilot/internal/github"
	"github.com/dakaneye/release-pilot/internal/pipeline"
	"github.com/dakaneye/release-pilot/internal/sign"
	"github.com/dakaneye/release-pilot/internal/version"
)
```

- [ ] **Step 2: Add missing git helpers**

Add to `internal/git/git.go`:

```go
func RemoteURL(dir string) (string, error) {
	out, err := runGit(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("get remote URL: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func TagTimestamp(dir string, tag string) (time.Time, error) {
	out, err := runGit(dir, "log", "-1", "--format=%aI", tag)
	if err != nil {
		return time.Time{}, fmt.Errorf("get tag timestamp for %s: %w", tag, err)
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(out))
}

func DiffSince(dir string, tag string) (string, error) {
	out, err := runGit(dir, "diff", tag+"..HEAD")
	if err != nil {
		return "", fmt.Errorf("diff since %s: %w", tag, err)
	}
	return out, nil
}
```

Add `"time"` to the imports in `internal/git/git.go`.

- [ ] **Step 3: Update main.go with ship command**

Update `cmd/release-pilot/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/dakaneye/release-pilot/internal/ship"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "release-pilot",
		Short: "Orchestrate releases with AI-powered release notes",
	}

	root.AddCommand(versionCmd())
	root.AddCommand(shipCmd())
	root.AddCommand(initCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}
}

func shipCmd() *cobra.Command {
	var opts ship.Options

	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Run the release pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			return ship.Run(dir, opts)
		},
	}

	cmd.Flags().StringVar(&opts.Step, "step", "", "Run a single pipeline step (detect, bump, notes, release, sign)")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Preview the release without making changes")
	cmd.Flags().BoolVar(&opts.Sign, "sign", false, "Sign release artifacts with cosign (keyless)")
	cmd.Flags().StringVar(&opts.VersionOver, "version", "", "Override the version instead of AI-determined bump")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "Reset pipeline state and start fresh")
	cmd.Flags().StringVar(&opts.ConfigPath, "config", "", "Path to config file (default: .release-pilot.yaml)")

	return cmd
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate a .release-pilot.yaml config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			content := `# release-pilot configuration
# See: https://github.com/dakaneye/release-pilot

ecosystem: auto
model: claude-sonnet-4-6

notes:
  include-diffs: false

github:
  draft: false
  prerelease: false
`
			if err := os.WriteFile(".release-pilot.yaml", []byte(content), 0o644); err != nil {
				return err
			}
			fmt.Println("Created .release-pilot.yaml")
			return nil
		},
	}
}
```

- [ ] **Step 4: Verify compilation**

```bash
go build ./cmd/release-pilot/
```

Expected: builds without errors.

- [ ] **Step 5: Run all tests**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ship/ internal/git/git.go cmd/release-pilot/main.go
git commit -m "feat(ship): wire pipeline steps into ship command with CLI flags"
```

---

### Task 13: Error Path Tests

**Files:**
- Create: `internal/ship/ship_test.go`

- [ ] **Step 1: Write error path tests**

Create `internal/ship/ship_test.go`:

```go
package ship_test

import (
	"os"
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
	if !containsStr(err.Error(), "ANTHROPIC_API_KEY") {
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
	if !containsStr(err.Error(), "GITHUB_TOKEN") {
		t.Errorf("error should mention GITHUB_TOKEN: %v", err)
	}
}

func TestShipNoManifest(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "fake")
	t.Setenv("GITHUB_TOKEN", "fake")

	dir := t.TempDir()
	// Init a git repo with a remote so parseRemote doesn't fail first
	initGitRepo(t, dir)

	err := ship.Run(dir, ship.Options{})
	if err == nil {
		t.Fatal("expected error for no manifest")
	}
	if !containsStr(err.Error(), "no ecosystem detected") {
		t.Errorf("error should mention ecosystem detection: %v", err)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "remote", "add", "origin", "https://github.com/test/repo.git"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}

	// Need at least one commit
	os.WriteFile(dir+"/README.md", []byte("test"), 0o644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

Add `"os/exec"` to the imports.

- [ ] **Step 2: Run tests**

```bash
go test ./internal/ship/... -v
```

Expected: all 3 error path tests pass.

- [ ] **Step 3: Commit**

```bash
git add internal/ship/ship_test.go
git commit -m "test(ship): add error path tests for missing credentials and manifests"
```

---

### Task 14: GitHub Action — Simple and Advanced Modes

**Files:**
- Create: `action/action.yml`
- Create: `action/setup/action.yml`
- Create: `action/install.sh`

- [ ] **Step 1: Create the install script**

Create `action/install.sh`:

```bash
#!/usr/bin/env bash
set -Eeuo pipefail

VERSION="${1:-latest}"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

if [[ "$VERSION" == "latest" ]]; then
  VERSION="$(curl -fsSL https://api.github.com/repos/dakaneye/release-pilot/releases/latest | grep '"tag_name"' | head -1 | cut -d'"' -f4)"
fi

URL="https://github.com/dakaneye/release-pilot/releases/download/${VERSION}/release-pilot_${VERSION#v}_${OS}_${ARCH}.tar.gz"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "$URL" -o "$TMPDIR/release-pilot.tar.gz"
tar -xzf "$TMPDIR/release-pilot.tar.gz" -C "$TMPDIR"
install -m 755 "$TMPDIR/release-pilot" /usr/local/bin/release-pilot

echo "release-pilot ${VERSION} installed"
```

- [ ] **Step 2: Create the setup-only action (advanced mode)**

Create `action/setup/action.yml`:

```yaml
name: "Setup release-pilot"
description: "Install the release-pilot CLI binary"
inputs:
  version:
    description: "Version to install (default: latest)"
    required: false
    default: "latest"
runs:
  using: "composite"
  steps:
    - name: Install release-pilot
      shell: bash
      run: bash "${{ github.action_path }}/../install.sh" "${{ inputs.version }}"
```

- [ ] **Step 3: Create the full action (simple mode)**

Create `action/action.yml`:

```yaml
name: "release-pilot"
description: "Orchestrate releases with AI-powered release notes"
inputs:
  anthropic-api-key:
    description: "Anthropic API key"
    required: true
  model:
    description: "Claude model to use"
    required: false
    default: "claude-sonnet-4-6"
  sign:
    description: "Sign release artifacts with cosign"
    required: false
    default: "false"
  draft:
    description: "Create draft release"
    required: false
    default: "false"
  prerelease:
    description: "Mark as prerelease"
    required: false
    default: "false"
  include-diffs:
    description: "Include full diffs in AI prompt"
    required: false
    default: "false"
  args:
    description: "Additional CLI arguments"
    required: false
    default: ""
outputs:
  version:
    description: "Released version"
    value: ${{ steps.release.outputs.version }}
  tag:
    description: "Git tag created"
    value: ${{ steps.release.outputs.tag }}
  release-url:
    description: "GitHub release URL"
    value: ${{ steps.release.outputs.release-url }}
  release-notes:
    description: "Generated release notes"
    value: ${{ steps.release.outputs.release-notes }}
runs:
  using: "composite"
  steps:
    - name: Install release-pilot
      shell: bash
      run: bash "${{ github.action_path }}/install.sh"

    - name: Run release-pilot
      id: release
      shell: bash
      env:
        ANTHROPIC_API_KEY: ${{ inputs.anthropic-api-key }}
        RELEASE_PILOT_MODEL: ${{ inputs.model }}
      run: |
        ARGS="ship"
        if [[ "${{ inputs.sign }}" == "true" ]]; then
          ARGS="$ARGS --sign"
        fi
        if [[ "${{ inputs.include-diffs }}" == "true" ]]; then
          # include-diffs is a config option, create temp config
          echo "notes:" > /tmp/.release-pilot-action.yaml
          echo "  include-diffs: true" >> /tmp/.release-pilot-action.yaml
          ARGS="$ARGS --config /tmp/.release-pilot-action.yaml"
        fi
        if [[ -n "${{ inputs.args }}" ]]; then
          ARGS="$ARGS ${{ inputs.args }}"
        fi

        release-pilot $ARGS

    - name: Post summary
      if: always()
      shell: bash
      run: |
        echo "## Release Pilot" >> "$GITHUB_STEP_SUMMARY"
        echo "" >> "$GITHUB_STEP_SUMMARY"
        echo "Release completed successfully." >> "$GITHUB_STEP_SUMMARY"
```

- [ ] **Step 4: Validate action YAML syntax**

```bash
python3 -c "import yaml; yaml.safe_load(open('action/action.yml')); print('action.yml: valid')"
python3 -c "import yaml; yaml.safe_load(open('action/setup/action.yml')); print('setup/action.yml: valid')"
```

Expected: both print "valid".

- [ ] **Step 5: Commit**

```bash
git add action/
git commit -m "feat(action): add GitHub Action with simple and advanced modes"
```

---

### Task 15: Goreleaser Config for release-pilot Itself

**Files:**
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Create goreleaser config**

Create `.goreleaser.yaml`:

```yaml
version: 2

builds:
  - main: ./cmd/release-pilot
    binary: release-pilot
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

changelog:
  disable: true # release-pilot generates its own notes

release:
  github:
    owner: dakaneye
    name: release-pilot
```

- [ ] **Step 2: Create release-pilot's own config**

Create `.release-pilot.yaml`:

```yaml
ecosystem: go
model: claude-sonnet-4-6

github:
  draft: false
  prerelease: false
```

- [ ] **Step 3: Commit**

```bash
git add .goreleaser.yaml .release-pilot.yaml
git commit -m "build: add goreleaser config and dogfood release-pilot config"
```

---

### Task 16: CLI Acceptance Test

**Files:**
- Create: `internal/ship/acceptance_test.go`

This is the critical test — it builds the binary and runs it against a real git repo with stubbed external services.

- [ ] **Step 1: Write the acceptance test**

Create `internal/ship/acceptance_test.go`:

```go
//go:build acceptance

package ship_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAcceptanceNodeRelease(t *testing.T) {
	// Build the binary
	binary := filepath.Join(t.TempDir(), "release-pilot")
	build := exec.Command("go", "build", "-o", binary, "./cmd/release-pilot")
	build.Dir = projectRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}

	// Set up a git repo with a package.json
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
	var createdRelease map[string]any
	ghStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/repos/test/repo/pulls":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"number":    1,
					"title":     "feat: cool feature",
					"body":      "Adds a cool feature",
					"merged_at": "2026-03-15T10:00:00Z",
					"state":     "closed",
				},
			})
		case r.URL.Path == "/repos/test/repo/releases" && r.Method == "POST":
			json.NewDecoder(r.Body).Decode(&createdRelease)
			json.NewEncoder(w).Encode(map[string]any{
				"id":       1,
				"html_url": "https://github.com/test/repo/releases/tag/v1.1.0",
			})
		default:
			w.WriteHeader(200)
			w.Write([]byte("{}"))
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
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ship failed: %s\n%s", err, out)
	}

	output := string(out)
	if !containsStr(output, "v1.1.0") {
		t.Errorf("output should mention new version v1.1.0:\n%s", output)
	}
}

func setupNodeRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "remote", "add", "origin", "https://github.com/test/repo.git"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}

	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "name": "test-app",
  "version": "1.0.0"
}`), 0o644)

	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
	exec.Command("git", "-C", dir, "tag", "v1.0.0").Run()

	// Add a change after the tag
	os.WriteFile(filepath.Join(dir, "index.js"), []byte("console.log('hello')"), 0o644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feat: cool feature").Run()
}

func projectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from test file to find go.mod
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
```

Note: this test uses `ANTHROPIC_BASE_URL` and `GITHUB_API_URL` env vars, which means `ship.go` needs to respect those for the API base URLs. Update `internal/ship/ship.go` to check:

```go
claudeBaseURL := os.Getenv("ANTHROPIC_BASE_URL")
claudeClient := claude.NewClient(apiKey, cfg.Model, claudeBaseURL)

ghBaseURL := os.Getenv("GITHUB_API_URL")
ghClient := gh.NewClient(ghToken, ghBaseURL)
```

- [ ] **Step 2: Run the acceptance test**

```bash
go test ./internal/ship/... -tags acceptance -run TestAcceptance -v
```

Expected: test passes — binary builds, stubs respond, dry-run completes.

- [ ] **Step 3: Commit**

```bash
git add internal/ship/acceptance_test.go internal/ship/ship.go
git commit -m "test(ship): add CLI acceptance test with stubbed APIs"
```

---

### Task 17: Final Verification

- [ ] **Step 1: Run all unit tests**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 2: Run acceptance tests**

```bash
go test ./internal/ship/... -tags acceptance -v
```

Expected: acceptance test passes.

- [ ] **Step 3: Build and verify CLI**

```bash
go build -o release-pilot ./cmd/release-pilot
./release-pilot version
./release-pilot ship --help
./release-pilot init
cat .release-pilot.yaml
```

Expected: version prints "dev", ship help shows all flags, init creates config file.

- [ ] **Step 4: Run go vet and check for issues**

```bash
go vet ./...
```

Expected: no issues.

- [ ] **Step 5: Commit any final fixes**

```bash
git add -A
git commit -m "chore: final cleanup and verification"
```
