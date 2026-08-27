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
| contents | write |
| issues | write |
| pull_requests | write |
| checks | read |
| actions | read |

## Runtime

Workflows mint a short-lived installation token via `actions/create-github-app-token` for owner `adamfriedl` (all repos the install can see).  
Falls back to `STUDIO_GITHUB_TOKEN` (PAT) if App secrets are missing.

## Phase 4 (`new:`) — required behavior

When Studio creates a repo from a template:

1. `gh repo create` / API create from template.
2. Ensure App can access it (automatic if install is **All repositories**; otherwise add repo to the installation).
3. Append the short name to `repos.yaml` (commit on `studio` or open a catalog PR — prefer direct commit on `main` for v1 solo).
4. Ensure label `repo:<name>` exists on studio.
5. Then `Worker.Start` on the new repo as usual.

Never dispatch to a repo that is not in `repos.yaml`, even if the App can see it.
