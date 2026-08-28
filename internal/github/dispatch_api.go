package github

import (
	"context"
	"fmt"
)

// RepositoryDispatch POSTs event_type + client_payload to owner/repo.
// Used to kick studio watch without waiting on Actions cron.
func (c *Client) RepositoryDispatch(ctx context.Context, owner, repo, eventType string, payload map[string]any) error {
	if eventType == "" {
		return fmt.Errorf("repository_dispatch: empty event_type")
	}
	body := map[string]any{
		"event_type":     eventType,
		"client_payload": payload,
	}
	path := fmt.Sprintf("/repos/%s/%s/dispatches", owner, repo)
	return c.do(ctx, "POST", path, body, nil)
}
