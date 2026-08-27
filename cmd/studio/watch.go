package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adamfriedl/studio/internal/binding"
	"github.com/adamfriedl/studio/internal/catalog"
	gh "github.com/adamfriedl/studio/internal/github"
	"github.com/adamfriedl/studio/internal/prompt"
	"github.com/adamfriedl/studio/internal/worker"
	"github.com/adamfriedl/studio/internal/worker/cursor"
)

func cmdWatch(args []string) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	issueN := fs.Int("issue", 0, "single studio issue (0 = all open pr-open)")
	dryRun := fs.Bool("dry-run", false, "print actions only")
	catalogPath := fs.String("catalog", "repos.yaml", "path to repos.yaml")
	owner := fs.String("owner", "", "studio repo owner")
	repo := fs.String("repo", "", "studio repo name")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}

	c, err := catalog.Load(findCatalog(*catalogPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog: %v\n", err)
		return 1
	}
	studioOwner, studioRepo := resolveStudioRepo(*owner, *repo, c.Org)
	token := strings.TrimSpace(os.Getenv("STUDIO_GITHUB_TOKEN"))
	if !*dryRun && token == "" {
		fmt.Fprintln(os.Stderr, "STUDIO_GITHUB_TOKEN required unless --dry-run")
		return 1
	}
	ctx := context.Background()
	client := &gh.Client{Token: token}

	var numbers []int
	if *issueN > 0 {
		numbers = []int{*issueN}
	} else {
		issues, err := client.ListOpenIssuesByLabel(ctx, studioOwner, studioRepo, "pr-open")
		if err != nil {
			fmt.Fprintf(os.Stderr, "list issues: %v\n", err)
			return 1
		}
		for _, is := range issues {
			numbers = append(numbers, is.Number)
		}
	}
	if len(numbers) == 0 {
		fmt.Println("watch: nothing to do")
		return 0
	}

	code := 0
	for _, n := range numbers {
		if err := watchOne(ctx, client, studioOwner, studioRepo, n, *dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "watch #%d: %v\n", n, err)
			code = 1
		}
	}
	return code
}

func resolveStudioRepo(owner, repo, org string) (string, string) {
	if owner == "" || repo == "" {
		if gr := os.Getenv("GITHUB_REPOSITORY"); gr != "" {
			parts := strings.SplitN(gr, "/", 2)
			if len(parts) == 2 {
				if owner == "" {
					owner = parts[0]
				}
				if repo == "" {
					repo = parts[1]
				}
			}
		}
	}
	if owner == "" {
		owner = org
	}
	if repo == "" {
		repo = "studio"
	}
	return owner, repo
}

func watchOne(ctx context.Context, client *gh.Client, studioOwner, studioRepo string, issueN int, dryRun bool) error {
	issue, err := client.GetIssue(ctx, studioOwner, studioRepo, issueN)
	if err != nil {
		return err
	}
	b, err := binding.Parse(issue.Body)
	if err != nil {
		if !dryRun {
			_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, "Studio watch: binding corrupt — needs-human: "+err.Error())
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
		}
		return err
	}
	if b.PRURL == "" || b.Repo == "" {
		return fmt.Errorf("binding missing pr_url/repo")
	}
	owner, repo, err := gh.ParseOwnerRepo(b.Repo)
	if err != nil {
		return err
	}
	prNum, err := gh.ParsePRNumber(b.PRURL)
	if err != nil {
		return err
	}
	pr, err := client.GetPull(ctx, owner, repo, prNum)
	if err != nil {
		return err
	}

	if pr.Merged {
		fmt.Printf("#%d: PR merged — closing\n", issueN)
		if dryRun {
			return nil
		}
		b.PRStatus = "merged"
		body, err := binding.Upsert(issue.Body, b)
		if err != nil {
			return err
		}
		_ = client.UpdateIssue(ctx, studioOwner, studioRepo, issueN, body, nil)
		_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"done"})
		_ = client.RemoveLabel(ctx, studioOwner, studioRepo, issueN, "working")
		_ = client.RemoveLabel(ctx, studioOwner, studioRepo, issueN, "pr-open")
		_ = client.CloseIssue(ctx, studioOwner, studioRepo, issueN)
		_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, "Studio: target PR merged — closing issue.")
		return nil
	}
	if pr.State != "open" {
		fmt.Printf("#%d: PR %s (not merged) — skip\n", issueN, pr.State)
		return nil
	}

	checks, err := client.ListCheckRuns(ctx, owner, repo, pr.Head.SHA)
	if err != nil {
		return err
	}
	if gh.ChecksPending(checks) {
		fmt.Printf("#%d: checks pending\n", issueN)
		return nil
	}
	failed := gh.FailedChecks(checks)
	if len(failed) > 0 {
		return watchCIFix(ctx, client, studioOwner, studioRepo, issueN, issue, b, owner, repo, pr, failed, dryRun)
	}
	return watchReviews(ctx, client, studioOwner, studioRepo, issueN, issue, b, owner, repo, pr, dryRun)
}

