package catalog

import (
	"fmt"
	"strings"
)

// TargetFromLabels picks repo:<name> or new:<template> from issue labels.
// Unknown / missing → error suitable for needs-human.
func (c *Catalog) TargetFromLabels(labels []string) (kind, name, repoURL string, err error) {
	var repoName, templateName string
	for _, l := range labels {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "repo:"):
			repoName = strings.TrimPrefix(l, "repo:")
		case strings.HasPrefix(l, "new:"):
			templateName = strings.TrimPrefix(l, "new:")
		}
	}
	if repoName != "" && templateName != "" {
		return "", "", "", fmt.Errorf("both repo: and new: labels present")
	}
	if repoName != "" {
		full, ok := c.FullName(repoName)
		if !ok {
			return "", "", "", fmt.Errorf("unknown repo %q", repoName)
		}
		return "repo", repoName, "https://github.com/" + full, nil
	}
	if templateName != "" {
		t, ok := c.Templates[templateName]
		if !ok || t.From == "" {
			return "", "", "", fmt.Errorf("unknown template %q", templateName)
		}
		// Caller creates from template (t.From), allowlists, then Starts.
		return "new", templateName, t.From, nil
	}
	return "", "", "", fmt.Errorf("missing repo:<name> or new:<template> label")
}
