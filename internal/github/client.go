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
