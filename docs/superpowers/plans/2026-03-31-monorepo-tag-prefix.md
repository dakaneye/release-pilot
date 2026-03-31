# Monorepo Tag Prefix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `tag-prefix` and `sub-dir` config options so release-pilot works in monorepos using `<name>/v<semver>` tag conventions.

**Architecture:** New config fields flow through the existing pipeline. The `version` package gains prefix-aware parsing/formatting. The `git` package gains prefix filtering for tags and path filtering for commits/diffs. The `ship` orchestrator threads both through existing steps.

**Tech Stack:** Go, standard library, existing test patterns (real git repos in `t.TempDir()`)

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/config/config.go` | Modify | Add `TagPrefix` and `SubDir` fields |
| `internal/config/config_test.go` | Modify | Test new fields load from YAML |
| `internal/version/version.go` | Modify | Add `ParsePrefixedTag` and `PrefixedTag` |
| `internal/version/version_test.go` | Modify | Test prefix parsing and formatting |
| `internal/git/git.go` | Modify | Add prefix param to `LatestTag`/`PreviousTag`, path param to `CommitsSince`/`DiffSince` |
| `internal/git/git_test.go` | Modify | Test prefix filtering and path scoping |
| `internal/ship/ship.go` | Modify | Thread `TagPrefix`/`SubDir` through pipeline |
| `internal/ship/ship_test.go` | Modify | Verify config propagation |
| `cmd/release-pilot/main.go` | Modify | Add `--tag-prefix` and `--sub-dir` flags |

---

### Task 1: Config — Add TagPrefix and SubDir Fields

**Files:**
- Modify: `internal/config/config.go:12-16`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test for tag-prefix and sub-dir config loading**

Add to `internal/config/config_test.go`:

```go
func TestLoadTagPrefixAndSubDir(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".release-pilot.yaml")
	err := os.WriteFile(cfgPath, []byte(`
ecosystem: go
tag-prefix: review-code/
sub-dir: review-code/
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TagPrefix != "review-code/" {
		t.Errorf("expected tag-prefix=review-code/, got %s", cfg.TagPrefix)
	}
	if cfg.SubDir != "review-code/" {
		t.Errorf("expected sub-dir=review-code/, got %s", cfg.SubDir)
	}
}

func TestDefaultsHaveEmptyPrefixAndSubDir(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TagPrefix != "" {
		t.Errorf("expected empty tag-prefix by default, got %s", cfg.TagPrefix)
	}
	if cfg.SubDir != "" {
		t.Errorf("expected empty sub-dir by default, got %s", cfg.SubDir)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go test ./internal/config/ -run 'TestLoadTagPrefix|TestDefaultsHaveEmpty' -v`
Expected: FAIL — `cfg.TagPrefix` undefined

- [ ] **Step 3: Add fields to Config struct**

In `internal/config/config.go`, add two fields to the `Config` struct:

```go
type Config struct {
	Ecosystem string       `yaml:"ecosystem"`
	Model     string       `yaml:"model"`
	TagPrefix string       `yaml:"tag-prefix"`
	SubDir    string       `yaml:"sub-dir"`
	Notes     NotesConfig  `yaml:"notes"`
	GitHub    GitHubConfig `yaml:"github"`
}
```

No changes to `defaults()` needed — Go zero values for strings are `""`, which is the desired default.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go test ./internal/config/ -v`
Expected: ALL PASS (new tests + existing tests)

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add tag-prefix and sub-dir fields"
```

---

### Task 2: Version — Add Prefix-Aware Parsing and Formatting

**Files:**
- Modify: `internal/version/version.go:43-57`
- Modify: `internal/version/version_test.go`

- [ ] **Step 1: Write failing tests for ParsePrefixedTag and PrefixedTag**

Add to `internal/version/version_test.go`:

```go
func TestParsePrefixedTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		prefix  string
		major   int
		minor   int
		patch   int
		wantErr bool
	}{
		{"standard prefix", "review-code/v1.2.3", "review-code/", 1, 2, 3, false},
		{"prefix without v", "review-code/1.2.3", "review-code/", 1, 2, 3, false},
		{"empty prefix passthrough", "v1.2.3", "", 1, 2, 3, false},
		{"wrong prefix", "other-code/v1.2.3", "review-code/", 0, 0, 0, true},
		{"no prefix on prefixed tag", "v1.2.3", "review-code/", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := version.ParsePrefixedTag(tt.tag, tt.prefix)
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

func TestPrefixedTag(t *testing.T) {
	v := version.Semver{Major: 1, Minor: 2, Patch: 3}

	if got := v.PrefixedTag("review-code/"); got != "review-code/v1.2.3" {
		t.Errorf("expected review-code/v1.2.3, got %s", got)
	}
	if got := v.PrefixedTag(""); got != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %s", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go test ./internal/version/ -run 'TestParsePrefixedTag|TestPrefixedTag' -v`
Expected: FAIL — `ParsePrefixedTag` and `PrefixedTag` undefined

- [ ] **Step 3: Implement ParsePrefixedTag and PrefixedTag**

Add to `internal/version/version.go`:

```go
// PrefixedTag returns the version as a git tag with the given prefix.
// When prefix is empty, this is equivalent to Tag().
func (v Semver) PrefixedTag(prefix string) string {
	return prefix + "v" + v.String()
}

// ParsePrefixedTag strips the given prefix from a tag and parses the remainder as semver.
// When prefix is empty, this is equivalent to ParseTag.
// Returns an error if the tag does not start with the expected prefix.
func ParsePrefixedTag(tag string, prefix string) (Semver, error) {
	if prefix != "" {
		if !strings.HasPrefix(tag, prefix) {
			return Semver{}, fmt.Errorf("tag %s does not match prefix %s", tag, prefix)
		}
		tag = strings.TrimPrefix(tag, prefix)
	}
	return ParseTag(tag)
}
```

Add `"strings"` to the imports in `version.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go test ./internal/version/ -v`
Expected: ALL PASS (new + existing tests)

- [ ] **Step 5: Commit**

```bash
git add internal/version/version.go internal/version/version_test.go
git commit -m "feat(version): add prefix-aware tag parsing and formatting"
```

---

### Task 3: Git — Add Prefix Filtering to LatestTag and PreviousTag

**Files:**
- Modify: `internal/git/git.go:17-50`
- Modify: `internal/git/git_test.go`

- [ ] **Step 1: Write failing tests for prefix-filtered LatestTag and PreviousTag**

Add to `internal/git/git_test.go`:

```go
func TestLatestTagWithPrefix(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)

	// Create mixed tags: unprefixed and two different prefixes
	run(t, dir, "git", "tag", "v0.1.0")
	run(t, dir, "git", "tag", "review-code/v0.1.0")
	run(t, dir, "git", "tag", "review-code/v0.2.0")
	run(t, dir, "git", "tag", "batch-review/v1.0.0")

	tag, err := git.LatestTag(ctx, dir, "review-code/")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "review-code/v0.2.0" {
		t.Errorf("expected review-code/v0.2.0, got %s", tag)
	}
}

func TestLatestTagEmptyPrefix(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)
	run(t, dir, "git", "tag", "v0.1.0")
	run(t, dir, "git", "tag", "v0.2.0")

	tag, err := git.LatestTag(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v0.2.0" {
		t.Errorf("expected v0.2.0, got %s", tag)
	}
}

func TestLatestTagPrefixNoMatch(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)
	run(t, dir, "git", "tag", "v0.1.0")

	_, err := git.LatestTag(ctx, dir, "review-code/")
	if err == nil {
		t.Fatal("expected error when no tags match prefix")
	}
}

func TestPreviousTagWithPrefix(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)
	run(t, dir, "git", "tag", "-a", "v1.0.0", "-m", "v1.0.0")
	run(t, dir, "git", "tag", "-a", "review-code/v0.1.0", "-m", "review-code/v0.1.0")

	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "feat: something")
	run(t, dir, "git", "tag", "-a", "review-code/v0.2.0", "-m", "review-code/v0.2.0")

	prev, err := git.PreviousTag(ctx, dir, "review-code/v0.2.0", "review-code/")
	if err != nil {
		t.Fatal(err)
	}
	if prev != "review-code/v0.1.0" {
		t.Errorf("expected review-code/v0.1.0, got %s", prev)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go test ./internal/git/ -run 'TestLatestTagWith|TestLatestTagEmpty|TestLatestTagPrefix|TestPreviousTagWith' -v`
Expected: FAIL — too many arguments to `LatestTag`/`PreviousTag`

- [ ] **Step 3: Update LatestTag and PreviousTag signatures**

In `internal/git/git.go`, update both functions:

```go
func LatestTag(ctx context.Context, dir string, prefix string) (string, error) {
	out, err := runGit(ctx, dir, "tag", "--sort=-version:refname")
	if err != nil {
		return "", fmt.Errorf("no tags found: %w", err)
	}
	tags := strings.Split(strings.TrimSpace(out), "\n")
	for _, t := range tags {
		if t == "" {
			continue
		}
		if prefix == "" || strings.HasPrefix(t, prefix) {
			return t, nil
		}
	}
	if prefix != "" {
		return "", fmt.Errorf("no tags found matching prefix %s", prefix)
	}
	return "", errors.New("no tags found")
}

func PreviousTag(ctx context.Context, dir string, current string, prefix string) (string, error) {
	out, err := runGit(ctx, dir, "tag", "--sort=-version:refname")
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}
	tags := strings.Split(strings.TrimSpace(out), "\n")
	found := false
	for _, t := range tags {
		if prefix != "" && !strings.HasPrefix(t, prefix) {
			continue
		}
		if t == current {
			found = true
			continue
		}
		if found && t != "" {
			return t, nil
		}
	}
	if !found {
		return "", fmt.Errorf("tag %s not found", current)
	}
	return "", fmt.Errorf("no tag before %s", current)
}
```

- [ ] **Step 4: Fix existing callers and tests that use old signatures**

Update all existing `LatestTag` calls to pass `""` as the third argument. Update all existing `PreviousTag` calls to pass `""` as the fourth argument.

In `internal/git/git_test.go`, update existing tests:
- `TestLatestTag`: `git.LatestTag(ctx, dir)` → `git.LatestTag(ctx, dir, "")`
- `TestLatestTagNoTags`: `git.LatestTag(ctx, dir)` → `git.LatestTag(ctx, dir, "")`
- `TestCreateTag`: `git.LatestTag(ctx, dir)` → `git.LatestTag(ctx, dir, "")`
- `TestContextCancellation`: `git.LatestTag(ctx, dir)` → `git.LatestTag(ctx, dir, "")`
- `TestPreviousTag`: `git.PreviousTag(ctx, dir, "v0.2.0")` → `git.PreviousTag(ctx, dir, "v0.2.0", "")`
- `TestPreviousTagNoPrevious`: `git.PreviousTag(ctx, dir, "v0.1.0")` → `git.PreviousTag(ctx, dir, "v0.1.0", "")`

In `internal/ship/ship.go`, update calls:
- Line 95: `git.PreviousTag(ctx, dir, opts.Tag)` → `git.PreviousTag(ctx, dir, opts.Tag, "")`
- Line 102: `git.LatestTag(ctx, dir)` → `git.LatestTag(ctx, dir, "")`

- [ ] **Step 5: Run all tests to verify they pass**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go test ./internal/git/ -v && go test ./internal/ship/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/git/git.go internal/git/git_test.go internal/ship/ship.go
git commit -m "feat(git): add prefix filtering to LatestTag and PreviousTag"
```

---

### Task 4: Git — Add Path Filtering to CommitsSince and DiffSince

**Files:**
- Modify: `internal/git/git.go:52-71,127-133`
- Modify: `internal/git/git_test.go`

- [ ] **Step 1: Write failing tests for path-filtered CommitsSince and DiffSince**

Add to `internal/git/git_test.go`:

```go
func TestCommitsSinceWithPaths(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)
	run(t, dir, "git", "tag", "v0.1.0")

	// Create a commit in subdir
	subdir := filepath.Join(dir, "review-code")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "feat: add review-code module")

	// Create a commit outside subdir
	if err := os.WriteFile(filepath.Join(dir, "root.go"), []byte("package root"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "feat: add root module")

	// Without path filter: both commits
	all, err := git.CommitsSince(ctx, dir, "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 commits without filter, got %d", len(all))
	}

	// With path filter: only the subdir commit
	filtered, err := git.CommitsSince(ctx, dir, "v0.1.0", "review-code/")
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 commit with path filter, got %d", len(filtered))
	}
	if !strings.Contains(filtered[0].Subject, "review-code") {
		t.Errorf("expected review-code commit, got %s", filtered[0].Subject)
	}
}

