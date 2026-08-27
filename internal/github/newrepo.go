package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Repository is a minimal GitHub repo payload.
type Repository struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	HTMLURL  string `json:"html_url"`
}

// RepoExists reports whether owner/repo is visible to the token.
func (c *Client) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	path := fmt.Sprintf("/repos/%s/%s", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	ua := c.UserAgent
	if ua == "" {
		ua = "studio"
	}
	req.Header.Set("User-Agent", ua)
	res, err := c.http().Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("github GET %s: %s", path, res.Status)
	}
}

// ErrRepoExists means owner/name already exists. CreateFromTemplate still returns the Repository.
var ErrRepoExists = fmt.Errorf("repository already exists")

// CreateFromTemplate generates a new repo from a GitHub template repository.
// templateFull is "owner/template-repo". New repo is created under owner as name.
func (c *Client) CreateFromTemplate(ctx context.Context, templateFull, owner, name string, private bool) (*Repository, error) {
	parts := strings.SplitN(templateFull, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("bad template %q", templateFull)
	}
	exists, err := c.RepoExists(ctx, owner, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return &Repository{
			Name:     name,
			FullName: owner + "/" + name,
			Private:  private,
			HTMLURL:  "https://github.com/" + owner + "/" + name,
		}, ErrRepoExists
	}
	path := fmt.Sprintf("/repos/%s/%s/generate", parts[0], parts[1])
	var out Repository
	err = c.do(ctx, http.MethodPost, path, map[string]any{
		"owner":                owner,
		"name":                 name,
		"private":              private,
		"include_all_branches": false,
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.FullName == "" {
		out.FullName = owner + "/" + name
	}
	if out.HTMLURL == "" {
		out.HTMLURL = "https://github.com/" + out.FullName
	}
	return &out, nil
}

// EnsureLabel creates a label on owner/repo if it does not already exist.
func (c *Client) EnsureLabel(ctx context.Context, owner, repo, name, color, description string) error {
	path := fmt.Sprintf("/repos/%s/%s/labels/%s", owner, repo, url.PathEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "studio")
	res, err := c.http().Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return nil
	}
	if res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("github GET %s: %s", path, res.Status)
	}
	if color == "" {
		color = "0E8A16"
	}
	payload := map[string]string{
		"name":  name,
		"color": strings.TrimPrefix(color, "#"),
	}
	if description != "" {
		payload["description"] = description
	}
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/labels", owner, repo), payload, nil)
}

// FileContent is a GitHub contents API blob (decoded Content).
type FileContent struct {
	SHA     string
	Content []byte
}

// GetFile reads a file from the default branch via the Contents API.
func (c *Client) GetFile(ctx context.Context, owner, repo, path string) (*FileContent, error) {
	var raw struct {
		SHA      string `json:"sha"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)
	if err := c.do(ctx, http.MethodGet, apiPath, nil, &raw); err != nil {
		return nil, err
	}
	content := raw.Content
	if raw.Encoding == "base64" {
		// GitHub may insert newlines in base64 payloads.
		cleaned := strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' || r == ' ' {
				return -1
			}
			return r
		}, content)
		b, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		return &FileContent{SHA: raw.SHA, Content: b}, nil
	}
	return &FileContent{SHA: raw.SHA, Content: []byte(content)}, nil
}

// PutFile creates or updates a file on the default branch.
func (c *Client) PutFile(ctx context.Context, owner, repo, path, message string, content []byte, sha string) error {
	payload := map[string]any{
		"message": message,
		"content": base64.StdEncoding.EncodeToString(content),
	}
	if sha != "" {
		payload["sha"] = sha
	}
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)
	return c.do(ctx, http.MethodPut, apiPath, payload, nil)
}
