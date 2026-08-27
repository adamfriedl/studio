package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adamfriedl/studio/internal/binding"
	"github.com/adamfriedl/studio/internal/catalog"
	gh "github.com/adamfriedl/studio/internal/github"
	"github.com/adamfriedl/studio/internal/parse"
	"github.com/adamfriedl/studio/internal/prompt"
	"github.com/adamfriedl/studio/internal/worker"
	"github.com/adamfriedl/studio/internal/worker/cursor"
)

// runPRReview runs Phase 6 SHA-gated reviewer. Updates binding in place via upsert.
// Returns after posting GitHub review (or skip). Does not FollowUp the implementer.
func runPRReview(ctx context.Context, client *gh.Client, cat *catalog.Catalog, studioOwner, studioRepo string, issueN int, issue *gh.Issue, labelNames []string, b *binding.Binding, owner, repo string, pr *gh.PullRequest, dryRun bool) error {
	if catalog.HasLabel(labelNames, "skip-review") {
		fmt.Printf("#%d: pr-review skipped (skip-review)\n", issueN)
		return nil
	}
	if strings.EqualFold(b.ReviewStatus, "error") {
		fmt.Printf("#%d: pr-review skipped (review_status=error — needs-human)\n", issueN)
		return nil
	}
	rounds, _ := strconv.Atoi(b.ReviewRounds)
	if rounds >= 2 {
		fmt.Printf("#%d: pr-review skipped (max rounds)\n", issueN)
		return nil
	}
	if b.ReviewSHA != "" && b.ReviewSHA == pr.Head.SHA {
		fmt.Printf("#%d: pr-review skipped (sha already reviewed)\n", issueN)
		return nil
	}

	files, err := client.ListPullFiles(ctx, owner, repo, pr.Number)
	if err != nil {
		return err
	}
	diff := gh.FormatDiff(files, 60000)
	model := catalog.ResolveModel("review", labelNames, stripBinding(issue.Body))
	p := prompt.PRReview(prompt.PRReviewInput{
		IssueNumber: issueN,
		Title:       issue.Title,
		Body:        stripBinding(issue.Body),
		PRURL:       pr.HTMLURL,
		SHA:         pr.Head.SHA,
		Diff:        diff,
	})
	fmt.Printf("#%d: pr-review model=%s sha=%s\n", issueN, model, pr.Head.SHA[:min(7, len(pr.Head.SHA))])
	if dryRun {
		fmt.Println(p)
		return nil
	}
	if strings.TrimSpace(os.Getenv("CURSOR_API_KEY")) == "" {
		return fmt.Errorf("CURSOR_API_KEY required")
	}
	_ = client.EnsureLabel(ctx, studioOwner, studioRepo, "skip-review", "C5DEF5", "Bypass Phase 6 PR reviewer")

	w := cursor.Helper{Timeout: 45 * time.Minute}
	res, err := w.Start(ctx, worker.StartReq{
		IssueNumber:  issueN,
		Title:        issue.Title,
		Body:         issue.Body,
		RepoURL:      "https://github.com/" + owner + "/" + repo,
		StartingRef:  pr.Head.Ref,
		Model:        model,
		AutoCreatePR: false,
		Prompt:       p,
	})
	if err != nil {
		_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio pr-review failed: %v", err))
		_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
		return err
	}

	text := firstNonEmpty(res.Message, "")
	parsed, perr := parse.ParsePRReview(text)
	if perr != nil {
		retry, rerr := w.FollowUp(ctx, worker.FollowUpReq{AgentID: res.AgentID, Prompt: parse.RetryPrompt("pr-review")})
		if rerr != nil {
			_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio pr-review parse/retry failed: %v\nraw: %s", rerr, truncate(text, 800)))
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
			b.ReviewStatus = "error"
			b.ReviewAgentID = res.AgentID
			return persistReviewBinding(ctx, client, studioOwner, studioRepo, issueN, issue.Body, b)
		}
		text = firstNonEmpty(retry.Message, text)
		parsed, perr = parse.ParsePRReview(text)
		if perr != nil {
			_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio pr-review unparseable after retry — needs-human.\nraw: %s", truncate(text, 800)))
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
			b.ReviewStatus = "error"
			b.ReviewAgentID = res.AgentID
			return persistReviewBinding(ctx, client, studioOwner, studioRepo, issueN, issue.Body, b)
		}
	}

	b.ReviewAgentID = res.AgentID
	b.ReviewSHA = pr.Head.SHA
	b.ReviewRounds = strconv.Itoa(rounds + 1)
	b.ReviewStatus = parsed.Verdict
	b.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	reviewBody := fmt.Sprintf("Studio automated review (`%s`): **%s**\n\n%s\n\nStudio: https://github.com/%s/%s/issues/%d",
		res.AgentID, parsed.Verdict, parsed.Summary, studioOwner, studioRepo, issueN)
	var inline []gh.ReviewCommentIn
	for _, c := range parsed.Comments {
		if c.Path == "" || c.Body == "" || c.Line <= 0 {
			continue
		}
		inline = append(inline, gh.ReviewCommentIn{Path: c.Path, Line: c.Line, Body: c.Body})
	}
	// COMMENT only — never APPROVE / request-changes event (human merges).
	if err := client.CreatePullReview(ctx, owner, repo, pr.Number, reviewBody, "COMMENT", inline); err != nil {
		// Retry without inline comments if line positions are invalid.
		if len(inline) > 0 {
			err = client.CreatePullReview(ctx, owner, repo, pr.Number, reviewBody+"\n\n(inline comments omitted — positions invalid)", "COMMENT", nil)
		}
		if err != nil {
			_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, "Studio pr-review could not post GitHub review: "+err.Error())
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
			return err
		}
	}
	_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio: posted pr-review **%s** (agent `%s`, round %s)", parsed.Verdict, res.AgentID, b.ReviewRounds))
	return persistReviewBinding(ctx, client, studioOwner, studioRepo, issueN, issue.Body, b)
}

func persistReviewBinding(ctx context.Context, client *gh.Client, studioOwner, studioRepo string, issueN int, body string, b *binding.Binding) error {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
