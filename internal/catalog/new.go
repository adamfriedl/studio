package catalog

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	repoNameRE       = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	issueFormNameRE  = regexp.MustCompile(`(?mi)^###\s+New repo name\s*\n+(\S[^\n]*)`)
	issueFormModelRE = regexp.MustCompile(`(?mi)^###\s+Cursor model[^\n]*\n+(\S[^\n]*)`)
)

// HasRepo reports whether name is already in the allowlist.
func (c *Catalog) HasRepo(name string) bool {
	_, _, ok := c.ResolveRepo(name)
	return ok
}

// TemplateSourceNames returns short repo names used as template sources (e.g. template-go-svc).
func (c *Catalog) TemplateSourceNames() map[string]struct{} {
	out := map[string]struct{}{}
	for _, t := range c.Templates {
		from := strings.TrimSpace(t.From)
		if from == "" {
			continue
		}
		parts := strings.Split(from, "/")
		if len(parts) >= 2 {
			out[parts[len(parts)-1]] = struct{}{}
		}
	}
	return out
}

// AppendRepo adds name to the in-memory allowlist if missing. Returns true if appended.
func (c *Catalog) AppendRepo(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || c.HasRepo(name) {
		return false
	}
	c.Repos = append(c.Repos, Repo{Name: name})
	return true
}

// NewRepoName picks the GitHub repo name for a new: dispatch.
// Prefer YAML frontmatter name:/repo:, then issue-form "New repo name", else slug the title.
func NewRepoName(title, body string) (string, error) {
	if fm, ok := parseFrontmatter(body); ok {
		for _, key := range []string{"name", "repo"} {
			if v := strings.TrimSpace(fm[key]); v != "" {
				return validateRepoName(v)
			}
		}
	}
	if m := issueFormNameRE.FindStringSubmatch(body); len(m) == 2 {
		v := strings.TrimSpace(m[1])
		if v != "" && !strings.EqualFold(v, "_No response_") {
			return validateRepoName(v)
		}
	}
	return validateRepoName(slugTitle(title))
}

func validateRepoName(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".git")
	if name == "" {
		return "", fmt.Errorf("empty repo name")
	}
	if len(name) > 100 {
		return "", fmt.Errorf("repo name too long (%d)", len(name))
	}
	if !repoNameRE.MatchString(name) {
		return "", fmt.Errorf("invalid repo name %q", name)
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("invalid repo name %q", name)
	}
	return name, nil
}

func parseFrontmatter(body string) (map[string]string, bool) {
	// Prefer leading frontmatter; also accept a --- block later (issue forms prepend headings).
	idx := strings.Index(body, "---\n")
	if idx < 0 {
		idx = strings.Index(body, "---\r\n")
	}
	if idx < 0 {
		return nil, false
	}
	rest := body[idx+len("---"):]
	rest = strings.TrimLeft(rest, "\r\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, false
	}
	block := rest[:end]
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(block), &raw); err != nil {
		return nil, false
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = fmt.Sprint(v)
	}
	return out, true
}

func slugTitle(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '.':
			b.WriteByte('-')
		}
		if b.Len() >= 50 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if out == "" {
		return "new-repo"
	}
	return out
}

// AppendRepoYAML surgically appends "  - name: <name>" to a repos.yaml document
// without rewriting unrelated keys/comments. No-op if the name is already listed.
func AppendRepoYAML(content []byte, name string) ([]byte, error) {
	name, err := validateRepoName(name)
	if err != nil {
		return nil, err
	}
	text := string(content)
	needle := "- name: " + name
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == needle {
			return content, nil
		}
	}
	if !strings.Contains(text, "\nrepos:") && !strings.HasPrefix(strings.TrimSpace(text), "repos:") {
		return nil, fmt.Errorf("repos.yaml: missing repos: key")
	}
	trimmed := strings.TrimRight(text, "\n") + "\n"
	trimmed += "  - name: " + name + "\n"
	return []byte(trimmed), nil
}
