package parse

import "testing"

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

func TestParsePRReviewLGTM(t *testing.T) {
	r, err := ParsePRReview("verdict: lgtm\nsummary: Looks good.\ncomments:\n")
	if err != nil || r.Verdict != "lgtm" {
		t.Fatalf("got %#v err=%v", r, err)
	}
}
