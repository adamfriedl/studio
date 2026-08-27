<!--
Studio PRD. Reconstructed from phone video, then revised 2026-08-27
(binding writer, watch latency, Cursor risk, PAT threat model, structured I/O).
Original editor tab: bot PRD.md
-->

# Studio

A Go CLI and GitHub Actions workflows that turn **issues on this repo** into **Cursor Cloud Agents** on allowlisted personal repos (or a new repo from a template) then follow the resulting pull request. The target repo's existing GitHub Actions stay the CI gate.

This file is the spec. If a later instruction conflicts, update this file.

**Working name:** Studio. Rename before Phase 0 if you want (`dispatch`, `foreman`, …). Change the module path, `studio:v1` prefix, and `Studio:` PR footer together.

**Biggest bet:** Cursor Cloud Start / resume / draft-PR reliability — not the Go scaffolding. Treat Phase 1 as an API spike with calendar time, not “wire the happy path and hope.”

## 1. What to build

A person files work as a GitHub Issue here. A label or frontmatter picks a target repo. GitHub Actions runs a Go program that starts (or resumes) a coding agent. The agent opens a draft PR on the target. Studio watches that PR's checks and review comments and sends follow-up prompts. When the PR merges, Studio closes its own issue.

The coding agent is **not** this Go program. Go admits work, builds prompts, talks to GitHub, and calls a **Worker**. v1 worker is Cursor Cloud. Later workers (Claude Code, Codex) implement the same interface.

## 2. Goals

1. Inbox is **GitHub Issues on this repo**. Label `repo:<name>` or `new:<template>`, or YAML frontmatter, selects the target.
2. At most **one** in-flight run per issue (concurrency group `studio-{issue-number}`, `cancel-in-progress: false`).
3. A Cursor Cloud agent implements the issue, pushes a branch, opens a **draft PR**.
4. Target-repo Actions run as they already do. Studio reads Check Runs. On failure it prefetches **Actions job logs** and follow-ups the **worker** agent id when possible.
5. Review comments on the target PR (human, or Phase 6 reviewer) become a follow-up on the **implementer** `agent_id`.
6. Merge closes the **studio** issue. Closing the target PR will not do that; Studio must.
7. A second worker implementation later does not rewrite intake or GitHub code.
8. After the first real red→green CI loop, **event-driven watch** (target → studio) is preferred over 15‑minute polling for CI failure and merge. Poll remains a backstop.

## Not in v1

- Jira, Linear, Slack, a web UI.
- A folder of markdown files as the source of truth (an importer that opens issues can come later).
- Running the coding agent on a Raspberry Pi.
- Extra specialist agents **in Phases 0–3**. Intake QC is Phase 5; PR reviewer is Phase 6. Both after the implementer path works.
- An **org-wide GitHub App**. v1 assumes a **personal** Cursor account and a fine-grained PAT (or user-owned App) on allowlisted personal repos. Org App / multi-user is a different threat model — do not pretend the PAT design scales there.
- Editing `.github/workflows` on the target unless logs clearly show this PR broke setup (prompt text, not a parser).

## 3. Constraints

| | |
| --- | --- |
| **Language** | Go 1.23+. Module `github.com/<user>/studio` (replace `<user>`). |
| **Worker v1** | Cursor Cloud. Call `https://api.cursor.com` with a Bearer key. Prefer Go HTTP if create + follow-up + resume work; else a 60–80 line TypeScript/Python helper that only wraps official `Agent.create` / resume / send. Do not write a general Go Cursor SDK. |
| **Inbox** | Issues on this repo only. |
| **Targets** | Allowlist in `repos.yaml`. Unknown repo → comment, label `needs-human`, do not start a worker. Each target keeps its own Actions. |
| **CI** | This repo's Actions run Studio. Target Actions stay the CI gate. |
| **Hosting** | GitHub Actions. No always-on server. |
| **Later** | Optional Pi 4 (ARM) as owner only: pull or webhook, SQLite, call Cursor Cloud. Never the coding VM. |
| **Secrets** | `CURSOR_API_KEY` and `STUDIO_GITHUB_TOKEN` as Actions secrets. Never put them in the agent environment or prompts. Cursor's GitHub integration handles push / draft PRs for cloud agents. Personal Cursor account. |
| **Threat model** | Solo operator, personal allowlisted repos. `STUDIO_GITHUB_TOKEN` can read/write PRs, checks, and Actions logs on every listed target — treat that blast radius as intentional and documented in the README. |

