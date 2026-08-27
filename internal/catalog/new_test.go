package catalog

import "testing"

func TestNewRepoName(t *testing.T) {
	name, err := NewRepoName("Hello World API", "plain body")
	if err != nil || name != "hello-world-api" {
		t.Fatalf("title slug: got %q %v", name, err)
	}
	name, err = NewRepoName("ignored", "---\nname: notes-api\n---\nbody")
	if err != nil || name != "notes-api" {
		t.Fatalf("frontmatter name: got %q %v", name, err)
	}
	name, err = NewRepoName("ignored", "---\nrepo: boom-svc\n---\n")
	if err != nil || name != "boom-svc" {
		t.Fatalf("frontmatter repo: got %q %v", name, err)
	}
	name, err = NewRepoName("ignored", "### What to do\n\nx\n\n---\nname: buried-svc\n---\n")
	if err != nil || name != "buried-svc" {
		t.Fatalf("buried frontmatter: got %q %v", name, err)
	}
	name, err = NewRepoName("ignored", "### What to do\n\nx\n\n### New repo name\n\nform-svc\n")
	if err != nil || name != "form-svc" {
		t.Fatalf("issue form: got %q %v", name, err)
	}
	_, err = NewRepoName("bad", "---\nname: has space\n---\n")
	if err == nil {
		t.Fatal("expected invalid name error")
	}
}

func TestTemplateSourceNames(t *testing.T) {
	c := &Catalog{Templates: map[string]Template{"go-service": {From: "adamfriedl/template-go-svc"}}}
	if _, ok := c.TemplateSourceNames()["template-go-svc"]; !ok {
		t.Fatal("expected template-go-svc")
	}
}

func TestAppendRepo(t *testing.T) {
	c := &Catalog{Org: "adamfriedl", Repos: []Repo{{Name: "pad-lab"}}}
	if !c.AppendRepo("notes-api") {
		t.Fatal("expected append")
	}
	if c.AppendRepo("notes-api") {
		t.Fatal("expected no-op")
	}
	if !c.HasRepo("notes-api") {
		t.Fatal("missing notes-api")
	}
}

func TestAppendRepoYAML(t *testing.T) {
	in := []byte(`org: adamfriedl
defaults:
  branchPrefix: studio/
  draftPR: true
templates: {}
repos:
  - name: pad-lab
    # self-target note
  - name: studio
`)
	out, err := AppendRepoYAML(in, "notes-api")
	if err != nil {
		t.Fatal(err)
	}
	want := string(in) + "  - name: notes-api\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	again, err := AppendRepoYAML(out, "notes-api")
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(out) {
		t.Fatal("expected idempotent append")
	}
}
