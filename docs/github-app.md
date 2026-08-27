# GitHub App (studio-bot)

Studio prefers a GitHub App over a personal PAT for GitHub API calls.

## One-time register

```bash
cd studio
python3 scripts/register-github-app.py
```

1. Browser opens → **Create GitHub App**
2. Script stores credentials in `~/Documents/tokens/studio-github-app.json` (not in git)
3. Sets Actions secrets `STUDIO_APP_ID` + `STUDIO_APP_PRIVATE_KEY` on `adamfriedl/studio`
4. Opens the **install** page — select only allowlisted repos:
   - `studio`, `pad-lab`, `job-search`, `homelab`, `adamfriedl.github.io`, `intake-desk`

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

Workflows mint a short-lived installation token via `actions/create-github-app-token`.  
Falls back to `STUDIO_GITHUB_TOKEN` (PAT) if App secrets are missing.
