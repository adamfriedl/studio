# studio

Shared Claude Code automation for my personal repos. One reusable workflow, [`claude-agent.yml`](.github/workflows/claude-agent.yml), gives every enabled repo an `@claude` teammate that implements issues, opens the PR, reviews it once, and takes follow-up requests in PR comments.

This repo used to be a Go orchestrator that drove Cursor cloud agents from issues filed here. That is retired; the last version is tagged [`studio-v1`](https://github.com/adamfriedl/studio/tree/studio-v1).

## How it works

```
issue or PR comment mentioning @claude  (in the target repo)
  → target repo's .github/workflows/claude.yml  (small caller)
  → studio/.github/workflows/claude-agent.yml@main
      job claude:  anthropics/claude-code-action implements the request on the runner
                   · issue  → new branch claude/issue-N-<timestamp>, then this workflow opens the PR
                   · open PR → commits pushed to that PR's branch
      job review:  only when job claude just opened a PR — one review pass, posted as a single
                   numbered PR comment (correctness, security, gaps vs. the issue; no nits)
  → you read the review, reply "@claude fix 1 and 3" for anything worth acting on, and merge
```

- Claude reads the target repo's `CLAUDE.md` (which imports `AGENTS.md`) for conventions and its verify steps. Keep those current — the runner has no access to `~/CLAUDE.md`.
- Only people with write access can trigger it, so public repos are safe.
- The review runs once per PR on purpose. Model-reviews-model loops mostly add churn, and every run spends the same subscription usage as interactive Claude Code.
- If Claude fails, the job prints its final error line (auth, usage limit, …) under **Show Claude error**.

## Enabled repos

`pad-lab`, `job-search`, `homelab`, `intake-desk`, `adamfriedl.github.io`

## Using it

1. Open an issue in the target repo and mention `@claude` in the title or body. Be specific: what to change, how to verify it, what the runner can't do (no cloud credentials).
2. Wait for the PR and the review comment (usually a few minutes).
3. Ask for changes with `@claude …` in a PR comment or review. Claude pushes to the same branch; no second review runs.
4. Verify locally if the change needs credentials the runner lacks (dbt against BigQuery, `terraform plan`), then merge.

## Adding a repo

1. Install the [Claude GitHub App](https://github.com/apps/claude) on it (skip if installed for all repositories).
2. Add `.github/workflows/claude.yml`:

   ```yaml
   name: Claude Code

   on:
     issue_comment:
       types: [created]
     pull_request_review_comment:
       types: [created]
     issues:
       types: [opened, assigned]
     pull_request_review:
       types: [submitted]

   jobs:
     claude:
       uses: adamfriedl/studio/.github/workflows/claude-agent.yml@main
       permissions:
         contents: write
         pull-requests: write
         issues: write
         id-token: write
         actions: read
       secrets: inherit
   ```

3. Add a `CLAUDE.md` (one line, `@AGENTS.md`, if the repo already has an `AGENTS.md`) with the repo's verify steps.
4. Run `scripts/claude-rollout.sh <repo>` in your own terminal. It checks the token with a live `claude -p` call, then sets the `CLAUDE_CODE_OAUTH_TOKEN` secret and allows Actions to open PRs.

## Secrets and settings

| What | Where | Notes |
|------|-------|-------|
| `CLAUDE_CODE_OAUTH_TOKEN` | Actions secret on each enabled repo | From `claude setup-token`; bills to the Claude subscription. Rotate by re-running `claude-rollout.sh` with a new token. |
| Allow GitHub Actions to create and approve pull requests | Settings → Actions → General, each enabled repo | Needed for the workflow to open PRs with `github.token`. |
| Claude GitHub App | Account installations | Mints the token Claude uses to push and comment. |

Nothing is stored in this repo. Personal accounts have no account-level Actions secrets, which is why the token lives on every repo.

## Caveats

- **PRs opened by this workflow don't trigger the repo's other `pull_request` workflows** (GitHub rule for `github.token`). Repos whose CI runs on branch pushes (`intake-desk`) are unaffected; `homelab`'s Terraform plan runs on the next push to the PR branch.
- **Callers pin `@main`.** A change to `claude-agent.yml` goes live in every repo on push — test it on `pad-lab` first.
- **No auto-merge.** Merging stays manual, and in `homelab` a merge applies Terraform.
