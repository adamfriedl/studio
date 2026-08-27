package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adamfriedl/studio/internal/catalog"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "doctor":
		os.Exit(cmdDoctor(os.Args[2:]))
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

Exit codes: 0 ok; 1 config/auth; 2 worker failed; 3 needs-human
`)
}

func cmdDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "print catalog only; no network")
	catalogPath := fs.String("catalog", "repos.yaml", "path to repos.yaml")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}

	path := *catalogPath
	if !filepath.IsAbs(path) {
		if wd, err := os.Getwd(); err == nil {
			path = filepath.Join(wd, path)
		}
	}

	c, err := catalog.Load(path)
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
	if len(c.Templates) > 0 {
		fmt.Println("templates:")
		for name, t := range c.Templates {
			fmt.Printf("  - %s ← %s\n", name, t.From)
		}
	}

	if *dryRun {
		fmt.Println("doctor: dry-run ok (no network)")
		return 0
	}

	// Live pings land in Phase 1+.
	fmt.Println("doctor: live GitHub/Cursor ping not implemented yet; use --dry-run")
	return 0
}
