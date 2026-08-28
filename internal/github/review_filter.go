package github

import "strings"

// SkipReviewFollowUp reports whether a PR review comment should not trigger
// an implementer FollowUp. Cursor cloud agents reply on threads as cursor[bot];
// re-feeding those replies causes an echo loop. Phase 6 inline comments from
// adamfriedl-studio must still be delivered to the implementer.
func SkipReviewFollowUp(login string) bool {
	l := strings.ToLower(strings.TrimSpace(login))
	switch l {
	case "cursor[bot]", "cursor":
		return true
	}
	// Other Cursor-published bot logins, if any.
	if strings.HasSuffix(l, "[bot]") && strings.Contains(l, "cursor") {
		return true
	}
	return false
}
