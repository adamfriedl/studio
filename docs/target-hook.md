# Target hook (required for daily use)

GitHub Actions **cron is a backstop only** — it often does not fire on quiet repos.
Studio watch must be event-driven:

1. **Studio-side:** after implementer Start returns a `pr_url`, dispatch kicks `studio-watch` itself (first Phase 6 / comment pass).
2. **Target-side:** this hook `repository_dispatch`es studio on PR sync, reviews, review comments, merge, and (optional) CI `workflow_run` so later FollowUps do not wait on cron.

Studio does **not** auto-edit target workflows. Copy into each allowlisted target (pilot: `pad-lab`).

## Dispatch types (studio `watch.yml`)

| Type | When |
| --- | --- |
| `studio-watch` | Generic kick (preferred) |
| `studio-checks-done` | Target CI `workflow_run` completed |
| `studio-review-activity` | PR review or inline review comment |
| `studio-ci-failed` | Legacy / CI failure alias |
| `studio-pr-merged` | Legacy / merge alias |

Payload (required): `issue_number`. Optional: `pr_url`, `sha`, `reason`.

## Secrets on each target

Prefer the Studio GitHub App (same as studio repo):

- `STUDIO_APP_ID`
- `STUDIO_APP_PRIVATE_KEY`

Fallback: `STUDIO_HOOK_TOKEN` — PAT that can `POST …/dispatches` on `adamfriedl/studio`.

## PR footer

```text
Studio: https://github.com/adamfriedl/studio/issues/N
```

No footer → hook skips (no dispatch).

## Anti-recursion

Do **not** use bare `check_suite` for “any CI” — this hook’s own run would re-fire. Prefer:

- `pull_request` / `pull_request_review` / `pull_request_review_comment` (covers no-CI repos like pad-lab draft docs PRs)
- `workflow_run` with an **explicit** `workflows: ["Exact CI Name"]` list that never includes `studio-hook`

## Workflow

Live pilot: [`pad-lab/.github/workflows/studio-hook.yml`](https://github.com/adamfriedl/pad-lab/blob/main/.github/workflows/studio-hook.yml).

Copy that file; adjust the `workflow_run.workflows` list to your target’s CI job names (or drop `workflow_run` if the target has no PR checks).
