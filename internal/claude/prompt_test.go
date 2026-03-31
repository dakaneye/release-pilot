package claude_test

import (
	"strings"
	"testing"

	"github.com/dakaneye/release-pilot/internal/claude"
	"github.com/dakaneye/release-pilot/internal/git"
	gh "github.com/dakaneye/release-pilot/internal/github"
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
			{Hash: "abc1234567890", Subject: "feat: add search"},
			{Hash: "def4567890123", Subject: "fix: null pointer in auth"},
			{Hash: "ghi7890123456", Subject: "chore: update deps"},
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
			{Hash: "abc1234567890", Subject: "feat: initial release"},
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
	if !strings.Contains(prompt, "No markdown fences") {
		t.Error("system prompt should prohibit markdown fences")
	}
	if !strings.Contains(prompt, "Do not invent") {
		t.Error("system prompt should guard against hallucination")
	}
}

func TestBuildPromptTaskAnchor(t *testing.T) {
	input := claude.PromptInput{
		RepoOwner:  "owner",
		RepoName:   "repo",
		CurrentTag: "v1.0.0",
		Commits: []git.Commit{
			{Hash: "abc1234567890", Subject: "fix: bug"},
		},
	}

	prompt := claude.BuildUserPrompt(input)

	if !strings.Contains(prompt, "Analyze the following changes") {
		t.Error("user prompt should start with task anchor")
	}
}

func TestBuildPromptNoPRsGuidesCommitUsage(t *testing.T) {
	input := claude.PromptInput{
		RepoOwner:  "owner",
		RepoName:   "repo",
		CurrentTag: "v1.0.0",
		Commits: []git.Commit{
			{Hash: "abc1234567890", Subject: "fix: bug"},
		},
	}

	prompt := claude.BuildUserPrompt(input)

	if !strings.Contains(prompt, "sole source of changes") {
		t.Error("prompt should instruct model to use commits when no PRs exist")
	}
}
