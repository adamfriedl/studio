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
	"github.com/adamfriedl/studio/internal/run"
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
		if err := watchOne(ctx, client, c, studioOwner, studioRepo, n, *dryRun); err != nil {
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

func watchOne(ctx context.Context, client *gh.Client, cat *catalog.Catalog, studioOwner, studioRepo string, issueN int, dryRun bool) error {
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
	if !b.ReadyForWatch() {
		// Soft-skip: hook can kick before Start writes pr_url (exit 0, retry later).
		fmt.Printf("#%d: binding not ready (missing pr_url/repo) — skip\n", issueN)
		return nil
	}
	if !cat.AllowsFullName(b.Repo) {
		if !dryRun {
			_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, "Studio watch: binding repo not in catalog allowlist — needs-human: "+b.Repo)
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
		}
		return fmt.Errorf("repo %q not allowlisted", b.Repo)
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
		// Closed without merge — do not mark done; human decides abandon vs new PR.
		fmt.Printf("#%d: PR closed unmerged — needs-human\n", issueN)
		if dryRun {
			return nil
		}
		b.PRStatus = "closed"
		body, err := binding.Upsert(issue.Body, b)
		if err != nil {
			return err
		}
		_ = client.UpdateIssue(ctx, studioOwner, studioRepo, issueN, body, nil)
		_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
		_ = client.RemoveLabel(ctx, studioOwner, studioRepo, issueN, "working")
		_ = client.RemoveLabel(ctx, studioOwner, studioRepo, issueN, "pr-open")
		_ = client.AddComment(ctx, studioOwner, studioRepo, issueN,
			"Studio: target PR was closed without merging — needs-human. Close this issue, or open a new PR and update the binding / re-dispatch.")
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
		return watchCIFix(ctx, client, cat, studioOwner, studioRepo, issueN, issue, b, owner, repo, pr, failed, dryRun)
	}

	var labelNames []string
	for _, l := range issue.Labels {
		labelNames = append(labelNames, l.Name)
	}
	if err := runPRReview(ctx, client, cat, studioOwner, studioRepo, issueN, issue, labelNames, b, owner, repo, pr, dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "watch #%d pr-review: %v\n", issueN, err)
		// Continue to human review comments so implementer still sees threads.
	}
	// Refresh binding after pr-review may have upserted.
	if !dryRun {
		if refreshed, err := client.GetIssue(ctx, studioOwner, studioRepo, issueN); err == nil {
			issue = refreshed
			if nb, err := binding.Parse(issue.Body); err == nil {
				b = nb
			}
		}
	}
	return watchReviews(ctx, client, cat, studioOwner, studioRepo, issueN, issue, b, owner, repo, pr, dryRun)
}

func watchCIFix(ctx context.Context, client *gh.Client, cat *catalog.Catalog, studioOwner, studioRepo string, issueN int, issue *gh.Issue, b *binding.Binding, owner, repo string, pr *gh.PullRequest, failed []gh.CheckRun, dryRun bool) error {
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
	if b.Branch == "" {
		b.Branch = pr.Head.Ref
	}
	p := prompt.CIFix(prompt.CIFixInput{
		PRURL:       pr.HTMLURL,
		SHA:         pr.Head.SHA,
		CheckList:   list.String(),
		LogExcerpts: logs,
	})
	fmt.Printf("#%d: CI FollowUp → %s\n", issueN, firstNonEmpty(b.AgentID, "(replace)"))
	if dryRun {
		fmt.Println(p)
		return nil
	}
	return sendWatchPrompt(ctx, client, cat, studioOwner, studioRepo, issueN, issue, b, p, "CI", "")
}

