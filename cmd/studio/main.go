package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adamfriedl/studio/internal/binding"
	"github.com/adamfriedl/studio/internal/catalog"
	gh "github.com/adamfriedl/studio/internal/github"
	"github.com/adamfriedl/studio/internal/prompt"
	"github.com/adamfriedl/studio/internal/worker"
	"github.com/adamfriedl/studio/internal/worker/cursor"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "doctor":
		os.Exit(cmdDoctor(os.Args[2:]))
	case "dispatch":
		os.Exit(cmdDispatch(os.Args[2:]))
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `studio — issue inbox → Cursor Cloud agents

Usage:
  studio doctor [--dry-run] [--catalog PATH]
  studio dispatch --issue N [--dry-run] [--catalog PATH] [--owner O] [--repo R]

Exit: 0 ok; 1 config/auth; 2 worker failed; 3 needs-human
`)
}

func findCatalog(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, path)
	}
	return path
}

func cmdDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "print catalog only; no network")
	catalogPath := fs.String("catalog", "repos.yaml", "path to repos.yaml")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}

	c, err := catalog.Load(findCatalog(*catalogPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog: %v\n", err)
		return 1
	}

	fmt.Printf("org: %s\n", c.Org)
	fmt.Printf("defaults.branchPrefix: %s\n", c.Defaults.BranchPrefix)
	fmt.Printf("defaults.draftPR: %v\n", c.Defaults.DraftPR)
	fmt.Println("repos:")
	for _, r := range c.Repos {
		full, _ := c.FullName(r.Name)
		fmt.Printf("  - %s → %s\n", r.Name, full)
	}

	if *dryRun {
		fmt.Println("doctor: dry-run ok (no network)")
		return 0
	}

	token := strings.TrimSpace(os.Getenv("STUDIO_GITHUB_TOKEN"))
	if token == "" {
		fmt.Fprintln(os.Stderr, "STUDIO_GITHUB_TOKEN unset")
		return 1
	}
	if strings.TrimSpace(os.Getenv("CURSOR_API_KEY")) == "" {
		fmt.Fprintln(os.Stderr, "CURSOR_API_KEY unset")
		return 1
	}
	w := cursor.Helper{}
	if err := w.Ping(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "cursor: %v\n", err)
		return 1
	}
	fmt.Println("doctor: github token present; cursor ping ok")
	return 0
}