Default `GITHUB_TOKEN` cannot open PRs on other repos. `STUDIO_GITHUB_TOKEN` must comment on studio issues and read/write PRs, checks, and Actions logs on all listed targets.

## 4. Happy path

1. Open an issue: title + body. Add label `repo:notes-api` or `new:go-service` (or frontmatter).
2. `dispatch.yml` starts (concurrency `studio-${{ issue.number }}`).
3. `studio dispatch --issue N`:
   - `repo:<name>`: resolve from `repos.yaml`.
   - `new:`: create the repo from the template in the same run, then dispatch against that URL.
   - `Worker.Start` with the issue text and constraints.
4. Agent opens a draft PR. CLI confirms via GitHub API. Comment the PR URL, label `pr-open`.
5. Target CI runs. `studio watch --issue N` (poll and/or `repository_dispatch`): `pending` → wait; `failed` with logs → `FollowUp`; `failed` without logs → `needs-human`; success → wait for review/merge; merged → close issue.
6. Review comments: `studio review --issue N` with new threads since `review_cursor`. Same implementer `agent_id`.
7. Merge + close issue, label `done`.

## 5. Architecture

```
this repo (issues + CLI + workflows)
  → concurrency studio-{issue}
  → cmd/studio → Worker → Cursor Cloud (v1); Claude / Codex later
  → Issues API (inbox + binding)
  → GitHub REST on allowlisted targets (PRs, checks, job logs)
  → Target repo Actions (unchanged)
  → optional: target workflow_run → repository_dispatch → studio watch
```

**Owner** = the Actions job plus concurrency. No long-running process in v1.

**State** = issue labels + one machine-parseable HTML comment on the issue. SQLite only if a Pi owner is added later.

### Binding rules (non-negotiable)

- **Single writer:** only `cmd/studio` (via Actions or local CLI) may create or rewrite the `<!-- studio:v1 ... -->` block. Humans may edit issue body/labels; they must not hand-edit the binding comment.
- **Upsert in place:** never duplicate the comment. Replace the existing block atomically in one issue-update call.
- **Parse failure → stop:** if the comment is missing when required, malformed, or ambiguous (two blocks, unknown keys that matter), do **not** best-effort merge. Comment what failed, label `needs-human`, exit `3`.
- Concurrent workflow runs are serialized by the issue concurrency group; still treat a mid-write read as corruption if checksum/`updated_at` goes backwards.

### Labels

| Label | Meaning |
| --- | --- |
| `working` | Worker started |
| `pr-open` | Binding has `pr_url` |
| `needs-human` | Unknown repo, create failed, CI failed with no logs, binding parse failure, or unparseable agent structured output after retry |
| `needs-spec` | Intake QC found holes; do not Start until `spec-ok` or `skip-qc` |
| `spec-ok` | Human accepted the spec (original or after edits); dispatch may Start |
| `skip-qc` | Bypass intake; dispatch may Start |
| `skip-review` | Bypass the Phase 6 PR reviewer; humans still review as in Phase 3 |
| `done` | PR merged; issue closed |
| `repo:<name>` | Catalog short name |
| `new:<template>` | Create from template key |

Do not build a formal state machine. Do not `FollowUp` without a stored `agent_id`, unless resume failed and you start a replacement agent on the **existing** branch.

## 6. Catalog — `repos.yaml`

```yaml
org: YOUR_GITHUB_USER
defaults:
  branchPrefix: studio/
  draftPR: true
templates:
  go-service:
    from: YOUR_GITHUB_USER/template-go-svc
repos:
  - name: notes-api
  - name: boomlab
    owner: YOUR_GITHUB_USER   # optional override
```

**Resolution, first match:**