func TestDiffSinceWithPaths(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)
	run(t, dir, "git", "tag", "v0.1.0")

	// Create files in and outside subdir
	subdir := filepath.Join(dir, "review-code")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "root.go"), []byte("package root"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "feat: add files")

	// Full diff includes both files
	fullDiff, err := git.DiffSince(ctx, dir, "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fullDiff, "root.go") {
		t.Error("full diff should include root.go")
	}

	// Filtered diff only includes subdir
	filteredDiff, err := git.DiffSince(ctx, dir, "v0.1.0", "review-code/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filteredDiff, "review-code/main.go") {
		t.Error("filtered diff should include review-code/main.go")
	}
	if strings.Contains(filteredDiff, "root.go") {
		t.Error("filtered diff should not include root.go")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go test ./internal/git/ -run 'TestCommitsSinceWithPaths|TestDiffSinceWithPaths' -v`
Expected: FAIL — too many arguments

- [ ] **Step 3: Update CommitsSince and DiffSince signatures**

In `internal/git/git.go`:

```go
func CommitsSince(ctx context.Context, dir string, tag string, paths ...string) ([]Commit, error) {
	args := []string{"log", tag + "..HEAD", "--pretty=format:%H %s"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	out, err := runGit(ctx, dir, args...)
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
```

```go
func DiffSince(ctx context.Context, dir string, tag string, paths ...string) (string, error) {
	args := []string{"diff", tag + "..HEAD"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	out, err := runGit(ctx, dir, args...)
	if err != nil {
		return "", fmt.Errorf("diff since %s: %w", tag, err)
	}
	return out, nil
}
```

No caller updates needed — variadic `paths ...string` is backward compatible with existing zero-arg calls.

- [ ] **Step 4: Run all git tests to verify they pass**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go test ./internal/git/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/git/git.go internal/git/git_test.go
git commit -m "feat(git): add path filtering to CommitsSince and DiffSince"
```

---

### Task 5: Ship — Thread TagPrefix and SubDir Through Pipeline

**Files:**
- Modify: `internal/ship/ship.go:72-270`

- [ ] **Step 1: Update the bump step to use prefix and sub-dir**

In `internal/ship/ship.go`, the bump step closure (starting ~line 90) needs these changes:

1. Read prefix from config at the top of the bump step:
```go
prefix := cfg.TagPrefix
```

2. Pass prefix to `LatestTag` and `PreviousTag`:
```go
// CI mode
prev, err := git.PreviousTag(ctx, dir, opts.Tag, prefix)
// Normal mode
tag, err := git.LatestTag(ctx, dir, prefix)
```

3. Use `ParsePrefixedTag` instead of `ParseTag`:
```go
current, err := version.ParsePrefixedTag(latestTag, prefix)
```

4. Pass `cfg.SubDir` to `CommitsSince` when non-empty:
```go
var commits []git.Commit
if cfg.SubDir != "" {
	commits, err = git.CommitsSince(ctx, dir, latestTag, cfg.SubDir)
} else {
	commits, err = git.CommitsSince(ctx, dir, latestTag)
}
```

5. Pass `cfg.SubDir` to `DiffSince` when non-empty:
```go
if cfg.Notes.IncludeDiffs {
	var diffs string
	if cfg.SubDir != "" {
		diffs, err = git.DiffSince(ctx, dir, latestTag, cfg.SubDir)
	} else {
		diffs, err = git.DiffSince(ctx, dir, latestTag)
	}
	if err != nil {
		return fmt.Errorf("get diffs: %w", err)
	}
	input.Diffs = diffs
}
```

6. Use `PrefixedTag` for new tag creation:
```go
// In the version override path:
sctx.State.Set("tag", overVer.PrefixedTag(prefix))
// In the normal bump path:
sctx.State.Set("tag", next.PrefixedTag(prefix))
```

7. Add import for `version` package (add to existing imports):
```go
"github.com/dakaneye/release-pilot/internal/version"
```

- [ ] **Step 2: Run existing tests to verify no regressions**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go test ./internal/ship/ -v`
Expected: ALL PASS (empty prefix = same behavior)

- [ ] **Step 3: Commit**

```bash
git add internal/ship/ship.go
git commit -m "feat(ship): thread tag-prefix and sub-dir through pipeline"
```

---

### Task 6: CLI — Add --tag-prefix and --sub-dir Flags

**Files:**
- Modify: `internal/ship/ship.go:23-30` (Options struct)
- Modify: `internal/ship/ship.go:34-38` (Run function, apply overrides)
- Modify: `cmd/release-pilot/main.go:53-61`

- [ ] **Step 1: Add TagPrefix and SubDir to Options struct**

In `internal/ship/ship.go`, add to `Options`:

```go
type Options struct {
	Step        string
	DryRun      bool
	Sign        bool
	VersionOver string
	Tag         string
	TagPrefix   string
	SubDir      string
	Force       bool
	ConfigPath  string
}
```

- [ ] **Step 2: Apply CLI overrides in Run function**

In `internal/ship/ship.go`, after loading config (~line 38), add override logic:

```go
if opts.TagPrefix != "" {
	cfg.TagPrefix = opts.TagPrefix
}
if opts.SubDir != "" {
	cfg.SubDir = opts.SubDir
}
```

- [ ] **Step 3: Add cobra flags**

In `cmd/release-pilot/main.go`, inside `shipCmd()`, add after existing flags:

```go
cmd.Flags().StringVar(&opts.TagPrefix, "tag-prefix", "", "tag prefix for monorepo (e.g. review-code/)")
cmd.Flags().StringVar(&opts.SubDir, "sub-dir", "", "sub-directory to scope commits/diffs to")
```

- [ ] **Step 4: Run all tests**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ship/ship.go cmd/release-pilot/main.go
git commit -m "feat(cli): add --tag-prefix and --sub-dir flags"
```

---

### Task 7: Lint and Final Verification

**Files:** None (verification only)

- [ ] **Step 1: Run golangci-lint**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && golangci-lint run ./...`
Expected: No issues

- [ ] **Step 2: Run go vet**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go vet ./...`
Expected: No issues

- [ ] **Step 3: Run full test suite**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 4: Build binary**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && go build ./cmd/release-pilot`
Expected: Clean build, no errors

- [ ] **Step 5: Verify --help shows new flags**

Run: `cd /Users/samueldacanay/dev/personal/release-pilot && ./release-pilot ship --help`
Expected: Output includes `--tag-prefix` and `--sub-dir` flags with descriptions
