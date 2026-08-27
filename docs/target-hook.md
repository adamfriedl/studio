# Target hook (optional)

After Studio’s first real red→green CI loop on a pilot target, prefer
`repository_dispatch` into the studio repo over 15-minute polling.

Studio does **not** auto-edit target workflows. Copy a snippet into the
target only when you opt in.

## Dispatch types (studio repo)

| Type | When |
| --- | --- |
| `studio-ci-failed` | Target CI failed for a PR whose body contains `Studio: adamfriedl/studio#N` |
| `studio-pr-merged` | That PR merged |

Payload should include at least: `issue_number`, `pr_url`, `sha` (for CI failure).

Studio verifies the `Studio:` footer and the allowlist before acting.

## Pilot note

`pad-lab` may stay poll-only until the hook is installed; document that lag in the README if so.
