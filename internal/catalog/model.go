package catalog

import (
	"os"
	"strings"
)

const DefaultImplementModel = "composer-2.5"

// ResolveModel picks a Cursor model id for a role.
// Override: label model:<id> or frontmatter model: wins for all roles.
// Else: role env, then chain fallbacks to implementer default.
func ResolveModel(role string, labels []string, body string) string {
	if m := modelOverride(labels, body); m != "" {
		return m
	}
	impl := envOr("STUDIO_CURSOR_MODEL", DefaultImplementModel)
	qc := envOr("STUDIO_QC_MODEL", impl)
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "qc", "intake":
		return qc
	case "review", "pr-review", "reviewer":
		if v := strings.TrimSpace(os.Getenv("STUDIO_REVIEW_MODEL")); v != "" {
			return v
		}
		return qc
	default:
		return impl
	}
}

func modelOverride(labels []string, body string) string {
	for _, l := range labels {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "model:") {
			m := strings.TrimSpace(strings.TrimPrefix(l, "model:"))
			if m != "" {
				return m
			}
		}
	}
	if fm, ok := parseFrontmatter(body); ok {
		if m := strings.TrimSpace(fm["model"]); m != "" {
			return m
		}
	}
	if m := issueFormModelRE.FindStringSubmatch(body); len(m) == 2 {
		v := strings.TrimSpace(m[1])
		if v != "" && !strings.EqualFold(v, "_No response_") {
			return v
		}
	}
	return ""
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

// HasLabel reports whether labels contain name (exact).
func HasLabel(labels []string, name string) bool {
	for _, l := range labels {
		if strings.TrimSpace(l) == name {
			return true
		}
	}
	return false
}

// SpecAllowsStart is true when QC may be skipped for implementer Start.
func SpecAllowsStart(labels []string, specStatus string) bool {
	if HasLabel(labels, "skip-qc") || HasLabel(labels, "spec-ok") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(specStatus)) {
	case "ok", "approved", "skipped":
		return true
	default:
		return false
	}
}
