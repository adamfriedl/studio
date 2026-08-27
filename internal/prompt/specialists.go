package prompt

import (
	"fmt"
	"strings"
)

type IntakeInput struct {
	IssueNumber int
	Title       string
	Body        string
	RepoURL     string
}

func Intake(in IntakeInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are reviewing a GitHub issue before any implementation starts.\n\n")
	fmt.Fprintf(&b, "Issue: studio#%d — %s\n", in.IssueNumber, in.Title)
	fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(in.Body))
	fmt.Fprintf(&b, "Target repository: %s (catalog only; do not clone or edit it).\n\n", in.RepoURL)
	fmt.Fprintf(&b, "Decide if this is complete enough to implement. Look for missing acceptance criteria, ambiguous scope, unspecified behavior, contradictions, and obvious holes. Do not implement. Do not invent product requirements; ask or suggest.\n\n")
	fmt.Fprintf(&b, "Reply with exactly:\n")
	fmt.Fprintf(&b, "verdict: ok | needs-work\n")
	fmt.Fprintf(&b, "summary: one short paragraph\n")
	fmt.Fprintf(&b, "questions:\n")
	fmt.Fprintf(&b, "- (concrete edits the human could make to the issue body)\n\n")
	fmt.Fprintf(&b, "If the issue is thin but the intent is obvious, verdict ok and note residual risk in summary. Prefer shipping a small clear issue over blocking on polish.\n")
	return b.String()
}

type PRReviewInput struct {
	IssueNumber int
	Title       string
	Body        string
	PRURL       string
	SHA         string
	Diff        string
}

func PRReview(in PRReviewInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are reviewing a pull request. You do not implement. You do not push. You do not merge.\n\n")
	fmt.Fprintf(&b, "Issue: studio#%d — %s\n", in.IssueNumber, in.Title)
	fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(in.Body))
	fmt.Fprintf(&b, "PR: %s (HEAD %s)\n\n", in.PRURL, in.SHA)
	fmt.Fprintf(&b, "Diff (authoritative, from GitHub). Do not guess files that are not listed.\n%s\n\n", in.Diff)
	fmt.Fprintf(&b, "Review against the issue. Look for missing acceptance criteria, wrong behavior, security/auth mistakes, tests that do not cover the change, and scope creep. Skip nitpicks and praise. Do not demand drive-by refactors. CI already ran; do not restate failing checks unless the diff clearly will fail again.\n\n")
	fmt.Fprintf(&b, "Reply with exactly:\n")
	fmt.Fprintf(&b, "verdict: lgtm | changes-requested\n")
	fmt.Fprintf(&b, "summary: one short paragraph\n")
	fmt.Fprintf(&b, "comments:\n")
	fmt.Fprintf(&b, "- path: relative/file.go\n")
	fmt.Fprintf(&b, "  line: 12\n")
	fmt.Fprintf(&b, "  body: what is wrong and what would fix it\n\n")
	fmt.Fprintf(&b, "If the change matches the issue and residual risk is small, verdict lgtm. Prefer a small mergeable PR over a perfect one.\n")
	return b.String()
}
