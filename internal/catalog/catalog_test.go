package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndResolve(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.yaml")
	content := []byte(`
org: adamfriedl
defaults:
  branchPrefix: studio/
  draftPR: true
templates: {}
repos:
  - name: pad-lab
  - name: boomlab
    owner: otheruser
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	full, ok := c.FullName("pad-lab")
	if !ok || full != "adamfriedl/pad-lab" {
		t.Fatalf("pad-lab: got %q ok=%v", full, ok)
	}
	full, ok = c.FullName("boomlab")
	if !ok || full != "otheruser/boomlab" {
		t.Fatalf("boomlab: got %q ok=%v", full, ok)
	}
	if _, ok := c.FullName("missing"); ok {
		t.Fatal("expected missing to fail")
	}
}
