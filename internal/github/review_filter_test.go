package github

import "testing"

func TestSkipReviewFollowUp(t *testing.T) {
	cases := []struct {
		login string
		skip  bool
	}{
		{"adamfriedl", false},
		{"adamfriedl-studio[bot]", false}, // Phase 6 inlines must reach implementer
		{"cursor[bot]", true},
		{"Cursor[bot]", true},
		{"cursor", true},
		{"dependabot[bot]", false},
	}
	for _, tc := range cases {
		if got := SkipReviewFollowUp(tc.login); got != tc.skip {
			t.Fatalf("%q: got %v want %v", tc.login, got, tc.skip)
		}
	}
}
