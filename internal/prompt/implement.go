package prompt

import (
	"fmt"
	"strings"
)

type ImplementInput struct {
	IssueNumber    int
	Title          string
	Body           string
	RepoURL        string
	Branch         string
	StudioIssueURL string
}

func Implement(in ImplementInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are implementing a GitHub issue from the studio inbox.\n\n")
	fmt.Fprintf(&b, "Issue: studio#%d — %s\n", in.IssueNumber, in.Title)
	fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(in.Body))
	fmt.Fprintf(&b, "Target repository: %s\n", in.RepoURL)
	fmt.Fprintf(&b, "Use branch: %s (create if needed).\n", in.Branch)
	fmt.Fprintf(&b, "Open a **draft** pull request when the change is testable. Do not merge.\n\n")
	fmt.Fprintf(&b, "Constraints:\n")
	fmt.Fprintf(&b, "- Keep the diff small. Do not drive-by refactor.\n")
	fmt.Fprintf(&b, "- Discover and run this repo's tests (README, Makefile, go test, package scripts).\n")
	fmt.Fprintf(&b, "- Do not edit .github/workflows or CI unless the failure is install/setup caused by this change.\n")
	fmt.Fprintf(&b, "- Do not add secrets or change auth unless the issue requires it.\n")
	fmt.Fprintf(&b, "- PR body must include Studio: %s and a short summary.\n", in.StudioIssueURL)
	return b.String()
}
