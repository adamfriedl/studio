package catalog

import (
	"os"
	"testing"
)

func TestResolveModelDefaults(t *testing.T) {
	t.Setenv("STUDIO_CURSOR_MODEL", "")
	t.Setenv("STUDIO_QC_MODEL", "")
	t.Setenv("STUDIO_REVIEW_MODEL", "")
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
	t.Setenv("STUDIO_CURSOR_MODEL", "impl-model")
	t.Setenv("STUDIO_QC_MODEL", "qc-model")
	t.Setenv("STUDIO_REVIEW_MODEL", "")
	if got := ResolveModel("review", nil, ""); got != "qc-model" {
		t.Fatalf("review fallback: %q", got)
	}
	t.Setenv("STUDIO_REVIEW_MODEL", "rev-model")
	if got := ResolveModel("review", nil, ""); got != "rev-model" {
		t.Fatalf("review: %q", got)
	}
}

func TestResolveModelOverride(t *testing.T) {
	t.Setenv("STUDIO_CURSOR_MODEL", "impl-model")
	if got := ResolveModel("implement", []string{"model:gpt-5"}, ""); got != "gpt-5" {
		t.Fatalf("label: %q", got)
	}
	if got := ResolveModel("qc", nil, "---\nmodel: opus\n---\nbody"); got != "opus" {
		t.Fatalf("frontmatter: %q", got)
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