1. Label `repo:<name>` → `repos[name]`
2. Body frontmatter `repo:` → same
3. Label `new:<template>` → `templates`
4. Else: comment, `needs-human`, exit `1`

Unknown name is the same as 4.

For `new:`: create via `gh repo create` from the template; name from frontmatter or the issue title slug. Do not silently rewrite `repos.yaml` unless a later phase adds a catalog PR.

## 7. Issue binding

Upsert in place (never duplicate) one HTML comment on the studio issue:

```html
<!-- studio:v1
agent_id: bc-XXXXXXXX
worker: cursor
repo: owner/notes-api
branch: studio/12-short-slug
pr_url: https://github.com/owner/notes-api/pull/87
pr_number: 87
pr_status: open
spec_status: ok
spec_agent_id: bc-YYYYYYYY
review_agent_id: bc-ZZZZZZZZ
review_sha: abcdef0
review_rounds: 1
review_status: changes-requested
review_cursor: ...
updated_at: 2026-08-25T18:00:00Z
-->
```

- `agent_id`: required after a successful Start; implementer id (not QC or PR-reviewer).
- `spec_status` (Phase 5): `pending` | `needs-work` | `approved` | `skipped`. If absent → Phases 1–3 behavior (dispatch may Start).
- `review_status` (Phase 6): `pending` | `changes-requested` | `approved` | `skipped`. If absent → Phase 3 behavior (human PR comments only).
- `pr_url`: required before CI/review follow-ups.

If resume fails: clear `agent_id`, keep `branch` / `pr_url`. Start a new agent on that branch. Comment that the worker was replaced.

Also post a normal comment: agent URL (if any) and PR link.

Target PR body must include `Studio: <this-repo>#N` (studio issue link).

Git is the source of truth for code. Cloud VMs go idle and may re-clone from the branch; uncommitted junk is gone.

## 8. Worker interface

Package `internal/worker`. Implement this before the Cursor client details.

```go
package worker

type StartReq struct {
    IssueNumber  int
    Title        string
    Body         string
    RepoURL      string
    StartingRef  string
    BranchName   string
    Model        string // env STUDIO_CURSOR_MODEL, default composer-2.5
    AutoCreatePR bool
    Prompt       string
}

type FollowUpReq struct {
    AgentID string
    Prompt  string
}

type Result struct {
    AgentID  string
    AgentURL string
    Status   string // finished | error | running | ...
    PRURL    string
    RunID    string
    Branch   string
    Message  string
}

type Worker interface {
    Start(ctx context.Context, req StartReq) (Result, error)
    FollowUp(ctx context.Context, req FollowUpReq) (Result, error)
    Ping(ctx context.Context) error
}
```

**v1:** `internal/worker/cursor`

- **Start:** create a **cloud** agent with `repository` (HTTPS GitHub URL). Never omit cloud config if using the official SDK — the SDK default is local. Agent name: `studio-{issue}`.
- **FollowUp:** resume by id and send. If REST cannot resume/send, use the small TS/Python helper for create/resume/send/wait only.
- **Transport/auth failure:** Go `error`. Run started and failed → `Result{Status: "error"}`. Different issue comments. Do not tight-loop retry auth errors.
- **Studio-only:** no MCP.
- **v1:** never put `CURSOR_API_KEY` in prompts or comments.

Stub workers: compile; `Ping` returns "not implemented". Select with `--worker` / `STUDIO_WORKER`.

Prompts live in `internal/prompt`, not in the Cursor client.

### Structured agent output

Phases 5–6 ask agents for machine-readable blocks (`verdict: …`). That will flake.

- Put parsers in `internal/prompt` (or `internal/parse`): accept the documented shape; ignore surrounding prose when the block is present.
- **One** automatic FollowUp on parse failure (“reply with exactly the schema, nothing else”).
- Second failure → comment the raw output (truncated), set `needs-human` (and `spec_status` / `review_status` = error as appropriate), exit `3`. Do not invent a verdict.
- Unit-test fixtures for: clean block, block with preamble, missing verdict, contradictory fields.

## 9. Prompts

### Implement (`dispatch`)

