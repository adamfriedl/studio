package catalog

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Defaults struct {
	BranchPrefix string `yaml:"branchPrefix"`
	DraftPR      bool   `yaml:"draftPR"`
}

type Template struct {
	From string `yaml:"from"`
}

type Repo struct {
	Name  string `yaml:"name"`
	Owner string `yaml:"owner,omitempty"`
}

type Catalog struct {
	Org       string              `yaml:"org"`
	Defaults  Defaults            `yaml:"defaults"`
	Templates map[string]Template `yaml:"templates"`
	Repos     []Repo              `yaml:"repos"`
}

func Load(path string) (*Catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Catalog
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	if c.Org == "" {
		return nil, fmt.Errorf("catalog: org is required")
	}
	if c.Templates == nil {
		c.Templates = map[string]Template{}
	}
	return &c, nil
}

func (c *Catalog) ResolveRepo(name string) (owner, repo string, ok bool) {
	for _, r := range c.Repos {
		if r.Name != name {
			continue
		}
		owner = r.Owner
		if owner == "" {
			owner = c.Org
		}
		return owner, r.Name, true
	}
	return "", "", false
}

func (c *Catalog) FullName(name string) (string, bool) {
	owner, repo, ok := c.ResolveRepo(name)
	if !ok {
		return "", false
	}
	return owner + "/" + repo, true
}
