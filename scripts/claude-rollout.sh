#!/usr/bin/env bash
# Sets CLAUDE_CODE_OAUTH_TOKEN and lets Actions open PRs on repos using studio's claude-agent workflow.
# Usage: scripts/claude-rollout.sh [repo ...]   (default: all enabled repos)
set -euo pipefail

OWNER=adamfriedl
if [ $# -gt 0 ]; then REPOS=("$@"); else REPOS=(pad-lab job-search homelab intake-desk adamfriedl.github.io); fi

read -rsp "Paste CLAUDE_CODE_OAUTH_TOKEN (from 'claude setup-token'; input hidden): " token
echo
[ -n "$token" ] || { echo "empty token"; exit 1; }
echo "token length: ${#token}"
if ! CLAUDE_CODE_OAUTH_TOKEN="$token" claude -p "reply with exactly: ok" 2>&1 | grep -qx "ok"; then
  echo "token failed a live check with claude -p; nothing was changed"; exit 1
fi

for r in "${REPOS[@]}"; do
  printf '%s' "$token" | gh secret set CLAUDE_CODE_OAUTH_TOKEN --repo "$OWNER/$r"
  gh api -X PUT "repos/$OWNER/$r/actions/permissions/workflow" \
    -f default_workflow_permissions=read -F can_approve_pull_request_reviews=true
  echo "$r: secret set, Actions may open PRs"
done
