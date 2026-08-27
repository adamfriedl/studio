package run

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/adamfriedl/studio/internal/binding"
	"github.com/adamfriedl/studio/internal/worker"
)

// Worker is the subset of worker.Worker used by watch follow-ups.
type Worker interface {
	Start(ctx context.Context, req worker.StartReq) (worker.Result, error)
	FollowUp(ctx context.Context, req worker.FollowUpReq) (worker.Result, error)
}

// SendOrReplace tries FollowUp on the bound agent_id. If that fails (or agent_id
// is empty), starts a replacement cloud agent on the existing branch, clears the
// old agent_id in the returned binding, and sets the new one.
//
// Keeps branch / pr_url. Caller must Upsert the returned binding (single writer).
// allowlisted must return true for binding.Repo before Start (catalog gate).
func SendOrReplace(ctx context.Context, w Worker, b *binding.Binding, prompt, model string, allowlisted func(repo string) bool) (worker.Result, *binding.Binding, bool, error) {
	if b == nil {
		return worker.Result{}, nil, false, fmt.Errorf("nil binding")
	}
	out := *b // copy; mutate returned binding only

	if strings.TrimSpace(b.AgentID) != "" {
		res, err := w.FollowUp(ctx, worker.FollowUpReq{AgentID: b.AgentID, Prompt: prompt})
		if err == nil {
			return res, &out, false, nil
		}
		// Fall through to replace.
		return replaceAgent(ctx, w, &out, prompt, model, err, allowlisted)
	}
	return replaceAgent(ctx, w, &out, prompt, model, fmt.Errorf("missing agent_id"), allowlisted)
}

func replaceAgent(ctx context.Context, w Worker, b *binding.Binding, prompt, model string, reason error, allowlisted func(repo string) bool) (worker.Result, *binding.Binding, bool, error) {
	branch := strings.TrimSpace(b.Branch)
	repo := strings.TrimSpace(b.Repo)
	if branch == "" || repo == "" {
		return worker.Result{}, b, false, fmt.Errorf("cannot replace agent: need branch+repo (%v)", reason)
	}
	if allowlisted != nil && !allowlisted(repo) {
		return worker.Result{}, b, false, fmt.Errorf("cannot replace agent: repo %q not in catalog allowlist", repo)
	}
	repoURL := repo
	if !strings.HasPrefix(repoURL, "https://") {
		repoURL = "https://github.com/" + repo
	}
	b.AgentID = "" // clear before Start (PRD); Upsert omits empty agent_id
	b.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	p := prompt
	if reason != nil {
		p = fmt.Sprintf("[studio] Previous worker could not be resumed (%v). Continue on branch %s / existing PR. Do not open a new PR.\n\n%s", reason, branch, prompt)
	}

	res, err := w.Start(ctx, worker.StartReq{
		RepoURL:      repoURL,
		StartingRef:  branch,
		BranchName:   branch,
		Model:        model,
		AutoCreatePR: false, // PR already exists
		Prompt:       p,
	})
	if err != nil {
		return res, b, true, fmt.Errorf("replace agent after %v: %w", reason, err)
	}
	if strings.TrimSpace(res.AgentID) == "" {
		return res, b, true, fmt.Errorf("replace agent after %v: empty agent_id", reason)
	}
	b.AgentID = res.AgentID
	b.Worker = "cursor"
	b.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return res, b, true, nil
}