func watchCIFix(ctx context.Context, client *gh.Client, studioOwner, studioRepo string, issueN int, issue *gh.Issue, b *binding.Binding, owner, repo string, pr *gh.PullRequest, failed []gh.CheckRun, dryRun bool) error {
	if b.AgentID == "" {
		if !dryRun {
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
		}
		return fmt.Errorf("CI failed but no agent_id")
	}
	var list strings.Builder
	for _, f := range failed {
		fmt.Fprintf(&list, "- %s (%s) %s\n", f.Name, f.Conclusion, f.HTMLURL)
	}
	logs, got, err := client.PrefetchFailedRunLogs(ctx, owner, repo, pr.Head.Ref, 8000)
	if err != nil {
		return err
	}
	if !got || strings.TrimSpace(logs) == "" {
		fmt.Printf("#%d: CI failed, no logs — needs-human\n", issueN)
		if !dryRun {
			_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, "Studio: CI failed but Actions logs could not be prefetched — needs-human.\n"+list.String())
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
		}
		return nil
	}
	p := prompt.CIFix(prompt.CIFixInput{
		PRURL:       pr.HTMLURL,
		SHA:         pr.Head.SHA,
		CheckList:   list.String(),
		LogExcerpts: logs,
	})
	fmt.Printf("#%d: CI FollowUp → %s\n", issueN, b.AgentID)
	if dryRun {
		fmt.Println(p)
		return nil
	}
	if strings.TrimSpace(os.Getenv("CURSOR_API_KEY")) == "" {
		return fmt.Errorf("CURSOR_API_KEY required")
	}
	w := cursor.Helper{Timeout: 45 * time.Minute}
	res, err := w.FollowUp(ctx, worker.FollowUpReq{AgentID: b.AgentID, Prompt: p})
	if err != nil {
		_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio: CI FollowUp failed: %v", err))
		_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
		return err
	}
	_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio: sent CI FollowUp (run `%s`, status %s)", res.RunID, res.Status))
	return nil
}

func watchReviews(ctx context.Context, client *gh.Client, studioOwner, studioRepo string, issueN int, issue *gh.Issue, b *binding.Binding, owner, repo string, pr *gh.PullRequest, dryRun bool) error {
	comments, err := client.ListPRReviewComments(ctx, owner, repo, pr.Number)
	if err != nil {
		return err
	}
	cursorID := int64(0)
	if b.ReviewCursor != "" {
		cursorID, _ = strconv.ParseInt(b.ReviewCursor, 10, 64)
	}
	var fresh []gh.PullComment
	maxID := cursorID
	for _, cmt := range comments {
		if cmt.ID <= cursorID {
			continue
		}
		fresh = append(fresh, cmt)
		if cmt.ID > maxID {
			maxID = cmt.ID
		}
	}
	if len(fresh) == 0 {
		fmt.Printf("#%d: no new review comments\n", issueN)
		return nil
	}
	if b.AgentID == "" {
		return fmt.Errorf("review comments but no agent_id")
	}
	var body strings.Builder
	for _, cmt := range fresh {
		fmt.Fprintf(&body, "- @%s on %s:%d (%s):\n  %s\n", cmt.User.Login, cmt.Path, cmt.Line, cmt.HTMLURL, strings.TrimSpace(cmt.Body))
	}
	p := prompt.Review(prompt.ReviewInput{PRURL: pr.HTMLURL, Comments: body.String()})
	fmt.Printf("#%d: review FollowUp (%d comments) → %s\n", issueN, len(fresh), b.AgentID)
	if dryRun {
		fmt.Println(p)
		return nil
	}
	if strings.TrimSpace(os.Getenv("CURSOR_API_KEY")) == "" {
		return fmt.Errorf("CURSOR_API_KEY required")
	}
	w := cursor.Helper{Timeout: 45 * time.Minute}
	res, err := w.FollowUp(ctx, worker.FollowUpReq{AgentID: b.AgentID, Prompt: p})
	if err != nil {
		_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio: review FollowUp failed: %v", err))
		_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
		return err
	}
	b.ReviewCursor = strconv.FormatInt(maxID, 10)
	newBody, err := binding.Upsert(issue.Body, b)
	if err != nil {
		return err
	}
	_ = client.UpdateIssue(ctx, studioOwner, studioRepo, issueN, newBody, nil)
	_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio: sent review FollowUp (run `%s`, status %s)", res.RunID, res.Status))
	return nil
}
