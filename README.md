# Studio

Go CLI + GitHub Actions that turn **issues on this repo** into **Cursor Cloud Agents** on allowlisted personal repos, then follow the resulting draft PR (CI logs, review comments) until merge closes the studio issue.

Spec: [`PRD.md`](./PRD.md). If a later instruction conflicts, update the PRD.

## Threat model (v1)

Solo operator. Personal Cursor account. Fine-grained or classic PAT stored only as Actions secrets / local env — **never** in prompts, comments, or the agent environment.

**PAT blast radius:** `STUDIO_GITHUB_TOKEN` can read/write issues on **this** repo and open/comment on PRs, read checks, and read Actions logs on every repo listed in [`repos.yaml`](./repos.yaml). Today that allowlist is:

| Catalog name | GitHub repo |
| --- | --- |
| `pad-lab` | [adamfriedl/pad-lab](https://github.com/adamfriedl/pad-lab) |
| `studio` | [adamfriedl/studio](https://github.com/adamfriedl/studio) (self — harness/allowlist PRs; review workflows carefully) |
| `job-search` | [adamfriedl/job-search](https://github.com/adamfriedl/job-search) |
| `homelab` | [adamfriedl/homelab](https://github.com/adamfriedl/homelab) |
| `adamfriedl.github.io` | [adamfriedl/adamfriedl.github.io](https://github.com/adamfriedl/adamfriedl.github.io) |
| `intake-desk` | [adamfriedl/intake-desk](https://github.com/adamfriedl/intake-desk) |

**App blast radius:** `STUDIO_APP_ID` / `STUDIO_APP_PRIVATE_KEY` stay on **studio** only (dispatch/watch minting). Targets use `STUDIO_HOOK_TOKEN` for `studio-hook` (Phase 4 copies it onto `new:` repos; existing allowlisted targets migrated).

## Required secrets (Actions)

| Secret | Purpose |
| --- | --- |
| `CURSOR_API_KEY` | Cursor API; cloud agent create / resume / send |
| `STUDIO_GITHUB_TOKEN` | Cross-repo GitHub access / PAT fallback (see blast radius) |
| `STUDIO_APP_ID` / `STUDIO_APP_PRIVATE_KEY` | Preferred GitHub App token mint (studio workflows only) |
| `STUDIO_HOOK_TOKEN` | Narrow PAT for target→studio `repository_dispatch`; **required** for `new:` provision (copied onto new targets) |

## Models

Studio only allows the **Cursor Models** pool (included generous usage on paid plans) — not third-party API-priced models:

| ID | Notes |
| --- | --- |
| `grok-4.6` | Default implementer / QC / review (stronger included-pool general model) |
| `grok-4.5` | Cursor Grok |
| `composer-2.5` | Faster/cheaper included-pool option; still allowlisted |

Env: `STUDIO_CURSOR_MODEL` / `STUDIO_QC_MODEL` / `STUDIO_REVIEW_MODEL` (must be one of the above).  
Per-issue: label `model:composer-2.5`, frontmatter `model:`, or the issue-form dropdown. Invalid IDs fall back to `grok-4.6`.

## How to file work

1. Open an issue on **this** repo.
2. Add a target label:
   - `repo:<name>` from [`repos.yaml`](./repos.yaml) — existing allowlisted repo
   - `new:go-service` — create from [`template-go-svc`](https://github.com/adamfriedl/template-go-svc), allowlist, **provision CI + hook + `STUDIO_HOOK_TOKEN`**, then Start
3. Optional: **New repo name**, `model:` override, or frontmatter.
4. Write a small, testable scope.
5. **Intake QC runs by default.** If `needs-work`, edit the issue and label `spec-ok` or `skip-qc`. If ok, implementer Starts and opens a draft PR.
6. Watch (event-driven): dispatch kicks watch when a PR URL appears; targets with [`docs/target-hook.md`](./docs/target-hook.md) notify on review comments / sync / merge. Cron is a backstop only. (`new:` repos get the hook automatically.)
7. CI FollowUp, automated PR review (COMMENT), human review threads → implementer. Merge → studio issue closes as `done`.

Escape labels: `skip-qc`, `spec-ok`, `skip-review`.

Other labels: `working`, `pr-open`, `needs-human`, `needs-spec`, `done`.

## Watch / target hook

GitHub schedule cron is **unreliable** on quiet repos. Daily use requires:

- Studio `repository_dispatch` kick after Start returns `pr_url`
- Target workflow from [`docs/target-hook.md`](./docs/target-hook.md) + App secrets on the target (manual for existing allowlisted repos; automatic on `new:`)

## Local CLI

```bash
go test ./...
go run ./cmd/studio doctor --dry-run
STUDIO_DRY_LABELS=repo:pad-lab,skip-qc go run ./cmd/studio dispatch --issue 1 --dry-run
STUDIO_DRY_LABELS=repo:pad-lab go run ./cmd/studio dispatch --issue 1 --dry-run
```

## Verify

```bash
go test ./...
go run ./cmd/studio doctor --dry-run
```

## Status

See [`HANDOFF.md`](./HANDOFF.md). Spec: [`PRD.md`](./PRD.md).
