#!/usr/bin/env python3
"""Register Studio GitHub App via manifest flow, store secrets, print install URL.

Usage: python3 scripts/register-github-app.py
Opens a local page that POSTs the manifest to GitHub. After you click
Create GitHub App, GitHub redirects here; we exchange the code and set
Actions secrets on adamfriedl/studio.
"""
from __future__ import annotations

import json
import os
import subprocess
import threading
import time
import urllib.parse
import urllib.request
import webbrowser
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

PORT = 8765
REDIRECT = f"http://127.0.0.1:{PORT}/callback"
REPO = "adamfriedl/studio"
OUT_DIR = Path.home() / "Documents" / "tokens"
OUT_DIR.mkdir(parents=True, exist_ok=True)
CREDS_PATH = OUT_DIR / "studio-github-app.json"

MANIFEST = {
    "name": "studio-bot",
    "url": "https://github.com/adamfriedl/studio",
    "description": "Studio: issue inbox → Cursor Cloud agents on allowlisted repos",
    "public": False,
    "redirect_url": REDIRECT,
    "callback_urls": [REDIRECT],
    "request_oauth_on_install": False,
    "setup_url": "https://github.com/adamfriedl/studio",
    "hook_attributes": {
        # Inactive — Studio polls / repository_dispatch; no inbound webhook yet.
        "url": "https://example.com/studio-webhook-unused",
        "active": False,
    },
    "default_permissions": {
        "metadata": "read",
        "contents": "write",
        "issues": "write",
        "pull_requests": "write",
        "checks": "read",
        "actions": "read",
    },
    "default_events": [],
}

done = threading.Event()
result: dict = {}


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("http:", fmt % args)

    def do_GET(self):
        if self.path.startswith("/callback"):
            qs = urllib.parse.urlparse(self.path).query
            code = urllib.parse.parse_qs(qs).get("code", [None])[0]
            if not code:
                self.send_response(400)
                self.end_headers()
                self.wfile.write(b"missing code")
                return
            try:
                app = exchange(code)
                result["app"] = app
                save_and_secrets(app)
                html = (
                    "<h1>studio-bot registered</h1>"
                    "<p>Credentials saved. You can close this tab and return to the terminal.</p>"
                    f"<p>Next: install on allowlisted repos — "
                    f"<a href='{install_url(app)}'>{install_url(app)}</a></p>"
                )
                self.send_response(200)
                self.send_header("Content-Type", "text/html")
                self.end_headers()
                self.wfile.write(html.encode())
            except Exception as e:
                self.send_response(500)
                self.end_headers()
                self.wfile.write(str(e).encode())
                result["error"] = str(e)
            finally:
                done.set()
            return

        if self.path in ("/", "/start"):
            body = f"""<!DOCTYPE html>
<html><body>
<h1>Register studio-bot</h1>
<p>Click the button (you must be logged into GitHub as adamfriedl).</p>
<form action="https://github.com/settings/apps/new" method="post">
  <input type="hidden" name="manifest" id="manifest">
  <button type="submit">Create GitHub App</button>
</form>
<script>
document.getElementById("manifest").value = {json.dumps(json.dumps(MANIFEST))};
</script>
</body></html>"""
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.end_headers()
            self.wfile.write(body.encode())
            return

        self.send_response(404)
        self.end_headers()


def exchange(code: str) -> dict:
    req = urllib.request.Request(
        f"https://api.github.com/app-manifests/{code}/conversions",
        method="POST",
        headers={
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
            "User-Agent": "studio-register",
        },
        data=b"",
    )
    with urllib.request.urlopen(req, timeout=60) as res:
        return json.load(res)


def install_url(app: dict) -> str:
    slug = app.get("slug") or "studio-bot"
    return f"https://github.com/apps/{slug}/installations/new"


def save_and_secrets(app: dict) -> None:
    # Persist locally (gitignored Documents path) — never commit.
    slim = {
        "id": app["id"],
        "slug": app.get("slug"),
        "client_id": app.get("client_id"),
        "html_url": app.get("html_url"),
        "pem": app["pem"],
        "webhook_secret": app.get("webhook_secret"),
        "install_url": install_url(app),
    }
    CREDS_PATH.write_text(json.dumps(slim, indent=2) + "\n")
    os.chmod(CREDS_PATH, 0o600)
    print(f"wrote {CREDS_PATH}")

    # Actions secrets on studio
    env = os.environ.copy()
    token_file = Path.home() / "Documents" / "tokens" / "studio.md"
    if token_file.exists() and not env.get("GH_TOKEN"):
        for line in token_file.read_text().splitlines():
            if line.startswith("ghp_"):
                env["GH_TOKEN"] = line.strip()
                break

    subprocess.run(
        ["gh", "secret", "set", "STUDIO_APP_ID", "--repo", REPO, "--body", str(app["id"])],
        check=True,
        env=env,
    )
    subprocess.run(
        ["gh", "secret", "set", "STUDIO_APP_PRIVATE_KEY", "--repo", REPO],
        input=app["pem"].encode(),
        check=True,
        env=env,
    )
    print("set Actions secrets STUDIO_APP_ID + STUDIO_APP_PRIVATE_KEY")


def main() -> None:
    server = HTTPServer(("127.0.0.1", PORT), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    url = f"http://127.0.0.1:{PORT}/start"
    print(f"Open: {url}")
    webbrowser.open(url)
    print("Waiting for GitHub redirect (up to 10 minutes)…")
    if not done.wait(timeout=600):
        raise SystemExit("timed out waiting for app registration")
    if "error" in result:
        raise SystemExit(result["error"])
    app = result["app"]
    print("App id:", app["id"], "slug:", app.get("slug"))
    print("Install on allowlisted repos:")
    print(" ", install_url(app))
    print("Select: studio, pad-lab, job-search, homelab, adamfriedl.github.io, intake-desk")
    webbrowser.open(install_url(app))
    server.shutdown()


if __name__ == "__main__":
    main()
