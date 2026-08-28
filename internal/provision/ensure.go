package provision

import (
	"context"
	"embed"
	"fmt"
	"os"
	"strings"
	"time"

	gh "github.com/adamfriedl/studio/internal/github"
)

//go:embed assets
var assets embed.FS

// FileClient upserts files on a target repo.
type FileClient interface {
	GetFile(ctx context.Context, owner, repo, path string) (*gh.FileContent, error)
	PutFile(ctx context.Context, owner, repo, path, message string, content []byte, sha string) error
}

// SecretClient writes Actions secrets on a target repo.
type SecretClient interface {
	PutActionsSecret(ctx context.Context, owner, repo, name, value string) error
}

// Client is both file and secret APIs (satisfied by *gh.Client).
type Client interface {
	FileClient
	SecretClient
}

// Result summarizes what Ensure wrote.
type Result struct {
	Files   []string
	Secrets []string
}

// PackFiles returns path→content for a known template key (ci + hook).
func PackFiles(templateKey string) (map[string][]byte, error) {
	key := strings.TrimSpace(templateKey)
	if key == "" {
		return nil, fmt.Errorf("empty template key")
	}
	files := map[string][]byte{}

	hook, err := assets.ReadFile("assets/common/studio-hook.yml")
	if err != nil {
		return nil, fmt.Errorf("read studio-hook: %w", err)
	}
	files[".github/workflows/studio-hook.yml"] = hook

	ciPath := "assets/" + key + "/ci.yml"
	ci, err := assets.ReadFile(ciPath)
	if err != nil {
		return nil, fmt.Errorf("unknown provision pack %q (no %s)", key, ciPath)
	}
	files[".github/workflows/ci.yml"] = ci
	return files, nil
}

// Ensure upserts pack workflows and copies STUDIO_HOOK_TOKEN onto owner/repo.
// The hook token comes from env STUDIO_HOOK_TOKEN (fail closed if missing).
// Do not copy the Studio App private key onto targets — see docs/target-hook.md.
func Ensure(ctx context.Context, client Client, owner, repo, templateKey string) (*Result, error) {
	files, err := PackFiles(templateKey)
	if err != nil {
		return nil, err
	}

	hookToken := strings.TrimSpace(os.Getenv("STUDIO_HOOK_TOKEN"))
	if hookToken == "" {
		return nil, fmt.Errorf("STUDIO_HOOK_TOKEN required to provision target secrets (narrow PAT for repository_dispatch on studio)")
	}

	msg := "chore(studio): provision CI and target hook"
	out := &Result{}
	for path, content := range files {
		if err := upsertFile(ctx, client, owner, repo, path, msg, content); err != nil {
			return out, fmt.Errorf("upsert %s: %w", path, err)
		}
		out.Files = append(out.Files, path)
	}

	if err := client.PutActionsSecret(ctx, owner, repo, "STUDIO_HOOK_TOKEN", hookToken); err != nil {
		return out, fmt.Errorf("put secret STUDIO_HOOK_TOKEN: %w", err)
	}
	out.Secrets = append(out.Secrets, "STUDIO_HOOK_TOKEN")
	return out, nil
}

func upsertFile(ctx context.Context, client FileClient, owner, repo, path, message string, content []byte) error {
	var last error
	for attempt := 0; attempt < 8; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}
		sha := ""
		if f, err := client.GetFile(ctx, owner, repo, path); err == nil {
			sha = f.SHA
			if string(f.Content) == string(content) {
				return nil
			}
		} else {
			last = err
			// 404 / not found → create; other errors may be "repo not ready" after generate.
			if !isNotFound(err) && attempt < 7 {
				continue
			}
			if !isNotFound(err) && attempt == 7 {
				// Still try Put without sha in case Get is flaky.
			}
		}
		if err := client.PutFile(ctx, owner, repo, path, message, content, sha); err != nil {
			last = err
			continue
		}
		return nil
	}
	if last != nil {
		return last
	}
	return fmt.Errorf("upsert %s: retries exhausted", path)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "404") || strings.Contains(s, "Not Found")
}