```
You are implementing a GitHub issue from the studio inbox.

Issue: studio#{{N}} — {{title}}
{{body}}

Target repository: {{repoURL}}
Use branch: {{branch}} (create if needed).
Open a **draft** pull request when the change is testable. Do not merge.

Constraints:
- Keep the diff small. Do not drive-by refactor.
- Discover and run this repo's tests (README, Makefile, go test, package scripts).
- Do not edit .github/workflows or CI unless the failure is install/setup caused by this change.
- Do not add secrets or change auth unless the issue requires it.
- PR body must include Studio: {{studioIssueURL}} and a short summary.
```

### CI fix (`watch`)

```
[studio] CI failed on {{prURL}} (HEAD {{sha}})

Failed checks:
{{checkList}}

Prefetched GitHub Actions logs (authoritative). Do not guess from status text alone.

{{logExcerpts}}

Fix the product/test code. Push to the same branch. Do not rewrite CI YAML unless these logs show setup/install you broke.
```

If log prefetch failed, do not send the CI-fix prompt.

### Review (`review`)

```
[studio] Review comments on {{prURL}}:

{{comments}}

Address valid items. Reply on threads if you can. Skip praise. If a comment is wrong, say so on the PR and do not change the code.
```

### Intake QC (`intake`) — Phase 5 only

```
You are reviewing a GitHub issue before any implementation starts.

Issue: studio#{{N}} — {{title}}
{{body}}

Target repository: {{repoURL}} (catalog only; do not clone or edit it).

Decide if this is complete enough to implement. Look for missing acceptance criteria, ambiguous scope, unspecified behavior, contradictions, and obvious holes. Do not implement. Do not invent product requirements; ask or suggest.

Reply with exactly:
verdict: ok | needs-work
summary: one short paragraph
questions:
- (concrete edits the human could make to the issue body)

If the issue is thin but the intent is obvious, verdict ok and note residual risk in summary. Prefer shipping a small clear issue over blocking on polish.
```

### PR review (`pr-review`) — Phase 6 only

```
You are reviewing a pull request. You do not implement. You do not push. You do not merge.

Issue: studio#{{N}} — {{title}}
{{body}}

PR: {{prURL}} (HEAD {{sha}})

Diff (authoritative, from GitHub). Do not guess files that are not listed.
{{diff}}

Review against the issue. Look for missing acceptance criteria, wrong behavior, security/auth mistakes, tests that do not cover the change, and scope creep. Skip nitpicks and praise. Do not demand drive-by refactors. CI already ran; do not restate failing checks unless the diff clearly will fail again.

Reply with exactly:
verdict: lgtm | changes-requested
summary: one short paragraph
comments:
- path: relative/file.go
  line: 12
  body: what is wrong and what would fix it

If the change matches the issue and residual risk is small, verdict lgtm. Prefer a small mergeable PR over a perfect one.
```

## 10. Workflows

### `dispatch.yml`

On: `issues` opened or labeled; `workflow_dispatch` with `issue_number`.

Skip unlabeled issues, closed issues, issues with no `repo:` / `new:` / frontmatter.

```yaml
concurrency:
  group: studio-${{ github.event.issue.number || inputs.issue_number }}
  cancel-in-progress: false
```

Job: `go run ./cmd/studio dispatch --issue $N` (or a built binary). Permissions: `issues: write`, `contents: read`, plus the PAT for other repos.

Phase 5: same workflow, same concurrency group. Intake runs first when QC is on and the issue is not `skip-qc` / `spec-ok` / `spec_status` in `{ok, approved, skipped}`. `needs-spec` issues do not Start. Relabel `spec-ok` (or `skip-qc`) re-triggers dispatch.

### `watch.yml`

On:

1. `workflow_dispatch` with `issue_number` (optional).
2. `repository_dispatch` types `studio-ci-failed` | `studio-pr-merged` (payload includes studio issue number and target PR URL / SHA).
3. `schedule` every 15 minutes listing open `pr-open` issues — **backstop only**, not the primary CI path once hooks are installed.

**Latency rule:** poll-only is allowed to prove Phase 2. After the first manual red→green loop works, install the target hook (below) before declaring Phase 2 done for daily use. A system that notices CI failure 15 minutes late trains distrust.

