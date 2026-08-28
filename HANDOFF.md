# Studio handoff — 2026-08-28

Short pickup note. Spec: [`PRD.md`](./PRD.md). App: [`docs/github-app.md`](./docs/github-app.md).

## Done

| Phase | What |
| --- | --- |
| **0–4** | CLI, dispatch, watch, App, `new:`, watch harden, closed-unmerged → needs-human |
| **5** | Intake QC on by default; `needs-spec` / `spec-ok` / `skip-qc`; separate `spec_agent_id` |
| **6** | SHA-gated PR reviewer (COMMENT only, max 2 rounds); then implementer FollowUp on threads |
| **Models** | `STUDIO_CURSOR_MODEL` / `STUDIO_QC_MODEL` / `STUDIO_REVIEW_MODEL` + per-issue `model:` / label |
| **Watch kick** | Dispatch `studio-watch` after Start returns `pr_url`; pad-lab target-hook + App secrets; cron = backstop |

**Allowlist:** pad-lab, studio, job-search, homelab, adamfriedl.github.io, intake-desk.

## Pick up here

1. ~~Live smoke paths~~ — done 2026-08-28:
   - Happy path: studio#3 / pad-lab#2 (QC ok → PR → lgtm → merge-close)
   - QC gate: studio#4 / pad-lab#3 (`needs-work` → edit → `spec-ok` → PR → lgtm → merge-close)
   - Phase 6: studio#5 / pad-lab#4 (`skip-qc` + secrets-in-git ask → **changes-requested** → implementer FollowUp → safe rewrite → merge-close). Note: round 2 briefly re-litigated issue text vs security; acceptance was updated to the safe guidance.
2. Copy pad-lab `studio-hook` + App secrets to other allowlisted targets as needed.
3. **Phase 6.1 — Reviewer Guide shape** (planned): reshape Phase 6 review body toward a compact “PR Reviewer Guide” (inspired by `github-actions[bot]` guides on work PRs): short effort/risk header (tests present? security blurb?), then a few ranked focus areas with *why* — still COMMENT-only, still structured `verdict`/`summary`/`comments` for the implementer. Not Bugbot; keep included-pool models.
4. Optional: raise default models in workflows once a stronger cloud id is confirmed.

## Quick verify

```bash
go test ./...
go run ./cmd/studio doctor --dry-run
STUDIO_DRY_LABELS=repo:pad-lab,skip-qc go run ./cmd/studio dispatch --issue 1 --dry-run
STUDIO_DRY_LABELS=repo:pad-lab go run ./cmd/studio dispatch --issue 1 --dry-run
```
