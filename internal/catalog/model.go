package catalog

import (
	"fmt"
	"os"
	"strings"
)

const DefaultImplementModel = "grok-4.6"

// IncludedModels are Cursor Models pool IDs (generous included usage on paid plans).
// See https://cursor.com/docs/models-and-pricing — keep third-party models out for cost.
var IncludedModels = []string{
	"composer-2.5",
	"grok-4.5",
	"grok-4.6",
}

var includedSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(IncludedModels))
	for _, id := range IncludedModels {
		m[id] = struct{}{}
	}
	return m
}()

// NormalizeModel lowercases and accepts only IncludedModels. Empty → default.
func NormalizeModel(id string) (string, error) {
	id = strings.TrimSpace(strings.ToLower(id))
	if id == "" {
		return DefaultImplementModel, nil
	}
	// Common aliases
	switch id {
	case "composer", "composer2.5", "composer-2":
		id = "composer-2.5"
	case "grok4.5", "cursor-grok-4.5":
		id = "grok-4.5"
	case "grok", "grok4.6", "cursor-grok-4.6":
		id = "grok-4.6"
	}
	if _, ok := includedSet[id]; !ok {
		return "", fmt.Errorf("model %q not in Cursor included pool (allowed: %s)", id, strings.Join(IncludedModels, ", "))
	}
	return id, nil
}

// ResolveModel picks a Cursor model id for a role from the included pool only.
// Override: label model:<id> or frontmatter/form model: wins for all roles.
// Else: role env, then chain fallbacks to implementer default.
// Invalid override/env falls back to DefaultImplementModel (logged by caller via MustResolve or check error).
func ResolveModel(role string, labels []string, body string) string {
	m, err := ResolveModelStrict(role, labels, body)
	if err != nil {
		return DefaultImplementModel
	}
	return m
}

// ResolveModelStrict is like ResolveModel but returns an error if the chosen id is not allowlisted.
func ResolveModelStrict(role string, labels []string, body string) (string, error) {
	if m := modelOverride(labels, body); m != "" {
		return NormalizeModel(m)
	}
	implRaw := envOr("STUDIO_CURSOR_MODEL", DefaultImplementModel)
	impl, err := NormalizeModel(implRaw)
	if err != nil {
		impl = DefaultImplementModel
	}
	qcRaw := envOr("STUDIO_QC_MODEL", impl)
	qc, err := NormalizeModel(qcRaw)
	if err != nil {
		qc = impl
	}
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "qc", "intake":
		return qc, nil
	case "review", "pr-review", "reviewer":
		if v := strings.TrimSpace(os.Getenv("STUDIO_REVIEW_MODEL")); v != "" {
			return NormalizeModel(v)
		}
		return qc, nil
	default:
		return impl, nil
	}
}

// ModelOverride returns the raw per-issue model string if set (unvalidated).
func ModelOverride(labels []string, body string) string {
	return modelOverride(labels, body)
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
