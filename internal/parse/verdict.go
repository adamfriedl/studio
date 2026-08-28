package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	verdictRE  = regexp.MustCompile(`(?mi)^verdict:\s*(\S+)\s*$`)
	summaryRE  = regexp.MustCompile(`(?mis)^summary:\s*(.+?)(?:\n(?:questions|comments|focus|effort|tests|security|verdict):|\z)`)
	effortRE   = regexp.MustCompile(`(?mi)^effort:\s*(\S+)\s*$`)
	testsRE    = regexp.MustCompile(`(?mi)^tests:\s*(.+?)\s*$`)
	securityRE = regexp.MustCompile(`(?mi)^security:\s*(.+?)\s*$`)
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

// FocusArea is one human-facing Reviewer Guide focus item.
type FocusArea struct {
	Area string
	Why  string
}

// PRReviewResult is the Phase 6 reviewer structured output.
type PRReviewResult struct {
	Verdict  string // lgtm | changes-requested
	Summary  string
	Effort   string // optional 1-5
	Tests    string // optional yes | no | n/a
	Security string // optional one-liner
	Focus    []FocusArea
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
	if em := effortRE.FindStringSubmatch(text); em != nil {
		out.Effort = strings.TrimSpace(em[1])
	}
	if tm := testsRE.FindStringSubmatch(text); tm != nil {
		out.Tests = strings.TrimSpace(tm[1])
	}
	if sec := securityRE.FindStringSubmatch(text); sec != nil {
		out.Security = strings.TrimSpace(sec[1])
	}
	out.Focus = parseFocusAreas(text)
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

func parseFocusAreas(text string) []FocusArea {
	re := regexp.MustCompile(`(?mi)^focus:\s*$`)
	loc := re.FindStringIndex(text)
	if loc == nil {
		return nil
	}
	rest := text[loc[1]:]
	// Stop at comments: section if present after focus.
	if ci := regexp.MustCompile(`(?mi)^comments:\s*$`).FindStringIndex(rest); ci != nil {
		rest = rest[:ci[0]]
	}
	var out []FocusArea
	var cur *FocusArea
	flush := func() {
		if cur != nil && cur.Area != "" {
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
			cur = &FocusArea{}
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
		case "area":
			cur.Area = val
		case "why":
			cur.Why = val
		}
	}
	flush()
	return out
}

// FormatPRReviewGuide builds the GitHub review body (COMMENT) in Reviewer Guide shape.
func FormatPRReviewGuide(r *PRReviewResult, agentID, studioIssueURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### PR Reviewer Guide\n\n")
	fmt.Fprintf(&b, "**Verdict:** %s", r.Verdict)
	if agentID != "" {
		fmt.Fprintf(&b, " · agent `%s`", agentID)
	}
	b.WriteByte('\n')
	if r.Effort != "" {
		fmt.Fprintf(&b, "**Estimated effort to review:** %s/5\n", r.Effort)
	}
	if r.Tests != "" {
		fmt.Fprintf(&b, "**Tests:** %s\n", r.Tests)
	}
	if r.Security != "" {
		fmt.Fprintf(&b, "**Security:** %s\n", r.Security)
	}
	if r.Summary != "" {
		fmt.Fprintf(&b, "\n%s\n", r.Summary)
	}
	if len(r.Focus) > 0 {
		fmt.Fprintf(&b, "\n#### Recommended focus areas\n\n")
		for i, f := range r.Focus {
			if f.Why != "" {
				fmt.Fprintf(&b, "%d. **%s** — %s\n", i+1, f.Area, f.Why)
			} else {
				fmt.Fprintf(&b, "%d. **%s**\n", i+1, f.Area)
			}
		}
	}
	if studioIssueURL != "" {
		fmt.Fprintf(&b, "\nStudio: %s\n", studioIssueURL)
	}
	return b.String()
}

// RetryPrompt asks the agent to reply with only the schema.
func RetryPrompt(kind string) string {
	switch kind {
	case "intake":
		return "Reply with exactly this schema and nothing else:\nverdict: ok | needs-work\nsummary: one short paragraph\nquestions:\n- …"
	case "pr-review":
		return "Reply with exactly this schema and nothing else:\nverdict: lgtm | changes-requested\neffort: 1-5\ntests: yes | no | n/a\nsecurity: one short line\nsummary: one short paragraph\nfocus:\n- area: …\n  why: …\ncomments:\n- path: relative/file.go\n  line: 12\n  body: what is wrong"
	default:
		return "Reply with exactly the required schema, nothing else."
	}
}
