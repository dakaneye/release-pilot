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
	run(t, dir, "git", "config", "tag.gpgsign", "false")
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
