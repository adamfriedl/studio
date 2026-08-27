package binding

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const markerStart = "<!-- studio:v1"
const markerEnd = "-->"

var blockRE = regexp.MustCompile(`(?s)<!-- studio:v1\s*(.*?)\s*-->`)

// ErrCorrupt means the binding comment is missing when required, duplicated, or unparseable.
var ErrCorrupt = fmt.Errorf("binding corrupt")

type Binding struct {
	AgentID        string
	Worker         string
	Repo           string
	Branch         string
	PRURL          string
	PRNumber       string
	PRStatus       string
	SpecStatus     string
	SpecAgentID    string
	ReviewAgentID  string
	ReviewSHA      string
	ReviewRounds   string
	ReviewStatus   string
	ReviewCursor   string
	UpdatedAt      string
}

func Parse(issueBody string) (*Binding, error) {
	matches := blockRE.FindAllStringSubmatch(issueBody, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: missing studio:v1 block", ErrCorrupt)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("%w: multiple studio:v1 blocks", ErrCorrupt)
	}
	b := &Binding{}
	for _, line := range strings.Split(matches[0][1], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("%w: bad line %q", ErrCorrupt, line)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "agent_id":
			b.AgentID = val
		case "worker":
			b.Worker = val
		case "repo":
			b.Repo = val
		case "branch":
			b.Branch = val
		case "pr_url":
			b.PRURL = val
		case "pr_number":
			b.PRNumber = val
		case "pr_status":
			b.PRStatus = val
		case "spec_status":
			b.SpecStatus = val
		case "spec_agent_id":
			b.SpecAgentID = val
		case "review_agent_id":
			b.ReviewAgentID = val
		case "review_sha":
			b.ReviewSHA = val
		case "review_rounds":
			b.ReviewRounds = val
		case "review_status":
			b.ReviewStatus = val
		case "review_cursor":
			b.ReviewCursor = val
		case "updated_at":
			b.UpdatedAt = val
		default:
			// unknown keys: ignore for forward compat, but empty key is corrupt
			if key == "" {
				return nil, fmt.Errorf("%w: empty key", ErrCorrupt)
			}
		}
	}
	return b, nil
}

func (b *Binding) Render() string {
	if b.UpdatedAt == "" {
		b.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	var sb strings.Builder
	sb.WriteString(markerStart)
	sb.WriteByte('\n')
	write := func(k, v string) {
		if v == "" {
			return
		}
		fmt.Fprintf(&sb, "%s: %s\n", k, v)
	}
	write("agent_id", b.AgentID)
	write("worker", b.Worker)
	write("repo", b.Repo)
	write("branch", b.Branch)
	write("pr_url", b.PRURL)
	write("pr_number", b.PRNumber)
	write("pr_status", b.PRStatus)
	write("spec_status", b.SpecStatus)
	write("spec_agent_id", b.SpecAgentID)
	write("review_agent_id", b.ReviewAgentID)
	write("review_sha", b.ReviewSHA)
	write("review_rounds", b.ReviewRounds)
	write("review_status", b.ReviewStatus)
	write("review_cursor", b.ReviewCursor)
	write("updated_at", b.UpdatedAt)
	sb.WriteString(markerEnd)
	return sb.String()
}

// Upsert replaces an existing studio:v1 block or appends one. Multiple blocks → ErrCorrupt.
func Upsert(issueBody string, b *Binding) (string, error) {
	matches := blockRE.FindAllStringIndex(issueBody, -1)
	block := b.Render()
	if len(matches) > 1 {
		return "", fmt.Errorf("%w: multiple studio:v1 blocks", ErrCorrupt)
	}
	if len(matches) == 1 {
		start, end := matches[0][0], matches[0][1]
		return issueBody[:start] + block + issueBody[end:], nil
	}
	body := strings.TrimRight(issueBody, "\n")
	if body == "" {
		return block + "\n", nil
	}
	return body + "\n\n" + block + "\n", nil
}
