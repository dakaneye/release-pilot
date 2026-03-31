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
