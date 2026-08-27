package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adamfriedl/studio/internal/binding"
	gh "github.com/adamfriedl/studio/internal/github"
	"github.com/adamfriedl/studio/internal/parse"
	"github.com/adamfriedl/studio/internal/prompt"
	"github.com/adamfriedl/studio/internal/worker"
	"github.com/adamfriedl/studio/internal/worker/cursor"
)

// continueOK false means stop (needs-spec or error). code is process exit when non-zero.
func runIntakeQC(ctx context.Context, client *gh.Client, studioOwner, studioRepo string, issueN int, issue *gh.Issue, targetURL, model string, existing *binding.Binding, dryRun bool) (spec *binding.Binding, continueOK bool, code int) {
	spec = &binding.Binding{}
	if existing != nil {
		*spec = *existing
	}

	intakePrompt := prompt.Intake(prompt.IntakeInput{
		IssueNumber: issueN,
		Title:       issue.Title,
		Body:        stripBinding(issue.Body),
		RepoURL:     targetURL,
	})
	fmt.Printf("intake: model=%s\n", model)
	if dryRun {
		fmt.Println("--- intake prompt ---")
		fmt.Println(intakePrompt)
		return spec, true, 0
	}

	_ = client.EnsureLabel(ctx, studioOwner, studioRepo, "needs-spec", "FBCA04", "Intake QC needs human edits")
	_ = client.EnsureLabel(ctx, studioOwner, studioRepo, "spec-ok", "0E8A16", "Human accepted spec after QC")
	_ = client.EnsureLabel(ctx, studioOwner, studioRepo, "skip-qc", "C5DEF5", "Bypass intake QC")

	w := cursor.Helper{Timeout: 45 * time.Minute}
	// Workspace = studio inbox repo (not the target). Prompt names the catalog target;
	// avoids giving QC write access to the target default branch.
	studioURL := fmt.Sprintf("https://github.com/%s/%s", studioOwner, studioRepo)
	res, err := w.Start(ctx, worker.StartReq{
		IssueNumber:  issueN,
		Title:        issue.Title,
		Body:         issue.Body,
		RepoURL:      studioURL,
		StartingRef:  "main",
		Model:        model,
		AutoCreatePR: false,
		Prompt:       intakePrompt,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "intake worker: %v\n", err)
		_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio intake failed: %v", err))
		_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
		return spec, false, 2
	}
	spec.SpecAgentID = res.AgentID
	text := firstNonEmpty(res.Message, "")
	parsed, perr := parse.ParseIntake(text)
	if perr != nil {
		fmt.Printf("intake: parse failed (%v) — retry once\n", perr)
		retry, rerr := w.FollowUp(ctx, worker.FollowUpReq{AgentID: res.AgentID, Prompt: parse.RetryPrompt("intake")})
		if rerr != nil {
			_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio intake parse/retry failed: %v\nraw: %s", rerr, truncate(text, 800)))
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
			spec.SpecStatus = "error"
			_ = upsertSpecOnly(ctx, client, studioOwner, studioRepo, issueN, issue.Body, existing, spec)
			return spec, false, 3
		}
		text = firstNonEmpty(retry.Message, text)
		parsed, perr = parse.ParseIntake(text)
		if perr != nil {
			_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, fmt.Sprintf("Studio intake unparseable after retry — needs-human.\nraw: %s", truncate(text, 800)))
			_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-human"})
			spec.SpecStatus = "error"
			_ = upsertSpecOnly(ctx, client, studioOwner, studioRepo, issueN, issue.Body, existing, spec)
			return spec, false, 3
		}
	}

	spec.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	var comment strings.Builder
	fmt.Fprintf(&comment, "Studio intake (`%s`): **%s**\n\n%s\n", res.AgentID, parsed.Verdict, parsed.Summary)
	if len(parsed.Questions) > 0 {
		comment.WriteString("\nQuestions / suggestions:\n")
		for _, q := range parsed.Questions {
			fmt.Fprintf(&comment, "- %s\n", q)
		}
	}

	if parsed.Verdict == "needs-work" {
		spec.SpecStatus = "needs-work"
		_ = upsertSpecOnly(ctx, client, studioOwner, studioRepo, issueN, issue.Body, existing, spec)
		_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, comment.String()+"\nEdit the issue, then label `spec-ok` or `skip-qc` to continue.")
		_ = client.AddLabels(ctx, studioOwner, studioRepo, issueN, []string{"needs-spec"})
		fmt.Println("intake: needs-work — stopped")
		return spec, false, 0
	}

	spec.SpecStatus = "approved"
	_ = upsertSpecOnly(ctx, client, studioOwner, studioRepo, issueN, issue.Body, existing, spec)
	_ = client.RemoveLabel(ctx, studioOwner, studioRepo, issueN, "needs-spec")
	_ = client.AddComment(ctx, studioOwner, studioRepo, issueN, comment.String())
	fmt.Println("intake: ok — continuing to implement")
	return spec, true, 0
}

func upsertSpecOnly(ctx context.Context, client *gh.Client, studioOwner, studioRepo string, issueN int, body string, existing, spec *binding.Binding) error {
	fresh, err := client.GetIssue(ctx, studioOwner, studioRepo, issueN)
	if err != nil {
		return err
	}
	merged := mergeSpecBinding(existing, spec)
	if prev, err := binding.Parse(fresh.Body); err == nil {
		merged = mergeSpecBinding(prev, spec)
	}
	newBody, err := binding.Upsert(fresh.Body, merged)
	if err != nil {
		return err
	}
	return client.UpdateIssue(ctx, studioOwner, studioRepo, issueN, newBody, nil)
}

func mergeSpecBinding(existing, spec *binding.Binding) *binding.Binding {
	out := &binding.Binding{}
	if existing != nil {
		*out = *existing
	}
	out.SpecStatus = spec.SpecStatus
	out.SpecAgentID = spec.SpecAgentID
	if spec.UpdatedAt != "" {
		out.UpdatedAt = spec.UpdatedAt
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
