package stackoverflow

import (
	"context"
	"encoding/json"
	"errors"
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

func (c *Client) QuestionUpdatedAt(ctx context.Context, id string) (time.Time, error) {
	url := fmt.Sprintf("%s/questions/%s?site=stackoverflow", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return time.Time{}, fmt.Errorf("do request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return time.Time{}, fmt.Errorf("stackoverflow status: %d", resp.StatusCode)
	}

	var payload struct {
		Items []struct {
			LastActivityDate int64 `json:"last_activity_date"`
		} `json:"items"`
	}
	decodeErr := json.NewDecoder(resp.Body).Decode(&payload)
	if decodeErr != nil {
		return time.Time{}, fmt.Errorf("decode body: %w", decodeErr)
	}
	if len(payload.Items) == 0 || payload.Items[0].LastActivityDate == 0 {
		return time.Time{}, errors.New("missing last_activity_date")
	}

	return time.Unix(payload.Items[0].LastActivityDate, 0).UTC(), nil
}
