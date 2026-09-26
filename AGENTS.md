# AGENTS.md

This repo holds shared GitHub Actions automation for my personal repos — see `README.md` for how it works. There is no application code.

## Changing `claude-agent.yml`

- Every enabled repo calls it at `@main`, so a push here is a deploy. Keep diffs small.
- Verify: `actionlint .github/workflows/claude-agent.yml`, then trigger it on `pad-lab` with a small `@claude` issue and check both jobs.
- Callers grant permissions; the reusable workflow can't exceed them. If you add a permission here, update the caller snippet in `README.md` and every enabled repo's `claude.yml`.
- The review prompt must never contain the literal `@claude` mention, or a review comment could trigger another run.
- Pass event data to `run:` steps through `env:`, never by interpolating `${{ }}` into shell.

## Secrets

Never commit tokens. `scripts/claude-rollout.sh` reads the Claude token from a hidden prompt; run it in a normal terminal, not through an agent session that would log its output.
