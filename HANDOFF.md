# Studio handoff — 2026-08-27

Short pickup note. Spec lives in [`PRD.md`](./PRD.md). App ops: [`docs/github-app.md`](./docs/github-app.md).

## Done

| Phase | What |
| --- | --- |
| **0** | Go CLI, `repos.yaml`, binding, `doctor`, tests, README |
| **1** | Cursor helper (`scripts/cursor-helper`), `dispatch`, `dispatch.yml` (label-only `repo:`/`new:`), idempotent if binding has `agent_id`. Live: pad-lab + studio→homelab#8 |
| **2–3 (MVP)** | `studio watch`, `watch.yml` (15m + `repository_dispatch`), CI log FollowUp, review-comment FollowUp, close-on-merge. Prefer App token; PAT fallback |
| **App** | `adamfriedl-studio` install **157096911**; **All repositories** (confirmed). Secrets `STUDIO_APP_ID` / `STUDIO_APP_PRIVATE_KEY` (+ still `CURSOR_API_KEY`, `STUDIO_GITHUB_TOKEN`). Workflows mint App token for `owner: adamfriedl` **without** hardcoded repo list. New repos inherit App access; `repos.yaml` is still the dispatch gate. |
| **4** | `new:` implemented: create from template → ensure `repo:<name>` label → commit allowlist to `repos.yaml` → `Worker.Start`. Template [`adamfriedl/template-go-svc`](https://github.com/adamfriedl/template-go-svc) (`is_template`); catalog key `go-service`; label `new:go-service`. |

**Allowlist today:** `pad-lab`, `studio`, `job-search`, `homelab`, `adamfriedl.github.io`, `intake-desk`. Default model `composer-2.5` via `STUDIO_CURSOR_MODEL`.

**Tokens (not in git):** `~/Documents/tokens/studio.md`, `~/Documents/tokens/studio-github-app.json`.

## Operator TODO (optional)

- Apply [`docs/target-hook.md`](./docs/target-hook.md) on a pilot target (homelab) so watch is event-driven; poll stays backstop.
- Grant App **Administration: write** so `new:` generate can drop the PAT fallback (`STUDIO_GITHUB_PAT`).

## Pick up here

1. **Watch hardening** (PRD §15 Phase 2 leftover): replace agent on resume failure; keep binding single-writer discipline.
2. **Do not start Phase 5/6** until ~two weeks of real Phase 3 use (PRD §16).

## Live Phase 4 smoke (done 2026-08-27)

- Issue [#2](https://github.com/adamfriedl/studio/issues/2) `new:go-service` → `adamfriedl/new-repo-smoke-test` + catalog commit + [PR #1](https://github.com/adamfriedl/new-repo-smoke-test/pull/1).
- First Cursor Start failed (race); re-dispatch succeeded with Cursor GitHub = All repositories.
## Quick verify

```bash
go test ./...
go run ./cmd/studio doctor --dry-run
STUDIO_DRY_LABELS=new:go-service STUDIO_DRY_TITLE='hello notes' go run ./cmd/studio dispatch --issue 99 --dry-run
```

Dirty tree note: local edits to `scripts/register-github-app.py` may be uncommitted — check before next push.
