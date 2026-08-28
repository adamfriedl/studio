package parse

import (
	"strings"
	"testing"
)

func TestParseIntakeOK(t *testing.T) {
	raw := `Here is my take.

verdict: ok
summary: Clear enough to ship a small change.
questions:
- Confirm PORT env is optional
`
	r, err := ParseIntake(raw)
	if err != nil || r.Verdict != "ok" || r.Summary == "" || len(r.Questions) != 1 {
		t.Fatalf("got %#v err=%v", r, err)
	}
}

func TestParseIntakeNeedsWork(t *testing.T) {
	raw := `verdict: needs-work
summary: Missing acceptance criteria.
questions:
- What should /healthz return?
- Public or private?
`
	r, err := ParseIntake(raw)
	if err != nil || r.Verdict != "needs-work" || len(r.Questions) != 2 {
		t.Fatalf("got %#v err=%v", r, err)
	}
}

func TestParseIntakeMissing(t *testing.T) {
	_, err := ParseIntake("no schema here")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseIntakeBadVerdict(t *testing.T) {
	_, err := ParseIntake("verdict: maybe\nsummary: x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePRReview(t *testing.T) {
	raw := `verdict: changes-requested
summary: Missing test for edge case.
comments:
- path: main.go
  line: 12
  body: Handle empty body
- path: main_test.go
  line: 3
  body: Add table test
`
	r, err := ParsePRReview(raw)
	if err != nil || r.Verdict != "changes-requested" || len(r.Comments) != 2 {
		t.Fatalf("got %#v err=%v", r, err)
	}
	if r.Comments[0].Path != "main.go" || r.Comments[0].Line != 12 {
		t.Fatalf("comment0 %#v", r.Comments[0])
	}
}

func TestParsePRReviewGuide(t *testing.T) {
	raw := `verdict: changes-requested
effort: 3
tests: yes
security: No credential leaks in the diff.
summary: Flag the share intent and meta filtering.
focus:
- area: Activity launch flag
  why: Non-activity context may crash without FLAG_ACTIVITY_NEW_TASK
- area: Sensitive meta filtering
  why: Regex may miss non-standard secret names
comments:
- path: app/Share.kt
  line: 40
  body: Add FLAG_ACTIVITY_NEW_TASK
`
	r, err := ParsePRReview(raw)
	if err != nil {
		t.Fatal(err)
	}
	if r.Effort != "3" || r.Tests != "yes" || r.Security == "" || len(r.Focus) != 2 {
		t.Fatalf("guide fields %#v", r)
	}
	if r.Focus[0].Area != "Activity launch flag" || r.Focus[0].Why == "" {
		t.Fatalf("focus0 %#v", r.Focus[0])
	}
	body := FormatPRReviewGuide(r, "bc-test", "https://github.com/adamfriedl/studio/issues/9")
	for _, want := range []string{"PR Reviewer Guide", "3/5", "Tests:", "Security:", "Recommended focus areas", "Activity launch flag", "Studio:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("guide body missing %q:\n%s", want, body)
		}
	}
}

func TestParsePRReviewLGTM(t *testing.T) {
	r, err := ParsePRReview("verdict: lgtm\nsummary: Looks good.\ncomments:\n")
	if err != nil || r.Verdict != "lgtm" {
		t.Fatalf("got %#v err=%v", r, err)
	}
}
