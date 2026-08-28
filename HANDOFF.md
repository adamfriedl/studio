# Studio handoff — 2026-08-28

Short pickup note. Spec: [`PRD.md`](./PRD.md). App: [`docs/github-app.md`](./docs/github-app.md).

## Done

| Phase | What |
| --- | --- |
| **0–4** | CLI, dispatch, watch, App, `new:`, watch harden, closed-unmerged → needs-human |
| **5** | Intake QC on by default; `needs-spec` / `spec-ok` / `skip-qc`; separate `spec_agent_id` |
| **6** | SHA-gated PR reviewer (COMMENT only, max 2 rounds); then implementer FollowUp on threads |
| **6.1** | Reviewer Guide body: effort / tests / security / focus + summary; soft-parse optional fields |
| **Models** | Default `grok-4.6` for implement / QC / review; allowlist also `grok-4.5`, `composer-2.5` + per-issue override |
| **Watch kick** | Dispatch `studio-watch` after Start returns `pr_url`; target-hook on all allowlisted repos + `STUDIO_HOOK_TOKEN`; cron = backstop |
| **`new:` provision** | After create: upsert pack CI + `studio-hook`, copy `STUDIO_HOOK_TOKEN` only (fail closed); App private key stays on studio |
| **Echo fix** | Skip `cursor[bot]` review comments when building FollowUps |

**Allowlist:** pad-lab, studio, job-search, homelab, adamfriedl.github.io, intake-desk.

**Live smoke (2026-08-28):** studio#3/#4/#5 → pad-lab#2/#3/#4 (happy path, QC needs-work→spec-ok, Phase 6 changes-requested→FollowUp). Smoke debris removed from pad-lab README.

**Live smoke `new:` (2026-08-28):** studio#6 → pad-go-smoke — create + pack CI/hook + `STUDIO_HOOK_TOKEN` + draft PR + Phase 6 lgtm; repo deleted after smoke. Hook fix: local reads use `github.token`. Targets migrated to `STUDIO_HOOK_TOKEN` (App secrets removed except on studio). Phase 6.1 glance (studio#7 → pad-lab#5): `### PR Reviewer Guide` on grok-4.6; PR discarded.

## Pick up here

1. Use Studio on a real `repo:` issue when you have one.

## Quick verify

```bash
go test ./...
go run ./cmd/studio doctor --dry-run
STUDIO_DRY_LABELS=repo:pad-lab,skip-qc go run ./cmd/studio dispatch --issue 1 --dry-run
STUDIO_DRY_LABELS=repo:pad-lab go run ./cmd/studio dispatch --issue 1 --dry-run
```
