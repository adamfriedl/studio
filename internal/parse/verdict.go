package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	verdictRE = regexp.MustCompile(`(?mi)^verdict:\s*(\S+)\s*$`)
	summaryRE = regexp.MustCompile(`(?mis)^summary:\s*(.+?)(?:\n(?:questions|comments|verdict):|\z)`)
)

// IntakeResult is the Phase 5 QC structured output.
type IntakeResult struct {
	Verdict   string // ok | needs-work
	Summary   string
	Questions []string
	Raw       string
}

// ReviewComment is one Phase 6 inline review comment.
type ReviewComment struct {
	Path string
	Line int
	Body string
}

// PRReviewResult is the Phase 6 reviewer structured output.
type PRReviewResult struct {
	Verdict  string // lgtm | changes-requested
	Summary  string
	Comments []ReviewComment
	Raw      string
}

// ParseIntake extracts verdict/summary/questions from agent output.
func ParseIntake(text string) (*IntakeResult, error) {
	text = strings.TrimSpace(text)
	out := &IntakeResult{Raw: text}
	m := verdictRE.FindStringSubmatch(text)
	if m == nil {
		return out, fmt.Errorf("missing verdict")
	}
	v := strings.ToLower(strings.TrimSpace(m[1]))
	switch v {
	case "ok", "needs-work":
		out.Verdict = v
	default:
		return out, fmt.Errorf("unknown intake verdict %q", m[1])
	}
	if sm := summaryRE.FindStringSubmatch(text); sm != nil {
		out.Summary = strings.TrimSpace(sm[1])
	}
	out.Questions = parseBulletSection(text, "questions")
	return out, nil
}

// ParsePRReview extracts verdict/summary/comments from agent output.
func ParsePRReview(text string) (*PRReviewResult, error) {
	text = strings.TrimSpace(text)
	out := &PRReviewResult{Raw: text}
	m := verdictRE.FindStringSubmatch(text)
	if m == nil {
		return out, fmt.Errorf("missing verdict")
	}
	v := strings.ToLower(strings.TrimSpace(m[1]))
	switch v {
	case "lgtm", "changes-requested":
		out.Verdict = v
	default:
		return out, fmt.Errorf("unknown review verdict %q", m[1])
	}
	if sm := summaryRE.FindStringSubmatch(text); sm != nil {
		out.Summary = strings.TrimSpace(sm[1])
	}
	out.Comments = parseReviewComments(text)
	return out, nil
}

func parseBulletSection(text, heading string) []string {
	re := regexp.MustCompile(`(?mi)^` + regexp.QuoteMeta(heading) + `:\s*$`)
	loc := re.FindStringIndex(text)
	if loc == nil {
		return nil
	}
	rest := text[loc[1]:]
	var out []string
	for _, line := range strings.Split(rest, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if matched, _ := regexp.MatchString(`(?i)^[a-z_]+:\s*$`, line); matched && !strings.HasPrefix(line, "-") {
			break
		}
		if strings.HasPrefix(line, "-") {
			item := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if item != "" {
				out = append(out, item)
			}
			continue
		}
		// Stop at next top-level key
		if strings.Contains(line, ":") && !strings.HasPrefix(line, " ") {
			key := strings.SplitN(line, ":", 2)[0]
			if key == "path" || key == "line" || key == "body" {
				continue
			}
			if matched, _ := regexp.MatchString(`^[a-z_]+$`, strings.ToLower(key)); matched {
				break
			}
		}
	}
	return out
}

func parseReviewComments(text string) []ReviewComment {
	re := regexp.MustCompile(`(?mi)^comments:\s*$`)
	loc := re.FindStringIndex(text)
	if loc == nil {
		return nil
	}
	rest := text[loc[1]:]
	var out []ReviewComment
	var cur *ReviewComment
	flush := func() {
		if cur != nil && cur.Path != "" && cur.Body != "" {
			out = append(out, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(rest, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "-") {
			flush()
			cur = &ReviewComment{}
			trim = strings.TrimSpace(strings.TrimPrefix(trim, "-"))
		}
		if cur == nil {
			continue
		}
		key, val, ok := strings.Cut(trim, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		switch key {
		case "path":
			cur.Path = val
		case "line":
			n, _ := strconv.Atoi(val)
			cur.Line = n
		case "body":
			cur.Body = val
		}
	}
	flush()
	return out
}

// RetryPrompt asks the agent to reply with only the schema.
func RetryPrompt(kind string) string {
	switch kind {
	case "intake":
		return "Reply with exactly this schema and nothing else:\nverdict: ok | needs-work\nsummary: one short paragraph\nquestions:\n- …"
	case "pr-review":
		return "Reply with exactly this schema and nothing else:\nverdict: lgtm | changes-requested\nsummary: one short paragraph\ncomments:\n- path: relative/file.go\n  line: 12\n  body: what is wrong"
	default:
		return "Reply with exactly the required schema, nothing else."
	}
}
