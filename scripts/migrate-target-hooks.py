#!/usr/bin/env python3
"""Migrate allowlisted targets to STUDIO_HOOK_TOKEN + github.token local reads.

Reads the hook token from ~/Documents/tokens/studio-hook-token.md (or STUDIO_HOOK_TOKEN env).
Keeps App secrets on adamfriedl/studio (dispatch/watch). Removes them from other targets.

Usage:
  python3 scripts/migrate-target-hooks.py
  python3 scripts/migrate-target-hooks.py --dry-run
"""
from __future__ import annotations

import argparse
import base64
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

OWNER = "adamfriedl"
ROOT = Path(__file__).resolve().parents[1]
PACK_HOOK = ROOT / "internal/provision/assets/common/studio-hook.yml"

# repo → optional workflow_run CI name (exact `name:`), or None for PR-events only
TARGETS: dict[str, str | None] = {
    "pad-lab": "Deploy dashboard to GitHub Pages",
    "studio": None,
    "job-search": None,
    "adamfriedl.github.io": None,
    "homelab": "Plan and apply",
    "intake-desk": "verify",
}


def gh_token() -> str:
    out = subprocess.check_output(["gh", "auth", "token"], text=True).strip()
    if not out:
        raise SystemExit("gh auth token empty")
    return out


def hook_token() -> str:
    env = os.environ.get("STUDIO_HOOK_TOKEN", "").strip()
    if env:
        return env
    path = Path.home() / "Documents/tokens/studio-hook-token.md"
    text = path.read_text()
    m = re.search(r"(github_pat_[A-Za-z0-9_]+)", text)
    if not m:
        raise SystemExit(f"no github_pat_ in {path}")
    return m.group(1)


def api(method: str, path: str, token: str, data: dict | None = None) -> tuple[int, object]:
    url = "https://api.github.com" + path
    body = None if data is None else json.dumps(data).encode()
    req = urllib.request.Request(
        url,
        data=body,
        method=method,
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
            "User-Agent": "studio-migrate-hooks",
            **({"Content-Type": "application/json"} if body else {}),
        },
    )
    try:
        with urllib.request.urlopen(req) as r:
            raw = r.read()
            return r.status, json.loads(raw) if raw else None
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()


def put_secret(repo: str, name: str, value: str, dry: bool) -> None:
    if dry:
        print(f"  [dry-run] set secret {name}")
        return
    subprocess.run(
        ["gh", "secret", "set", name, "--repo", f"{OWNER}/{repo}"],
        input=value.encode(),
        check=True,
    )
    print(f"  set secret {name}")


def delete_secret(repo: str, name: str, token: str, dry: bool) -> None:
    if dry:
        print(f"  [dry-run] delete secret {name}")
        return
    code, _ = api("DELETE", f"/repos/{OWNER}/{repo}/actions/secrets/{name}", token)
    if code in (204, 404):
        print(f"  deleted secret {name} ({code})")
    else:
        print(f"  warn delete {name}: {code}")


def put_hook(repo: str, content: str, token: str, dry: bool) -> None:
    path = ".github/workflows/studio-hook.yml"
    code, existing = api("GET", f"/repos/{OWNER}/{repo}/contents/{path}", token)
    sha = existing.get("sha") if isinstance(existing, dict) else None
    if dry:
        print(f"  [dry-run] put {path} ({len(content)} bytes) sha={sha}")
        return
    payload = {
        "message": "chore(studio): migrate hook to STUDIO_HOOK_TOKEN + github.token",
        "content": base64.b64encode(content.encode()).decode(),
    }
    if sha:
        payload["sha"] = sha
    code, out = api("PUT", f"/repos/{OWNER}/{repo}/contents/{path}", token, payload)
    if code not in (200, 201):
        raise SystemExit(f"put hook {repo}: {code} {out}")
    print(f"  put {path}")


def render_hook(ci_name: str | None) -> str:
    text = PACK_HOOK.read_text()
    # Strip pack's default workflow_run: ["ci"] block (may be absent lines after).
    text = re.sub(
        r"\n  workflow_run:\n    workflows: \[\"ci\"\]\n    types: \[completed\]\n",
        "\n",
        text,
        count=1,
    )
    if ci_name:
        insert = (
            "  workflow_run:\n"
            f'    workflows: ["{ci_name}"]\n'
            "    types: [completed]\n"
        )
        # After pull_request closed block, before blank line + concurrency
        text = text.replace(
            "    types: [opened, synchronize, reopened, closed]\n\nconcurrency:",
            f"    types: [opened, synchronize, reopened, closed]\n{insert}\nconcurrency:",
            1,
        )
    return text


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()
    token = gh_token()
    ht = hook_token()
    print(f"migrating {len(TARGETS)} targets (dry_run={args.dry_run})")
    for repo, ci in TARGETS.items():
        print(f"== {repo} ==")
        put_secret(repo, "STUDIO_HOOK_TOKEN", ht, args.dry_run)
        put_hook(repo, render_hook(ci), token, args.dry_run)
        if repo == "studio":
            print("  keep STUDIO_APP_* on studio (dispatch/watch)")
            continue
        delete_secret(repo, "STUDIO_APP_ID", token, args.dry_run)
        delete_secret(repo, "STUDIO_APP_PRIVATE_KEY", token, args.dry_run)
    print("done")
    return 0


if __name__ == "__main__":
    sys.exit(main())
