package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	Token      string
	BaseURL    string // default https://api.github.com
	HTTPClient *http.Client
	UserAgent  string
}

func (c *Client) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://api.github.com"
}

func (c *Client) http() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base()+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	ua := c.UserAgent
	if ua == "" {
		ua = "studio"
	}
	req.Header.Set("User-Agent", ua)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("github %s %s: %s: %s", method, path, res.Status, truncate(string(data), 400))
	}
	if out == nil || len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

type Issue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
	Labels  []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (c *Client) GetIssue(ctx context.Context, owner, repo string, number int) (*Issue, error) {
	var issue Issue
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number)
	if err := c.do(ctx, http.MethodGet, path, nil, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

func (c *Client) UpdateIssue(ctx context.Context, owner, repo string, number int, body string, labels []string) error {
	payload := map[string]any{"body": body}
	if labels != nil {
		payload["labels"] = labels
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number)
	return c.do(ctx, http.MethodPatch, path, payload, nil)
}

func (c *Client) AddComment(ctx context.Context, owner, repo string, number int, body string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	return c.do(ctx, http.MethodPost, path, map[string]string{"body": body}, nil)
}

func (c *Client) AddLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, number)
	return c.do(ctx, http.MethodPost, path, labels, nil)
}