func cmdDispatch(args []string) int {
	fs := flag.NewFlagSet("dispatch", flag.ContinueOnError)
	issueN := fs.Int("issue", 0, "studio issue number")
	dryRun := fs.Bool("dry-run", false, "print target + prompt; no worker/GitHub writes")
	catalogPath := fs.String("catalog", "repos.yaml", "path to repos.yaml")
	owner := fs.String("owner", "", "studio repo owner (default: GITHUB_REPOSITORY owner or catalog org)")
	repo := fs.String("repo", "", "studio repo name (default: GITHUB_REPOSITORY name or studio)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *issueN <= 0 {
		fmt.Fprintln(os.Stderr, "--issue required")
		return 1
	}

	c, err := catalog.Load(findCatalog(*catalogPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog: %v\n", err)
		return 1
	}

	studioOwner, studioRepo := *owner, *repo
	if studioOwner == "" || studioRepo == "" {
		if gr := os.Getenv("GITHUB_REPOSITORY"); gr != "" {
			parts := strings.SplitN(gr, "/", 2)
			if len(parts) == 2 {
				if studioOwner == "" {
					studioOwner = parts[0]
				}
				if studioRepo == "" {
					studioRepo = parts[1]
				}
			}
		}
	}
	if studioOwner == "" {
		studioOwner = c.Org
	}
	if studioRepo == "" {
		studioRepo = "studio"
	}

	token := strings.TrimSpace(os.Getenv("STUDIO_GITHUB_TOKEN"))
	if !*dryRun && token == "" {
		fmt.Fprintln(os.Stderr, "STUDIO_GITHUB_TOKEN required unless --dry-run")
		return 1
	}

	ctx := context.Background()
	var issue *gh.Issue
	var labelNames []string

	if *dryRun && token == "" {
		// Offline dry-run: require env STUDIO_DRY_LABELS=repo:pad-lab and title/body stubs.
		labelNames = strings.Split(os.Getenv("STUDIO_DRY_LABELS"), ",")
		issue = &gh.Issue{
			Number: *issueN,
			Title:  envOr("STUDIO_DRY_TITLE", "dry-run issue"),
			Body:   envOr("STUDIO_DRY_BODY", "dry-run body"),
		}
	} else {
		client := &gh.Client{Token: token}
		var err error
		issue, err = client.GetIssue(ctx, studioOwner, studioRepo, *issueN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "github: %v\n", err)
			return 1
		}
		for _, l := range issue.Labels {
			labelNames = append(labelNames, l.Name)
		}
		// Idempotent: already started → do not launch a second cloud agent.
		if existing, err := binding.Parse(issue.Body); err == nil && existing.AgentID != "" {
			fmt.Printf("dispatch: already bound agent_id=%s pr=%s — skipping\n", existing.AgentID, existing.PRURL)
			return 0
		}
	}

	kind, name, targetURL, err := c.TargetFromLabels(labelNames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "needs-human: %v\n", err)
		if !*dryRun && token != "" {
			client := &gh.Client{Token: token}
			_ = client.AddComment(ctx, studioOwner, studioRepo, *issueN, "Studio: "+err.Error())
			_ = client.AddLabels(ctx, studioOwner, studioRepo, *issueN, []string{"needs-human"})
		}
		return 3
	}
	if kind == "new" {
		fmt.Fprintln(os.Stderr, "needs-human: new: templates not implemented until Phase 4")
		return 3
	}

	branch := fmt.Sprintf("%s%d-%s", c.Defaults.BranchPrefix, *issueN, slug(issue.Title))
	studioIssueURL := fmt.Sprintf("https://github.com/%s/%s/issues/%d", studioOwner, studioRepo, *issueN)
	p := prompt.Implement(prompt.ImplementInput{
		IssueNumber:    *issueN,
		Title:          issue.Title,
		Body:           stripBinding(issue.Body),
		RepoURL:        targetURL,
		Branch:         branch,
		StudioIssueURL: studioIssueURL,
	})

	fmt.Printf("target: %s (%s)\n", targetURL, name)
	fmt.Printf("branch: %s\n", branch)
	if *dryRun {
		fmt.Println("--- prompt ---")
		fmt.Println(p)
		fmt.Println("dispatch: dry-run ok (no worker)")
		return 0
	}

	if strings.TrimSpace(os.Getenv("CURSOR_API_KEY")) == "" {
		fmt.Fprintln(os.Stderr, "CURSOR_API_KEY required")
		return 1
	}

	client := &gh.Client{Token: token}
	_ = client.AddLabels(ctx, studioOwner, studioRepo, *issueN, []string{"working"})

	w := cursor.Helper{Timeout: 45 * time.Minute}
	res, err := w.Start(ctx, worker.StartReq{
		IssueNumber:  *issueN,
		Title:        issue.Title,
		Body:         issue.Body,
		RepoURL:      targetURL,
		StartingRef:  "main",
		BranchName:   branch,
		Model:        envOr("STUDIO_CURSOR_MODEL", "composer-2.5"),
		AutoCreatePR: c.Defaults.DraftPR,
		Prompt:       p,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "worker: %v\n", err)
		msg := fmt.Sprintf("Studio: worker failed: %v", err)
		if res.AgentID != "" {
			msg += "\nagent_id: " + res.AgentID
		}
		_ = client.AddComment(ctx, studioOwner, studioRepo, *issueN, msg)
		_ = client.AddLabels(ctx, studioOwner, studioRepo, *issueN, []string{"needs-human"})
		return 2
	}

	b := &binding.Binding{
		AgentID:  res.AgentID,
		Worker:   "cursor",
		Repo:     strings.TrimPrefix(targetURL, "https://github.com/"),
		Branch:   firstNonEmpty(res.Branch, branch),
		PRURL:    res.PRURL,
		PRStatus: "open",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if res.PRURL != "" {
		if n := prNumber(res.PRURL); n != "" {
			b.PRNumber = n
		}
	}
	newBody, err := binding.Upsert(issue.Body, b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "binding: %v\n", err)
		_ = client.AddComment(ctx, studioOwner, studioRepo, *issueN, "Studio: binding corrupt — needs-human: "+err.Error())
		_ = client.AddLabels(ctx, studioOwner, studioRepo, *issueN, []string{"needs-human"})
		return 3
	}
	if err := client.UpdateIssue(ctx, studioOwner, studioRepo, *issueN, newBody, nil); err != nil {
		fmt.Fprintf(os.Stderr, "github update: %v\n", err)
		return 1
	}

	comment := fmt.Sprintf("Studio: started agent `%s`", res.AgentID)
	if res.PRURL != "" {
		comment += fmt.Sprintf("\nPR: %s", res.PRURL)
		_ = client.AddLabels(ctx, studioOwner, studioRepo, *issueN, []string{"pr-open"})
	} else {
		comment += "\nNo PR URL returned — needs-human if the run finished without a PR."
		_ = client.AddLabels(ctx, studioOwner, studioRepo, *issueN, []string{"needs-human"})
	}
	_ = client.AddComment(ctx, studioOwner, studioRepo, *issueN, comment)
	fmt.Println(comment)
	return 0
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteByte('-')
		}
		if b.Len() >= 40 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "work"
	}
	return out
}

func stripBinding(body string) string {
	const start = "<!-- studio:v1"
	const end = "-->"
	for {
		i := strings.Index(body, start)
		if i < 0 {
			return body
		}
		j := strings.Index(body[i:], end)
		if j < 0 {
			return body
		}
		body = body[:i] + body[i+j+len(end):]
	}
}

func prNumber(url string) string {
	// https://github.com/o/r/pull/12
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return ""
	}
	n := parts[len(parts)-1]
	if _, err := strconv.Atoi(n); err == nil {
		return n
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
