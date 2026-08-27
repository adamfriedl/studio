package catalog

import (
	"os"
	"testing"
)

func TestResolveModelDefaults(t *testing.T) {
	_ = os.Unsetenv("STUDIO_CURSOR_MODEL")
	_ = os.Unsetenv("STUDIO_QC_MODEL")
	_ = os.Unsetenv("STUDIO_REVIEW_MODEL")
	if got := ResolveModel("implement", nil, ""); got != DefaultImplementModel {
		t.Fatalf("implement: %q", got)
	}
	if got := ResolveModel("qc", nil, ""); got != DefaultImplementModel {
		t.Fatalf("qc: %q", got)
	}
}

func TestResolveModelChain(t *testing.T) {
	t.Setenv("STUDIO_CURSOR_MODEL", "composer-2.5")
	t.Setenv("STUDIO_QC_MODEL", "grok-4.5")
	t.Setenv("STUDIO_REVIEW_MODEL", "")
	if got := ResolveModel("review", nil, ""); got != "grok-4.5" {
		t.Fatalf("review fallback: %q", got)
	}
	t.Setenv("STUDIO_REVIEW_MODEL", "grok-4.6")
	if got := ResolveModel("review", nil, ""); got != "grok-4.6" {
		t.Fatalf("review: %q", got)
	}
}

func TestResolveModelOverride(t *testing.T) {
	t.Setenv("STUDIO_CURSOR_MODEL", "composer-2.5")
	if got := ResolveModel("implement", []string{"model:grok-4.5"}, ""); got != "grok-4.5" {
		t.Fatalf("label: %q", got)
	}
	if got := ResolveModel("qc", nil, "---\nmodel: grok-4.6\n---\nbody"); got != "grok-4.6" {
		t.Fatalf("frontmatter: %q", got)
	}
}

func TestNormalizeModelRejectsThirdParty(t *testing.T) {
	_, err := NormalizeModel("claude-opus-5")
	if err == nil {
		t.Fatal("expected reject")
	}
	if got := ResolveModel("implement", []string{"model:claude-opus-5"}, ""); got != DefaultImplementModel {
		t.Fatalf("fallback: %q", got)
	}
}

func TestSpecAllowsStart(t *testing.T) {
	if SpecAllowsStart(nil, "") {
		t.Fatal("empty should block")
	}
	if !SpecAllowsStart([]string{"skip-qc"}, "") {
		t.Fatal("skip-qc")
	}
	if !SpecAllowsStart([]string{"spec-ok"}, "needs-work") {
		t.Fatal("spec-ok")
	}
	if !SpecAllowsStart(nil, "approved") {
		t.Fatal("approved")
	}
}
