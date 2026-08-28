# Simplify

Review the current uncommitted (or just-authored) code change and simplify it before commit.

Goals:
- Prefer the smallest clear diff that still meets the request
- Remove dead code, unused imports, speculative abstractions, and redundant comments
- Collapse duplication only when it improves clarity (not for its own sake)
- Do not change behavior, public APIs, or tests unless needed to keep them green
- Do not drive-by refactor unrelated files

Process:
1. Inspect the diff (staged + unstaged) for this change
2. Apply focused edits that simplify without expanding scope
3. Re-run the repo verify path if you touched executable code
4. Summarize what you simplified (or say “already tight” if nothing to cut)
