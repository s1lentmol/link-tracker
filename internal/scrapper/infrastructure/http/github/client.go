package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

func (c *Client) GetRepoUpdatedAt(ctx context.Context, owner string, repo string) (time.Time, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return time.Time{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return time.Time{}, fmt.Errorf("github status: %d", resp.StatusCode)
	}

	var payload struct {
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return time.Time{}, fmt.Errorf("decode body: %w", err)
	}
	if strings.TrimSpace(payload.UpdatedAt) == "" {
		return time.Time{}, fmt.Errorf("missing updated_at")
	}

	ts, err := time.Parse(time.RFC3339, payload.UpdatedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse updated_at: %w", err)
	}

	return ts, nil
}