### Target hook — `docs/target-hook.md`

Optional snippet for allowlisted targets (copy into the target’s workflows; Studio does **not** auto-edit target CI):

- On `workflow_run` completed + failure (or `check_suite` failure), if the head commit’s PR body contains `Studio: <host>#N`, `repository_dispatch` the studio repo with type `studio-ci-failed` and that issue number.
- On `pull_request` closed + merged with the same footer, dispatch `studio-pr-merged`.

Studio verifies the footer and allowlist before acting. Unknown or missing footer → ignore.

### Review

v1: fold into `watch` — pull new review comments after `review_cursor`. Dedicated `review.yml` is optional.

Phase 6: same `watch` tick, after CI is green (or the PR has no checks), `studio pr-review --issue N`. If in the same run, `studio review` so the implementer sees those comments without waiting another poll. Do not pr-review when CI is red — that is the implementer CI-fix path.

### Close on merge

Same `watch` path (dispatch or poll): if the bound PR is merged, close the issue, label `done`, remove `working`. Prefer `studio-pr-merged` dispatch over waiting for the schedule.

## 11. CLI

| Command | Behavior |
| --- | --- |
| `studio dispatch --issue N` | Resolve repo, start worker, upsert binding, comment. Phase 5: intake first unless skipped/approved |
| `studio intake --issue N` | QC agent only: upsert `SPEC-*`, intake comment; no implementer Start |
| `studio watch --issue N` | CI fix / pull review comments / or close. Phase 6: may `pr-review` then `review` |
| `studio review --issue N` | New review comments → implementer FollowUp only (never the reviewer `agent_id`) |
| `studio pr-review --issue N` | Reviewer agent only: post a GitHub review; upsert `review_*`; no implementer Start |
| `studio doctor` | Ping GitHub + worker; print status; warn if watch is poll-only with no target hook docs applied |
| `studio bind --issue N --print` | Show parsed binding (exit `3` if corrupt) |

Flags: `--worker`, `--dry-run` (print prompt and target, no create).

Exit: `0` ok / nothing to do; `1` config or auth; `2` worker run failed after start; `3` needs-human (including binding/parse failures).

## 12. Layout

```
studio/
  PRD.md
  binding.md          # optional human notes — not machine state
  repos.yaml
  docs/target-hook.md
  cmd/studio/main.go
  internal/worker/
  internal/catalog/
  internal/github/
  internal/binding/
  internal/prompt/
  internal/parse/     # structured verdict parsers + fixtures
  internal/run/
  .github/workflows/dispatch.yml
  .github/workflows/watch.yml
  testdata/
```

`go test ./...` — httptest for GitHub and Cursor. No live network in default tests.

## 13. Cursor

Verify paths against current Cursor docs (`cursor.com/docs`) before freezing them. Official SDKs: TypeScript `@cursor/sdk`, Python `cursor-sdk`. No official Go SDK.

Likely HTTP base: `https://api.cursor.com` (Bearer). Expect at least create/list/get agent, maybe repositories, archive, cancel.

- Cloud create must include cloud + `repository` / repo id.
- Store `agent_id` (often `bc-…`) for resume across jobs.
- `autoCreatePR`: fine if the API supports it; still confirm the PR with GitHub before `pr-open`. No PR after a finished run → `needs-human`; do not invent one.
- Target repo must be connected to the user's Cursor GitHub account. If a repositories list API exists, fail closed when the slug is missing.

**Phase 1 discipline:** do not expand catalog, labels, or watch logic while Start/resume is still stubbed or flaky. Shipping orchestration around a non-working worker is false progress. Prefer: spike create → resume → confirm PR via GitHub → *then* harden the CLI.

## 14. Security

- Allowlist only.
- Least-privilege token **within** the personal-repos threat model: this repo's issues + listed targets. Document every target the PAT can touch in README.
- No tokens in prompts, comments, or worker env.
- Studio workflow: no `contents: write` on this repo unless a later phase opens catalog PRs.
- Org / multi-user / shared runners: out of scope until a GitHub App design replaces the PAT. Do not stretch v1 security language to cover that.

## 15. Build order

