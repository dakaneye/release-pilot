package git_test

import (
	"context"
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
	run(t, dir, "git", "config", "tag.gpgsign", "false")
	run(t, dir, "git", "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
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

func TestLatestTagNoTags(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)

	_, err := git.LatestTag(ctx, dir, "")
	if err == nil {
		t.Fatal("expected error when no tags exist")
	}
}

func TestCommitsSince(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)
	run(t, dir, "git", "tag", "v0.1.0")

	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "feat: add feature A")

	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "fix: fix bug B")

	commits, err := git.CommitsSince(ctx, dir, "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
}

func TestCreateTag(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)

	err := git.CreateTag(ctx, dir, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	tag, err := git.LatestTag(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", tag)
	}
}

func TestRemoteURL(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)
	run(t, dir, "git", "remote", "add", "origin", "https://github.com/test/repo.git")

	url, err := git.RemoteURL(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://github.com/test/repo.git" {
		t.Errorf("got %s", url)
	}
}

func TestTagTimestamp(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)
	run(t, dir, "git", "tag", "-a", "v1.0.0", "-m", "v1.0.0")

	ts, err := git.TagTimestamp(ctx, dir, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if ts.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestPreviousTag(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)
	run(t, dir, "git", "tag", "-a", "v0.1.0", "-m", "v0.1.0")

	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "feat: something")
	run(t, dir, "git", "tag", "-a", "v0.2.0", "-m", "v0.2.0")

	prev, err := git.PreviousTag(ctx, dir, "v0.2.0", "")
	if err != nil {
		t.Fatal(err)
	}
	if prev != "v0.1.0" {
		t.Errorf("expected v0.1.0, got %s", prev)
	}
}

func TestPreviousTagNoPrevious(t *testing.T) {
	ctx := t.Context()
	dir := initRepo(t)
	run(t, dir, "git", "tag", "-a", "v0.1.0", "-m", "v0.1.0")

	_, err := git.PreviousTag(ctx, dir, "v0.1.0", "")
	if err == nil {
		t.Fatal("expected error when no previous tag")
	}
}

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

func TestContextCancellation(t *testing.T) {
	dir := initRepo(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := git.LatestTag(ctx, dir, "")
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}
