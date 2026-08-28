# Studio handoff — 2026-08-28

Short pickup note. Spec: [`PRD.md`](./PRD.md). App: [`docs/github-app.md`](./docs/github-app.md).

## Done

| Phase | What |
| --- | --- |
| **0–4** | CLI, dispatch, watch, App, `new:`, watch harden, closed-unmerged → needs-human |
| **5** | Intake QC on by default; `needs-spec` / `spec-ok` / `skip-qc`; separate `spec_agent_id` |
| **6** | SHA-gated PR reviewer (COMMENT only, max 2 rounds); then implementer FollowUp on threads |
| **6.1** | Reviewer Guide body: effort / tests / security / focus + summary; soft-parse optional fields |
| **Models** | `STUDIO_CURSOR_MODEL` / `STUDIO_QC_MODEL` / `STUDIO_REVIEW_MODEL` + per-issue `model:` / label |
| **Watch kick** | Dispatch `studio-watch` after Start returns `pr_url`; pad-lab target-hook + App secrets; cron = backstop |
| **Echo fix** | Skip `cursor[bot]` review comments when building FollowUps |

**Allowlist:** pad-lab, studio, job-search, homelab, adamfriedl.github.io, intake-desk.

**Live smoke (2026-08-28):** studio#3/#4/#5 → pad-lab#2/#3/#4 (happy path, QC needs-work→spec-ok, Phase 6 changes-requested→FollowUp). Smoke debris removed from pad-lab README.

## Pick up here

1. Copy pad-lab `studio-hook` + App secrets to other allowlisted targets as needed.
2. Optional: raise default models in workflows once a stronger cloud id is confirmed.
3. Optional: live glance that a new Phase 6 review posts the Reviewer Guide header.

## Quick verify

```bash
go test ./...
go run ./cmd/studio doctor --dry-run
STUDIO_DRY_LABELS=repo:pad-lab,skip-qc go run ./cmd/studio dispatch --issue 1 --dry-run
STUDIO_DRY_LABELS=repo:pad-lab go run ./cmd/studio dispatch --issue 1 --dry-run
```
