package github_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gh "github.com/dakaneye/release-pilot/internal/github"
)

func TestListMergedPRs(t *testing.T) {
	ctx := t.Context()
	mergedAt := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	prs := []map[string]any{
		{
			"number":     1,
			"title":      "feat: add feature A",
			"body":       "This adds feature A",
			"merged_at":  mergedAt.Format(time.RFC3339),
			"updated_at": mergedAt.Format(time.RFC3339),
			"state":      "closed",
		},
		{
			"number":     2,
			"title":      "closed but not merged",
			"body":       "This was closed",
			"merged_at":  nil,
			"updated_at": mergedAt.Format(time.RFC3339),
			"state":      "closed",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/pulls" {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(prs); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := gh.NewClient("fake-token", srv.URL)
	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	result, err := client.MergedPRsSince(ctx, "owner", "repo", since)
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
	ctx := t.Context()
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/releases" && r.Method == "POST" {
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{
				"id":       123,
				"html_url": "https://github.com/owner/repo/releases/tag/v1.0.0",
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := gh.NewClient("fake-token", srv.URL)
	url, err := client.CreateRelease(ctx, "owner", "repo", gh.ReleaseParams{
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
	ctx := t.Context()
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/releases/123" && r.Method == "PATCH" {
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{"id": 123}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		if r.URL.Path == "/repos/owner/repo/releases/tags/v1.0.0" {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{"id": 123}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := gh.NewClient("fake-token", srv.URL)
	err := client.EditReleaseBody(ctx, "owner", "repo", "v1.0.0", "Updated notes")
	if err != nil {
		t.Fatal(err)
	}
	if received["body"] != "Updated notes" {
		t.Errorf("expected updated body, got %v", received["body"])
	}
}

func TestHTTPErrorResponse(t *testing.T) {
	ctx := t.Context()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer srv.Close()

	client := gh.NewClient("fake", srv.URL)
	_, err := client.MergedPRsSince(ctx, "o", "r", time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 in error: %v", err)
	}
}

func TestPaginatedPRs(t *testing.T) {
	ctx := t.Context()
	mergedAt := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	callCount := 0

	// We need the server URL for the Link header, so use a variable.
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			// First page: include Link header for next page
			w.Header().Set("Link", `<`+srvURL+`/repos/owner/repo/pulls?page=2>; rel="next"`)
			if err := json.NewEncoder(w).Encode([]map[string]any{
				{
					"number":     1,
					"title":      "feat: page 1",
					"body":       "",
					"merged_at":  mergedAt.Format(time.RFC3339),
					"updated_at": mergedAt.Format(time.RFC3339),
				},
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// Second page: no Link header
		if err := json.NewEncoder(w).Encode([]map[string]any{
			{
				"number":     2,
				"title":      "feat: page 2",
				"body":       "",
				"merged_at":  mergedAt.Format(time.RFC3339),
				"updated_at": mergedAt.Format(time.RFC3339),
			},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	client := gh.NewClient("fake-token", srv.URL)
	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	result, err := client.MergedPRsSince(ctx, "owner", "repo", since)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 PRs across pages, got %d", len(result))
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}
}