Commit after each phase (Conventional Commits: `feat:`, `fix:`, …). No `Co-authored-by: Cursor`. Do not start the next phase until the exit line passes.

### Phase 0 — skeleton

`go mod init`, `repos.yaml` (one real repo or a documented placeholder), `studio doctor`, README (how to file an issue, required secrets, **PAT blast radius**). Binding package: upsert + parse + corrupt→`needs-human` tests.

**Exit:** `go test ./...` green; `studio doctor --dry-run` prints the catalog; binding round-trip tests pass.

### Phase 1 — dispatch (API spike first)

1. Spike Cursor Cloud create / resume / send (Go HTTP or the tiny helper) against one allowlisted repo until it is boring.
2. Then: catalog, binding, Worker, `dispatch.yml`, `--dry-run` without an API key.

**Exit:** unit tests for unknown repos (no worker call) and `repo:` / `new:` resolution. Manual: one labeled issue → cloud agent → draft PR; binding has `agent_id` and `pr_url`.

If REST cannot create a cloud agent from Actions, add the TS/Python helper in this phase. Do not start Phase 2 without a working Start. Do not count “CLI wired, Start TODO” as Phase 1 complete.

### Phase 2 — watch

Check Runs + Actions logs. No logs → `needs-human`. Resume; on failure, replace agent on the existing branch.

**Exit:**

- Fixture with empty logs → no FollowUp; fixture with excerpt → prompt contains it.
- Manual: a red CI run gets a second turn.
- Before “daily use”: `docs/target-hook.md` applied on the pilot target **or** an explicit README note that poll-only lag is accepted for that repo.

### Phase 3 — review + close

`review_cursor`; close studio issue on merge; PR body has `Studio: #N`. Prefer merge close via `studio-pr-merged` dispatch when the hook exists.

**Exit:** mock merged PR closes the issue in a test; one real merge.

### Phase 4 — `new:` (optional)

Create from template and dispatch in one run.

**Must also:**

1. Ensure the GitHub App can access the new repo (prefer account install = **All repositories** so this is automatic; otherwise add the repo to the App installation).
2. Whitelist it: append to `repos.yaml` and create label `repo:<name>` on the studio inbox.
3. Only then `Worker.Start`.

App access without a catalog entry is not enough — `repos.yaml` remains the allowlist.

**Exit:** one `new:go-service` issue produces a new repo, catalog+label updated, and a PR.

### Phase 5 — Intake QC (optional)

A **separate** cloud agent reviews the issue before `Worker.Start`. Same Worker interface, different prompt and `agent_id`. Default model: `STUDIO_QC_MODEL` (fall back to `STUDIO_CURSOR_MODEL`). Never put QC and implementation on one agent.

Do not start this until Phase 3 has two weeks of real use (§16). Vague issues that still yield PRs are a prompt/catalog problem, not a reason to add this.

**Behavior:**

1. `dispatch` (QC on): if `skip-qc` or `spec_status` in `{ok, approved, skipped}` → existing Start path.
2. Else `studio intake` = `Worker.Start` with the intake prompt only. No target branch, no PR, no `working`.
3. Parse the agent's `verdict` via `internal/parse` (one retry on failure). Upsert `spec_agent_id` and `spec_status`. Post a normal comment with summary / questions / suggestions.
4. If `ok` / approved, then Start the implementer in the same run (or the next dispatch).
5. If `needs-work`: label `needs-spec`, set `spec_status=needs-work`. Suggestions are advice — Studio never silently rewrites the issue. Human applies edits, then labels `spec-ok` (or sets approved) / `skip-qc`.
6. Second parse failure → `needs-human`; do not Start.

### Phase 6 — PR reviewer (optional)

A separate cloud agent reviews draft PRs after CI is green. Default model: `STUDIO_REVIEW_MODEL` (fall back to `STUDIO_QC_MODEL`, then `STUDIO_CURSOR_MODEL`).

Never put review and implementation on one agent. Never FollowUp the reviewer with PR comments; that path is implementer-only.

Triggered by SHA, not by comments: each distinct PR HEAD is reviewed at most once (stops a reviewer ↔ implementer infinite loop). Max ~2 rounds.

