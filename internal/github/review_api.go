package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// PullFile is one file in a PR diff listing.
type PullFile struct {
	Filename string `json:"filename"`
	Status   string `json:"status"`
	Patch    string `json:"patch"`
}

// ListPullFiles returns changed files (with patches when available).
func (c *Client) ListPullFiles(ctx context.Context, owner, repo string, number int) ([]PullFile, error) {
	var out []PullFile
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=100", owner, repo, number)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FormatDiff builds a truncated text diff for reviewer prompts.
func FormatDiff(files []PullFile, limit int) string {
	if limit <= 0 {
		limit = 60000
	}
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "### %s (%s)\n", f.Filename, f.Status)
		if f.Patch == "" {
			b.WriteString("(no patch — binary or too large)\n\n")
			continue
		}
		b.WriteString(f.Patch)
		b.WriteString("\n\n")
		if b.Len() >= limit {
			b.WriteString("…(diff truncated)\n")
			break
		}
	}
	return b.String()
}

// ReviewCommentIn is an inline comment for CreatePullReview.
type ReviewCommentIn struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
}

// CreatePullReview posts a PR review. event should be "COMMENT" for Studio Phase 6.
func (c *Client) CreatePullReview(ctx context.Context, owner, repo string, number int, body, event string, comments []ReviewCommentIn) error {
	payload := map[string]any{
		"body":  body,
		"event": event,
	}
	if len(comments) > 0 {
		payload["comments"] = comments
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, repo, number)
	return c.do(ctx, http.MethodPost, path, payload, nil)
}
