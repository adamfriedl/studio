package prompt

import (
	"fmt"
	"strings"
)

type CIFixInput struct {
	PRURL       string
	SHA         string
	CheckList   string
	LogExcerpts string
}

func CIFix(in CIFixInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[studio] CI failed on %s (HEAD %s)\n\n", in.PRURL, in.SHA)
	fmt.Fprintf(&b, "Failed checks:\n%s\n\n", in.CheckList)
	fmt.Fprintf(&b, "Prefetched GitHub Actions logs (authoritative). Do not guess from status text alone.\n\n")
	fmt.Fprintf(&b, "%s\n\n", in.LogExcerpts)
	fmt.Fprintf(&b, "Fix the product/test code. Push to the same branch. Do not rewrite CI YAML unless these logs show setup/install you broke.\n")
	return b.String()
}

type ReviewInput struct {
	PRURL    string
	Comments string
}

func Review(in ReviewInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[studio] Review comments on %s:\n\n", in.PRURL)
	fmt.Fprintf(&b, "%s\n\n", in.Comments)
	fmt.Fprintf(&b, "Address valid items. Reply on threads if you can. Skip praise. If a comment is wrong, say so on the PR and do not change the code.\n")
	return b.String()
}