**Behavior:**

1. Skip if `skip-review`, or CI pending/failed.
2. Prefetch PR diff via GitHub; `studio pr-review --issue N`.
3. Parse verdict (one retry). `lgtm` → submit GitHub review as `COMMENT` (not `APPROVE`). Does not merge; human merges.
4. `changes-requested` → submit as `COMMENT` / request-changes as designed; implementer then receives those threads via `review`.
5. Second unparseable verdict → `review_status=error`, `needs-human`.
6. Reviewer does not merge, dismiss human reviews, or rewrite the issue.

### Later (do not build now)

Pi owner, Claude/Codex workers, markdown importer, dashboard, GitHub App for org multi-user.

## 16. After Phase 3

For two weeks of real use:

| Signal | Target |
| --- | --- |
| Labeled issues that got a PR | ≥ 70% |
| CI-fix prompts sent without logs | 0 |
| Two concurrent runs on one issue | 0 |
| Issues left open after merge | 0 |
| Binding parse / corrupt stops (should be rare) | investigated, not ignored |
| Median time from CI red → FollowUp sent | &lt; 5 min on hooked targets (poll-only exempt if documented) |

If PRs are rare, fix prompts and catalog — do not add more agents. Intake QC (Phase 5) is for wasted runs on holes in the issue. The PR reviewer (Phase 6) is for holes in the diff after CI is green. Neither is a fix for a low PR rate.

## 17. Implementing

1. New GitHub repo. Put this `PRD.md` at the root. Keep it current.
2. Phase 0 then **Cursor spike** then 1. Do not invent API keys; the operator adds Actions secrets.
3. Stdlib is enough (`net/http`, `encoding/json`, `flag`). Cobra only if you already want it.
4. Claude/Codex: stubs only until asked.
5. If this spec is ambiguous, take the smaller option and note it in §18 / §19.

## 18. Locked decisions

1. Inbox = Issues on this repo.
2. Owner v1 = Actions concurrency, not a Pi.
3. Coding v1 = Cursor Cloud, not the Pi and not a runner checkout (unless Cursor create is blocked).
4. No Go Cursor SDK: HTTP or a tiny helper. Phase 1 is an API reliability spike first.
5. Target CI = those repos' existing Actions.
6. Closing the inbox issue is Studio's job.
7. Worker interface now; a second implementation later.
8. Intake QC is a separate agent and a separate `agent_id`. It does not implement, open PRs, or share follow-ups with the coder. It does not rewrite the issue; it posts a structured assessment. Phases 0–3 ship without it.
9. PR review is a separate agent and a separate `agent_id`. It is SHA-triggered, max 2 rounds, posts via the GitHub review API from parsed output. It never merges. Comment follow-ups always go to the implementer. Phases 0–3 ship without it.
10. Binding HTML comment: single writer (`cmd/studio`); parse/corrupt → `needs-human`, no best-effort merge.
11. Watch: poll is a backstop; after first red→green, target `repository_dispatch` hook is the preferred CI/merge path.
12. Structured agent output: small parser + one retry + `needs-human`; never invent verdicts.
13. Threat model v1 = personal allowlisted repos + PAT/App blast radius documented; org App is a later redesign.
14. GitHub App install prefers **All repositories** on the personal account so Phase 4 `new:` repos get App access automatically; `repos.yaml` remains the only dispatch allowlist. Phase 4 must update the catalog (and label) before Start.

## 19. Decisions made during implementation

- **2026-08-27:** Intake QC is in scope as Phase 5 (optional). Separate specialist agent; human accepts suggested issue edits; `skip-qc` / `spec-ok` escape hatches. Not a planner–coder chain in one run of one agent.
- **2026-08-27:** PR reviewer is in scope as Phase 6 (optional). Separate specialist agent; GitHub review on the target PR; SHA + max-2-rounds loop brake; `skip-review` escape hatch. Does not merge. Implementer still owns code follow-ups.
- **2026-08-27:** GitHub App `adamfriedl-studio`; prefer All-repositories install so Phase 4 new repos inherit App access; catalog whitelist still required before dispatch.
