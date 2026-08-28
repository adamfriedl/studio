# Target hook (required for daily use)

GitHub Actions **cron is a backstop only** — it often does not fire on quiet repos.
Studio watch must be event-driven:

1. **Studio-side:** after implementer Start returns a `pr_url`, dispatch kicks `studio-watch` itself (first Phase 6 / comment pass).
2. **Target-side:** this hook `repository_dispatch`es studio on PR sync, reviews, review comments, merge, and (optional) CI `workflow_run` so later FollowUps do not wait on cron.

## Installed on allowlisted targets

| Repo | `workflow_run` CI names |
| --- | --- |
| `pad-lab` | `Deploy dashboard to GitHub Pages` |
| `studio` | _(none — PR events only; avoid watch/dispatch recurse)_ |
| `job-search` | _(none)_ |
| `adamfriedl.github.io` | _(none)_ |
| `homelab` | `Plan and apply` |
| `intake-desk` | `verify` |
| **`new:` (Phase 4)** | Auto: `ci` (from provision pack; e.g. `go-service`) |

**`new:` targets:** Studio upserts `.github/workflows/ci.yml` + `studio-hook.yml` and copies **`STUDIO_HOOK_TOKEN` only** after create — no manual step. Pack source of truth: `internal/provision/assets/`. The App private key stays on `adamfriedl/studio`.

**Existing** targets: install the hook + secrets by hand (table above). Prefer `STUDIO_HOOK_TOKEN` (same narrow PAT) over App secrets on those repos when you next touch them.

### `STUDIO_HOOK_TOKEN` (required for `new:`)

Fine-grained PAT (or classic with minimal scope) that can create `repository_dispatch` on **`adamfriedl/studio` only**. Store as Actions secret `STUDIO_HOOK_TOKEN` on studio; Phase 4 copies it onto each new target.

Each manually installed target needs `STUDIO_HOOK_TOKEN` (preferred) or legacy `STUDIO_APP_ID` + `STUDIO_APP_PRIVATE_KEY`.

Canonical copy to adapt: [`.github/workflows/studio-hook.yml`](../.github/workflows/studio-hook.yml) on studio (PR-events-only variant). pad-lab / homelab / intake-desk add a `workflow_run` block with their CI `name:` values — never include `studio-hook` itself. The `new:` pack hook already listens for `workflows: ["ci"]`.

## Dispatch types (studio `watch.yml`)

| Type | When |
| --- | --- |
| `studio-watch` | Generic kick (preferred) |
| `studio-checks-done` | Target CI `workflow_run` completed |
| `studio-review-activity` | PR review or inline review comment |
| `studio-ci-failed` | Legacy / CI failure alias |
| `studio-pr-merged` | Legacy / merge alias |

Payload (required): `issue_number`. Optional: `pr_url`, `sha`, `reason`.

## PR footer

```text
Studio: https://github.com/adamfriedl/studio/issues/N
```

No footer → hook skips (no dispatch).

## Anti-recursion

Do **not** use bare `check_suite` for “any CI” — this hook’s own run would re-fire. Prefer:

- `pull_request` / `pull_request_review` / `pull_request_review_comment`
- `workflow_run` with an **explicit** `workflows: ["Exact CI Name"]` list that never includes `studio-hook`
