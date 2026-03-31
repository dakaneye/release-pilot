package claude_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dakaneye/release-pilot/internal/claude"
	"github.com/dakaneye/release-pilot/internal/git"
	gh "github.com/dakaneye/release-pilot/internal/github"
)

func TestAnalyze(t *testing.T) {
	responseJSON := `{"bump": "minor", "notes": "## Features\n\n- Search added"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

		if req["model"] == nil {
			t.Error("expected model in request")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": responseJSON},
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
			{Hash: "abc1234567890", Subject: "feat: add search"},
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
			json.NewEncoder(w).Encode(map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "not json at all"},
				},
			})
		} else {
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
		Commits:    []git.Commit{{Hash: "abc1234567890", Subject: "fix: bug"}},
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
		Commits:    []git.Commit{{Hash: "abc1234567890", Subject: "fix: bug"}},
	}

	_, err := client.Analyze(input)
	if err == nil {
		t.Fatal("expected error after both retries fail")
	}
}
