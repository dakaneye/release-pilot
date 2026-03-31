package claude

import (
	"bytes"
	"context"
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

func (c *Client) Analyze(ctx context.Context, input PromptInput) (AnalysisResult, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		result, err := c.callAPI(ctx, input)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return AnalysisResult{}, fmt.Errorf("claude analysis failed after retry: %w", lastErr)
}

func (c *Client) callAPI(ctx context.Context, input PromptInput) (AnalysisResult, error) {
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

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(data))
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
	defer func() { _ = resp.Body.Close() }()

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
