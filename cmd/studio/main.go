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
	"github.com/adamfriedl/studio/internal/provision"
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
	case "watch":
		os.Exit(cmdWatch(os.Args[2:]))
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
  studio watch [--issue N] [--dry-run] [--catalog PATH]

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
	if len(c.Templates) > 0 {
		fmt.Println("templates:")
		for name, t := range c.Templates {
			fmt.Printf("  - %s ← %s\n", name, t.From)
		}
	}
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

	// Offline dry-run when labels are stubbed, even if a token is present in the env.
	if *dryRun && strings.TrimSpace(os.Getenv("STUDIO_DRY_LABELS")) != "" {
		labelNames = strings.Split(os.Getenv("STUDIO_DRY_LABELS"), ",")
		issue = &gh.Issue{
			Number: *issueN,
			Title:  envOr("STUDIO_DRY_TITLE", "dry-run issue"),
			Body:   envOr("STUDIO_DRY_BODY", "dry-run body"),
		}
	} else if *dryRun && token == "" {
		fmt.Fprintln(os.Stderr, "dry-run without token requires STUDIO_DRY_LABELS")
		return 1
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

	catalogFile := findCatalog(*catalogPath)
	var client *gh.Client
	if !*dryRun {
		client = &gh.Client{Token: token}
	}

	if kind == "new" {
		templateKey := name // TargetFromLabels returns template key as name
		templateFrom := targetURL // owner/template-repo
		newName, err := catalog.NewRepoName(issue.Title, stripBinding(issue.Body))
		if err != nil {
			fmt.Fprintf(os.Stderr, "needs-human: %v\n", err)
			if client != nil {
				_ = client.AddComment(ctx, studioOwner, studioRepo, *issueN, "Studio: "+err.Error())
				_ = client.AddLabels(ctx, studioOwner, studioRepo, *issueN, []string{"needs-human"})
			}
			return 3
		}
		name = newName
		targetURL = "https://github.com/" + c.Org + "/" + name
		fmt.Printf("new: template=%s (%s) → %s/%s\n", templateKey, templateFrom, c.Org, name)

		if *dryRun {
			fmt.Printf("dry-run: would create from template, allowlist %q, provision CI/hook/HOOK_TOKEN, ensure label repo:%s, then Start\n", name, name)
		} else if err := provisionNewRepo(ctx, client, c, catalogFile, studioOwner, studioRepo, *issueN, templateKey, templateFrom, name); err != nil {
			fmt.Fprintf(os.Stderr, "needs-human: %v\n", err)
			_ = client.AddComment(ctx, studioOwner, studioRepo, *issueN, "Studio: new: failed: "+err.Error())
			_ = client.AddLabels(ctx, studioOwner, studioRepo, *issueN, []string{"needs-human"})
			return 3
		}
	}

	branch := fmt.Sprintf("%s%d-%s", c.Defaults.BranchPrefix, *issueN, slug(issue.Title))
	studioIssueURL := fmt.Sprintf("https://github.com/%s/%s/issues/%d", studioOwner, studioRepo, *issueN)
	implModel := catalog.ResolveModel("implement", labelNames, stripBinding(issue.Body))
	qcModel := catalog.ResolveModel("qc", labelNames, stripBinding(issue.Body))
	if o := catalog.ModelOverride(labelNames, stripBinding(issue.Body)); o != "" {
		if _, err := catalog.NormalizeModel(o); err != nil {
			fmt.Fprintf(os.Stderr, "model override %q rejected — using included pool (%s)\n", o, implModel)
		}
	}

	var existingBind *binding.Binding
	if b, err := binding.Parse(issue.Body); err == nil {
		existingBind = b
	}
	specStatus := ""
	if existingBind != nil {
		specStatus = existingBind.SpecStatus
	}

	fmt.Printf("target: %s (%s)\n", targetURL, name)
	fmt.Printf("branch: %s\n", branch)
	fmt.Printf("model implement=%s qc=%s\n", implModel, qcModel)

	needQC := !catalog.SpecAllowsStart(labelNames, specStatus)
	if needQC {
		specFrag, cont, code := runIntakeQC(ctx, client, studioOwner, studioRepo, *issueN, issue, targetURL, qcModel, existingBind, *dryRun)
		if code != 0 {
			return code
		}
		if !cont {
			return 0
		}
		if existingBind == nil {
			existingBind = &binding.Binding{}
		}
		existingBind.SpecStatus = specFrag.SpecStatus
		existingBind.SpecAgentID = specFrag.SpecAgentID
		// Refresh issue body after QC upsert for later binding merge.
		if !*dryRun && client != nil {
			if refreshed, err := client.GetIssue(ctx, studioOwner, studioRepo, *issueN); err == nil {
				issue = refreshed
				if b, err := binding.Parse(issue.Body); err == nil {
					existingBind = b
				}
			}
		}
	} else {
		fmt.Println("intake: skipped (skip-qc / spec-ok / approved)")
	}

	p := prompt.Implement(prompt.ImplementInput{
		IssueNumber:    *issueN,
		Title:          issue.Title,
		Body:           stripBinding(issue.Body),
		RepoURL:        targetURL,
		Branch:         branch,
		StudioIssueURL: studioIssueURL,
	})

	if *dryRun {
		fmt.Println("--- implement prompt ---")
		fmt.Println(p)
		fmt.Println("dispatch: dry-run ok (no worker)")
		return 0
	}

	if strings.TrimSpace(os.Getenv("CURSOR_API_KEY")) == "" {
		fmt.Fprintln(os.Stderr, "CURSOR_API_KEY required")
		return 1
	}

	_ = client.AddLabels(ctx, studioOwner, studioRepo, *issueN, []string{"working"})
	_ = client.RemoveLabel(ctx, studioOwner, studioRepo, *issueN, "needs-spec")
	w := cursor.Helper{Timeout: 45 * time.Minute}
	res, err := w.Start(ctx, worker.StartReq{
		IssueNumber:  *issueN,
		Title:        issue.Title,
		Body:         issue.Body,
		RepoURL:      targetURL,
		StartingRef:  "main",
		BranchName:   branch,
		Model:        implModel,
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
		AgentID:   res.AgentID,
		Worker:    "cursor",
		Repo:      strings.TrimPrefix(targetURL, "https://github.com/"),
		Branch:    firstNonEmpty(res.Branch, branch),
		PRURL:     res.PRURL,
		PRStatus:  "open",
		Model:     implModel,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if existingBind != nil {
		b.SpecStatus = existingBind.SpecStatus
		b.SpecAgentID = existingBind.SpecAgentID
		if b.SpecStatus == "" && catalog.SpecAllowsStart(labelNames, "") {
			b.SpecStatus = "skipped"
		}
	} else if catalog.HasLabel(labelNames, "skip-qc") {
		b.SpecStatus = "skipped"
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

	comment := fmt.Sprintf("Studio: started agent `%s` (model `%s`)", res.AgentID, implModel)
	if res.PRURL != "" {
		comment += fmt.Sprintf("\nPR: %s", res.PRURL)
		_ = client.AddLabels(ctx, studioOwner, studioRepo, *issueN, []string{"pr-open"})
		// Don't wait on Actions cron — kick watch for Phase 6 / comment FollowUp.
		if err := client.RepositoryDispatch(ctx, studioOwner, studioRepo, "studio-watch", map[string]any{
			"issue_number": *issueN,
			"reason":       "dispatch-pr-open",
			"pr_url":       res.PRURL,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "watch kick: %v\n", err)
			comment += "\n(watch kick failed — cron/target-hook/backstop still apply)"
		} else {
			comment += "\nWatch kicked (`studio-watch`)."
		}
	} else {
		comment += "\nNo PR URL returned — needs-human if the run finished without a PR."
		_ = client.AddLabels(ctx, studioOwner, studioRepo, *issueN, []string{"needs-human"})
	}
	_ = client.AddComment(ctx, studioOwner, studioRepo, *issueN, comment)
	fmt.Println(comment)
	return 0
}

// provisionNewRepo creates from template, allowlists in repos.yaml, ensures repo:<name> label,
// then upserts CI + studio-hook and copies App secrets. Must complete before Worker.Start.
func provisionNewRepo(ctx context.Context, client *gh.Client, c *catalog.Catalog, catalogPath, studioOwner, studioRepo string, issueN int, templateKey, templateFrom, name string) error {
	if _, blocked := c.TemplateSourceNames()[name]; blocked {
		return fmt.Errorf("repo name %q is a template source — pick another name", name)
	}

	createClient := clientForRepoCreate(client)
	repo, err := createClient.CreateFromTemplate(ctx, templateFrom, c.Org, name, true)
	created := err == nil
	if err != nil {
		if err != gh.ErrRepoExists {
			return fmt.Errorf("create from template %s: %w", templateFrom, err)
		}
		// Idempotent retry only: name already allowlisted (prior successful new: or manual).
		if !c.HasRepo(name) {
			return fmt.Errorf("repo %s/%s already exists and is not allowlisted — pick another name or use repo:%s", c.Org, name, name)
		}
		fmt.Printf("new: reusing allowlisted %s\n", repo.FullName)
	} else {
		fmt.Printf("new: created %s\n", repo.FullName)
	}

	if err := client.EnsureLabel(ctx, studioOwner, studioRepo, "repo:"+name, "0E8A16", "Target: "+name); err != nil {
		return fmt.Errorf("ensure label repo:%s: %w", name, err)
	}

	if !c.HasRepo(name) {
		file, err := client.GetFile(ctx, studioOwner, studioRepo, "repos.yaml")
		if err != nil {
			return fmt.Errorf("read repos.yaml: %w", err)
		}
		updated, err := catalog.AppendRepoYAML(file.Content, name)
		if err != nil {
			return err
		}
		if string(updated) != string(file.Content) {
			msg := fmt.Sprintf("chore(catalog): allowlist %s (studio#%d)", name, issueN)
			if err := client.PutFile(ctx, studioOwner, studioRepo, "repos.yaml", msg, updated, file.SHA); err != nil {
				return fmt.Errorf("commit repos.yaml: %w", err)
			}
			fmt.Printf("new: committed allowlist %s\n", name)
		}
		c.AppendRepo(name)
		_ = writeLocalCatalog(catalogPath, updated)
	}

	if created {
		_ = client.AddComment(ctx, studioOwner, studioRepo, issueN,
			fmt.Sprintf("Studio: created `%s` from template `%s` and allowlisted it.", repo.FullName, templateFrom))
	} else {
		_ = client.AddComment(ctx, studioOwner, studioRepo, issueN,
			fmt.Sprintf("Studio: reusing allowlisted `%s` (already existed).", repo.FullName))
	}

	// Prefer PAT for secrets/contents if App lacks secrets:write (same as generate).
	provClient := clientForRepoCreate(client)
	res, err := provision.Ensure(ctx, provClient, c.Org, name, templateKey)
	if err != nil {
		return fmt.Errorf("provision target: %w", err)
	}
	fmt.Printf("new: provisioned files=%v secrets=%v\n", res.Files, res.Secrets)
	_ = client.AddComment(ctx, studioOwner, studioRepo, issueN,
		fmt.Sprintf("Studio: provisioned `%s` — workflows %v; secrets %v.", repo.FullName, res.Files, res.Secrets))
	return nil
}

// clientForRepoCreate prefers STUDIO_GITHUB_PAT for template generate / secret put when the
// App token lacks administration or secrets:write.
func clientForRepoCreate(primary *gh.Client) *gh.Client {
	pat := strings.TrimSpace(os.Getenv("STUDIO_GITHUB_PAT"))
	if pat == "" || primary == nil || pat == primary.Token {
		return primary
	}
	fmt.Println("new: using STUDIO_GITHUB_PAT for template generate / provision")
	return &gh.Client{Token: pat, BaseURL: primary.BaseURL, HTTPClient: primary.HTTPClient, UserAgent: primary.UserAgent}
}

func writeLocalCatalog(path string, content []byte) error {
	if path == "" {
		return nil
	}
	return os.WriteFile(path, content, 0o644)
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