func watchReviews(ctx context.Context, client *gh.Client, cat *catalog.Catalog, studioOwner, studioRepo string, issueN int, issue *gh.Issue, b *binding.Binding, owner, repo string, pr *gh.PullRequest, dryRun bool) error {
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
	skipped := 0
	for _, cmt := range comments {
		if cmt.ID <= cursorID {
			continue
		}
		if cmt.ID > maxID {
			maxID = cmt.ID
		}
		// Advance past implementer-bot replies without FollowUp — otherwise
		// each cursor[bot] thread reply re-enters watch and echoes.
		if gh.SkipReviewFollowUp(cmt.User.Login) {
			skipped++
			continue
		}
		fresh = append(fresh, cmt)
	}
	if len(fresh) == 0 {
		if maxID > cursorID && !dryRun {
			fmt.Printf("#%d: advanced review_cursor past %d bot-only comment(s)\n", issueN, skipped)
			return advanceReviewCursor(ctx, client, studioOwner, studioRepo, issueN, issue, b, strconv.FormatInt(maxID, 10))
		}
		fmt.Printf("#%d: no new review comments\n", issueN)
		return nil
	}
	if b.Branch == "" {
		b.Branch = pr.Head.Ref
	}
	var body strings.Builder
	for _, cmt := range fresh {
		fmt.Fprintf(&body, "- @%s on %s:%d (%s):\n  %s\n", cmt.User.Login, cmt.Path, cmt.Line, cmt.HTMLURL, strings.TrimSpace(cmt.Body))
	}
	p := prompt.Review(prompt.ReviewInput{PRURL: pr.HTMLURL, Comments: body.String()})
	fmt.Printf("#%d: review FollowUp (%d comments, skipped %d bot) → %s\n", issueN, len(fresh), skipped, firstNonEmpty(b.AgentID, "(replace)"))
	if dryRun {
		fmt.Println(p)
		return nil
	}
	// Cursor includes skipped IDs so bot replies are not re-fed next tick.
	return sendWatchPrompt(ctx, client, cat, studioOwner, studioRepo, issueN, issue, b, p, "review", strconv.FormatInt(maxID, 10))
}

func advanceReviewCursor(ctx context.Context, client *gh.Client, studioOwner, studioRepo string, issueN int, issue *gh.Issue, b *binding.Binding, cursor string) error {
	b.ReviewCursor = cursor
	fresh, err := client.GetIssue(ctx, studioOwner, studioRepo, issueN)
	if err != nil {
		return err
	}
	newBody, err := binding.Upsert(fresh.Body, b)
	if err != nil {
		return err
	}
	return client.UpdateIssue(ctx, studioOwner, studioRepo, issueN, newBody, nil)
}

// sendWatchPrompt FollowUps the implementer, or replaces the agent on the existing branch if resume fails.
// If reviewCursor is non-empty, upserts it in the same binding write (with agent_id on replace).
func sendWatchPrompt(ctx context.Context, client *gh.Client, cat *catalog.Catalog, studioOwner, studioRepo string, issueN int, issue *gh.Issue, b *binding.Binding, promptText, kind, reviewCursor string) error {
	if strings.TrimSpace(os.Getenv("CURSOR_API_KEY")) == "" {
		return fmt.Errorf("CURSOR_API_KEY required")
	}
	oldID := b.AgentID
	model := firstNonEmpty(b.Model, catalog.ResolveModel("implement", nil, ""), catalog.DefaultImplementModel)
	w := cursor.Helper{Timeout: 45 * time.Minute}
	res, updated, replaced, err := run.SendOrReplace(ctx, w, b, promptText, model, cat.AllowsFullName)
	if err != nil {
		_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio: %s FollowUp/replace failed: %v", kind, err))
		_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
		return err
	}
	if reviewCursor != "" {
		updated.ReviewCursor = reviewCursor
	}
	if replaced || reviewCursor != "" {
		fresh, err := client.GetIssue(ctx, studioOwner, studioRepo, issueN)
		if err != nil {
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
			return err
		}
		newBody, err := binding.Upsert(fresh.Body, updated)
		if err != nil {
			_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, "Studio: binding corrupt after watch — needs-human: "+err.Error())
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
			return err
		}
		if err := client.UpdateIssue(ctx, studioOwner, studioRepo, issueN, newBody, nil); err != nil {
			_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio: failed to persist binding after %s (agent `%s`) — needs-human: %v", kind, updated.AgentID, err))
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
			return err
		}
	}
	if replaced {
		msg := fmt.Sprintf("Studio: worker replaced (`%s` → `%s`) after resume failure; sent %s prompt (run `%s`, status %s)",
			firstNonEmpty(oldID, "none"), updated.AgentID, kind, res.RunID, res.Status)
		_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, msg)
		fmt.Println(msg)
		return nil
	}
	_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio: sent %s FollowUp (run `%s`, status %s)", kind, res.RunID, res.Status))
	return nil
}
