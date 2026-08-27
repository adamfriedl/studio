package catalog

import "testing"

func TestTargetFromLabels(t *testing.T) {
	c := &Catalog{
		Org: "adamfriedl",
		Repos: []Repo{{Name: "pad-lab"}},
		Templates: map[string]Template{
			"go-service": {From: "adamfriedl/template-go-svc"},
		},
	}
	kind, name, url, err := c.TargetFromLabels([]string{"bug", "repo:pad-lab"})
	if err != nil || kind != "repo" || name != "pad-lab" || url != "https://github.com/adamfriedl/pad-lab" {
		t.Fatalf("got %s %s %s %v", kind, name, url, err)
	}
	_, _, _, err = c.TargetFromLabels([]string{"repo:nope"})
	if err == nil {
		t.Fatal("expected unknown repo error")
	}
	_, _, _, err = c.TargetFromLabels([]string{"enhancement"})
	if err == nil {
		t.Fatal("expected missing label error")
	}
	kind, name, url, err = c.TargetFromLabels([]string{"new:go-service"})
	if err != nil || kind != "new" || name != "go-service" || url != "adamfriedl/template-go-svc" {
		t.Fatalf("new: got %s %s %s %v", kind, name, url, err)
	}
}
