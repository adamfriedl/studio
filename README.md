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

## Required secrets (Actions)

| Secret | Purpose |
| --- | --- |
| `CURSOR_API_KEY` | Cursor API; cloud agent create / resume / send |
| `STUDIO_GITHUB_TOKEN` | Cross-repo GitHub access (see blast radius above) |

## How to file work

1. Open an issue on **this** repo.
2. Add label `repo:pad-lab` or `repo:studio` (self) — names from `repos.yaml`.
3. Write a small, testable scope in the body.
4. Studio starts a cloud agent on the target; agent opens a **draft** PR. You merge.

For `repo:studio`, prefer allowlist/docs/CLI tweaks over editing `.github/workflows` until watch is solid.

Labels Studio uses: `working`, `pr-open`, `needs-human`, `done` (plus later QC/review labels — see PRD).

## Local CLI

```bash
go test ./...
go run ./cmd/studio doctor --dry-run
STUDIO_DRY_LABELS=repo:pad-lab go run ./cmd/studio dispatch --issue 1 --dry-run
```

Live dispatch needs `CURSOR_API_KEY` + `STUDIO_GITHUB_TOKEN`. Actions workflow: `.github/workflows/dispatch.yml`.

Cursor cloud helper: `scripts/cursor-helper/` (TypeScript wrapper around `@cursor/sdk`).

`doctor --dry-run` prints the catalog and does not call network APIs.

## Verify

```bash
go test ./...
go run ./cmd/studio doctor --dry-run
```

## Status

- **Phase 0** — skeleton (this commit family)
- **Phase 1+** — see PRD §15
