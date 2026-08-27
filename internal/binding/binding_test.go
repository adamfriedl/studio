package binding

import (
	"errors"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	in := &Binding{
		AgentID:  "bc-abc",
		Worker:   "cursor",
		Repo:     "adamfriedl/pad-lab",
		Branch:   "studio/1-fix",
		PRURL:    "https://github.com/adamfriedl/pad-lab/pull/1",
		PRNumber: "1",
		PRStatus: "open",
	}
	body, err := Upsert("## Task\n\nFix the dashboard.\n", in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if out.AgentID != in.AgentID || out.PRURL != in.PRURL || out.Repo != in.Repo {
		t.Fatalf("round trip mismatch: %+v", out)
	}
	// upsert again replaces, does not duplicate
	in.AgentID = "bc-def"
	body2, err := Upsert(body, in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(body2, "<!-- studio:v1") != 1 {
		t.Fatalf("expected one block, body:\n%s", body2)
	}
	out2, err := Parse(body2)
	if err != nil {
		t.Fatal(err)
	}
	if out2.AgentID != "bc-def" {
		t.Fatalf("agent_id=%q", out2.AgentID)
	}
}

func TestCorruptMissing(t *testing.T) {
	_, err := Parse("no binding here")
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("got %v", err)
	}
}

func TestCorruptMultiple(t *testing.T) {
	body := `<!-- studio:v1
agent_id: a
-->
<!-- studio:v1
agent_id: b
-->`
	_, err := Parse(body)
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("got %v", err)
	}
	_, err = Upsert(body, &Binding{AgentID: "c"})
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("upsert got %v", err)
	}
}
