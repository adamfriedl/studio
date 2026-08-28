# GitHub App (`adamfriedl-studio`)

Studio prefers a GitHub App over a personal PAT for GitHub API calls.

## One-time register

```bash
cd studio
python3 scripts/register-github-app.py
```

1. Browser opens → **Create GitHub App** (name: `adamfriedl-studio` — App names cannot match existing GitHub users/orgs)
2. Script stores credentials in `~/Documents/tokens/studio-github-app.json` (not in git)
3. Sets Actions secrets `STUDIO_APP_ID` + `STUDIO_APP_PRIVATE_KEY` on `adamfriedl/studio`
4. Install on your account

## Install mode (important)

Prefer **All repositories** for `adamfriedl`.

- New repos Studio creates (`new:` / Phase 4) then get App access automatically.
- **Dispatch allowlist is still `repos.yaml`** — App access ≠ Studio will touch it; catalog is the gate.
- “Only select repositories” forces a Phase 4 step to add each new repo to the installation (easy to forget).

Configure: https://github.com/settings/installations/157096911 → Repository access → **All repositories**.

## Permissions (manifest)

| Permission | Access |
| --- | --- |
| metadata | read |
| **administration** | **write** (required to `POST …/generate` for Phase 4 `new:`) |
| contents | write |
| issues | write |
| pull_requests | write |
| checks | read |
| actions | read |
| **secrets** | **write** (copy `STUDIO_HOOK_TOKEN` onto `new:` targets for the hook) |

If generate returns `403 Resource not accessible by integration`, open  
https://github.com/settings/apps/adamfriedl-studio → **Permissions & events** → Repository → **Administration: Read and write** → Save → accept the permission request on the installation.

If secret put returns `403`, grant **Secrets: Read and write** the same way (or keep using `STUDIO_GITHUB_PAT` for provision until the install accepts it).

Until administration is approved, dispatch passes the classic PAT as `STUDIO_GITHUB_PAT` and Studio uses it for template generate / provision when needed.

## Runtime

Workflows mint a short-lived installation token via `actions/create-github-app-token` for owner `adamfriedl` (all repos the install can see).  
Falls back to `STUDIO_GITHUB_TOKEN` (PAT) if App secrets are missing.  
`STUDIO_GITHUB_PAT` (same PAT secret) is used for `new:` repo create / provision when the App lacks administration or secrets:write.

## Phase 4 (`new:`) — required behavior

When Studio creates a repo from a template:

1. Create from template (`POST …/generate`).
2. Ensure App can access it (automatic if install is **All repositories**; otherwise add repo to the installation).
3. Append the short name to `repos.yaml` (commit on `studio`).
4. Ensure label `repo:<name>` exists on studio.
5. **Provision pack** (fail closed if this fails — do not Start poll-only):
   - Upsert stack CI + `studio-hook.yml` from `internal/provision` (keyed by template, e.g. `go-service`).
   - Copy `STUDIO_HOOK_TOKEN` from the dispatch job env onto the new repo as an Actions secret (narrow PAT for `repository_dispatch` on studio — **not** the App private key).
6. Then `Worker.Start` on the new repo as usual.

Never dispatch to a repo that is not in `repos.yaml`, even if the App can see it.

If `new:` picks a name that already exists and is **not** allowlisted, Studio fails closed (`needs-human`) — it will not hijack an unrelated repo. Idempotent retry is allowed only when the name is already in the catalog (re-runs provision so a half-failed create can heal).
