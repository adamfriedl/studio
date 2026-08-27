## Verify

```bash
go test ./...
go run ./cmd/studio doctor --dry-run
STUDIO_DRY_LABELS=repo:pad-lab go run ./cmd/studio dispatch --issue 1 --dry-run
```

No live network in default tests. Do not commit secrets or `.env`.

## Go style

Follow the personal `go` skill (`~/.cursor/skills/go`, from [spf13/go-skills](https://github.com/spf13/go-skills/blob/main/go/SKILL.md)) and the `go-standards` Cursor rule. Keep Studio’s `internal/` layout from the PRD for now; write idiomatic Go inside it.