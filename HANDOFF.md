# Studio handoff — 2026-08-27

Short pickup note. Spec: [`PRD.md`](./PRD.md). App: [`docs/github-app.md`](./docs/github-app.md).

## Done

| Phase | What |
| --- | --- |
| **0–4** | CLI, dispatch, watch, App, `new:`, watch harden, closed-unmerged → needs-human |
| **5** | Intake QC on by default; `needs-spec` / `spec-ok` / `skip-qc`; separate `spec_agent_id` |
| **6** | SHA-gated PR reviewer (COMMENT only, max 2 rounds); then implementer FollowUp on threads |
| **Models** | `STUDIO_CURSOR_MODEL` / `STUDIO_QC_MODEL` / `STUDIO_REVIEW_MODEL` + per-issue `model:` / label |

**Allowlist:** pad-lab, studio, job-search, homelab, adamfriedl.github.io, intake-desk.

## Pick up here

1. Live smoke: one intake `needs-work` → edit → `spec-ok`; one full ok→PR; one reviewer `changes-requested` → implementer.
2. Optional: target-hook on a pilot repo (event-driven watch).
3. Optional: raise default models in workflows once a stronger cloud id is confirmed.

## Quick verify

```bash
go test ./...
go run ./cmd/studio doctor --dry-run
STUDIO_DRY_LABELS=repo:pad-lab,skip-qc go run ./cmd/studio dispatch --issue 1 --dry-run
STUDIO_DRY_LABELS=repo:pad-lab go run ./cmd/studio dispatch --issue 1 --dry-run
```
