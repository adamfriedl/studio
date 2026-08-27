package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type ListedIssue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
	Labels  []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (c *Client) ListOpenIssuesByLabel(ctx context.Context, owner, repo, label string) ([]ListedIssue, error) {
	var out []ListedIssue
	path := fmt.Sprintf("/repos/%s/%s/issues?state=open&labels=%s&per_page=100", owner, repo, label)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) RemoveLabel(ctx context.Context, owner, repo string, number int, label string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels/%s", owner, repo, number, label)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) CloseIssue(ctx context.Context, owner, repo string, number int) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number)
	return c.do(ctx, http.MethodPatch, path, map[string]string{"state": "closed"}, nil)
}

type PullRequest struct {
	Number  int    `json:"number"`
	State   string `json:"state"`
	Merged  bool   `json:"merged"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
	Head    struct {
		SHA string `json:"sha"`
		Ref string `json:"ref"`
	} `json:"head"`
}

func (c *Client) GetPull(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	var pr PullRequest
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	if err := c.do(ctx, http.MethodGet, path, nil, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

type CheckRun struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
}

func (c *Client) ListCheckRuns(ctx context.Context, owner, repo, sha string) ([]CheckRun, error) {
	var payload struct {
		CheckRuns []CheckRun `json:"check_runs"`
	}
	path := fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs?per_page=100", owner, repo, sha)
	if err := c.do(ctx, http.MethodGet, path, nil, &payload); err != nil {
		return nil, err
	}
	return payload.CheckRuns, nil
}

func FailedChecks(runs []CheckRun) []CheckRun {
	var failed []CheckRun
	for _, cr := range runs {
		if cr.Status == "completed" && (cr.Conclusion == "failure" || cr.Conclusion == "timed_out") {
			failed = append(failed, cr)
		}
	}
	return failed
}

func ChecksPending(runs []CheckRun) bool {
	if len(runs) == 0 {
		return false
	}
	for _, cr := range runs {
		if cr.Status != "completed" {
			return true
		}
	}
	return false
}

func (c *Client) PrefetchFailedRunLogs(ctx context.Context, owner, repo, branch string, limit int) (string, bool, error) {
	if limit <= 0 {
		limit = 8000
	}
	var runs struct {
		WorkflowRuns []struct {
			ID         int64  `json:"id"`
			Conclusion string `json:"conclusion"`
			HTMLURL    string `json:"html_url"`
		} `json:"workflow_runs"`
	}
	path := fmt.Sprintf("/repos/%s/%s/actions/runs?branch=%s&per_page=15", owner, repo, branch)
	if err := c.do(ctx, http.MethodGet, path, nil, &runs); err != nil {
		return "", false, err
	}
	var failedID int64
	var runURL string
	for _, r := range runs.WorkflowRuns {
		if r.Conclusion == "failure" || r.Conclusion == "timed_out" {
			failedID = r.ID
			runURL = r.HTMLURL
			break
		}
	}
	if failedID == 0 {
		return "", false, nil
	}

	var jobs struct {
		Jobs []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			Conclusion string `json:"conclusion"`
		} `json:"jobs"`
	}
	jpath := fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs", owner, repo, failedID)
	if err := c.do(ctx, http.MethodGet, jpath, nil, &jobs); err != nil {
		return "", false, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Workflow run: %s\n", runURL)
	got := false
	for _, j := range jobs.Jobs {
		if j.Conclusion != "failure" && j.Conclusion != "timed_out" {
			continue
		}
		text, err := c.getRaw(ctx, fmt.Sprintf("/repos/%s/%s/actions/jobs/%d/logs", owner, repo, j.ID))
		if err != nil {
			fmt.Fprintf(&b, "\n## job %s: log fetch failed: %v\n", j.Name, err)
			continue
		}
		got = true
		fmt.Fprintf(&b, "\n## job %s\n", j.Name)
		b.WriteString(tail(text, limit/2))
		b.WriteByte('\n')
	}
	return b.String(), got, nil
}

type PullComment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	HTMLURL string `json:"html_url"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
}

func (c *Client) ListPRReviewComments(ctx context.Context, owner, repo string, number int) ([]PullComment, error) {
	var out []PullComment
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments?per_page=100", owner, repo, number)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) getRaw(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "studio")
	res, err := c.http().Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 2_000_000))
	if err != nil {
		return "", err
	}
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("github GET %s: %s: %s", path, res.Status, truncate(string(data), 200))
	}
	return string(data), nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…\n" + s[len(s)-n:]
}

func ParseOwnerRepo(full string) (owner, repo string, err error) {
	full = strings.TrimPrefix(full, "https://github.com/")
	full = strings.TrimSuffix(full, ".git")
	parts := strings.Split(full, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("bad repo %q", full)
	}
	return parts[0], parts[1], nil
}

func ParsePRNumber(prURL string) (int, error) {
	parts := strings.Split(strings.TrimRight(prURL, "/"), "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("bad pr url")
	}
	return strconv.Atoi(parts[len(parts)-1])
}
