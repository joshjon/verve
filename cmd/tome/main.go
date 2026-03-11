package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/joshjon/verve/internal/tome"
)

func main() {
	app := &cli.App{
		Name:  "tome",
		Usage: "Agent session memory — record and search session history",
		Commands: []*cli.Command{
			searchCmd(),
			recordCmd(),
			logCmd(),
			indexCmd(),
			initCmd(),
			syncCmd(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func resolveDir() (string, error) {
	if dir := os.Getenv("TOME_DIR"); dir != "" {
		return dir, nil
	}

	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository and TOME_DIR not set")
	}
	return filepath.Join(strings.TrimSpace(string(out)), ".tome"), nil
}

func openTome() (*tome.Tome, error) {
	dir, err := resolveDir()
	if err != nil {
		return nil, err
	}
	return tome.Open(dir)
}

func searchCmd() *cli.Command {
	return &cli.Command{
		Name:      "search",
		Usage:     "Search session history",
		ArgsUsage: "QUERY",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "file", Usage: "Filter by files touched"},
			&cli.StringFlag{Name: "status", Usage: "Filter by status (succeeded/failed)"},
			&cli.IntFlag{Name: "limit", Aliases: []string{"n"}, Value: 5, Usage: "Max results"},
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
			&cli.BoolFlag{Name: "bm25-only", Usage: "Force BM25-only search (skip LSA)"},
		},
		Action: func(c *cli.Context) error {
			query := c.Args().First()
			if query == "" {
				return fmt.Errorf("search query required")
			}

			t, err := openTome()
			if err != nil {
				return err
			}
			defer t.Close()

			results, err := t.Search(c.Context, query, tome.SearchOpts{
				FilePattern: c.String("file"),
				Status:      c.String("status"),
				Limit:       c.Int("limit"),
				BM25Only:    c.Bool("bm25-only"),
			})
			if err != nil {
				return err
			}

			if c.Bool("json") {
				return tome.FormatJSON(os.Stdout, results)
			}
			tome.FormatSearchResults(os.Stdout, results)
			return nil
		},
	}
}

func recordCmd() *cli.Command {
	return &cli.Command{
		Name:  "record",
		Usage: "Record a session",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "summary", Required: true, Usage: "Session summary"},
			&cli.StringFlag{Name: "learnings", Usage: "Key learnings"},
			&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
			&cli.StringFlag{Name: "files", Usage: "Comma-separated files touched"},
			&cli.StringFlag{Name: "status", Value: "succeeded", Usage: "Session status (succeeded/failed)"},
			&cli.StringFlag{Name: "branch", Usage: "Git branch (auto-detected if not set)"},
			&cli.StringFlag{Name: "author", Usage: "Session author (auto-detected from git config if not set)"},
		},
		Action: func(c *cli.Context) error {
			t, err := openTome()
			if err != nil {
				return err
			}
			defer t.Close()

			branch := c.String("branch")
			if branch == "" {
				if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
					branch = strings.TrimSpace(string(out))
				}
			}

			author := c.String("author")
			if author == "" {
				author = detectAuthor()
			}

			s := tome.Session{
				Summary:   c.String("summary"),
				Learnings: c.String("learnings"),
				Tags:      splitCSV(c.String("tags")),
				Files:     splitCSV(c.String("files")),
				Status:    c.String("status"),
				Branch:    branch,
				Author:    author,
			}

			if err := t.Record(c.Context, s); err != nil {
				return err
			}

			fmt.Println("Session recorded.")
			return nil
		},
	}
}

func logCmd() *cli.Command {
	return &cli.Command{
		Name:  "log",
		Usage: "Show recent sessions",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "limit", Aliases: []string{"n"}, Value: 10, Usage: "Number of sessions"},
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(c *cli.Context) error {
			t, err := openTome()
			if err != nil {
				return err
			}
			defer t.Close()

			sessions, err := t.Log(c.Context, c.Int("limit"))
			if err != nil {
				return err
			}

			if c.Bool("json") {
				return tome.FormatJSON(os.Stdout, sessions)
			}
			tome.FormatLog(os.Stdout, sessions)
			return nil
		},
	}
}

func indexCmd() *cli.Command {
	return &cli.Command{
		Name:  "index",
		Usage: "Rebuild the LSA semantic search index",
		Action: func(c *cli.Context) error {
			t, err := openTome()
			if err != nil {
				return err
			}
			defer t.Close()

			numDocs, numTerms, dim, err := t.BuildIndex(c.Context)
			if err != nil {
				return fmt.Errorf("build index (%d sessions): %v", numDocs, err)
			}

			fmt.Printf("Built LSA index: %d sessions, %d terms, %d dimensions\n", numDocs, numTerms, dim)
			return nil
		},
	}
}

func initCmd() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize session database",
		Action: func(c *cli.Context) error {
			dir, err := resolveDir()
			if err != nil {
				return err
			}

			t, err := tome.Open(dir)
			if err != nil {
				return err
			}
			defer t.Close()

			fmt.Printf("Initialized tome at %s\n", dir)
			return nil
		},
	}
}

func syncCmd() *cli.Command {
	return &cli.Command{
		Name:  "sync",
		Usage: "Synchronize sessions with a git remote via orphan branches",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "pull", Usage: "Pull only (import from remote)"},
			&cli.BoolFlag{Name: "push", Usage: "Push only (export to remote)"},
			&cli.StringFlag{Name: "branch", Usage: "Override branch name (default: tome/context/<author>)"},
			&cli.StringFlag{Name: "author", Usage: "Author identity (auto-detected from git config if not set)"},
		},
		Action: func(c *cli.Context) error {
			t, err := openTome()
			if err != nil {
				return err
			}
			defer t.Close()

			repoDir, err := resolveRepoDir()
			if err != nil {
				return err
			}

			author := c.String("author")
			if author == "" {
				author = detectAuthor()
			}
			if author == "" && !c.Bool("pull") {
				return fmt.Errorf("author required for push (set via --author or git config user.email)")
			}

			result, err := t.Sync(c.Context, repoDir, author, tome.SyncOpts{
				PullOnly: c.Bool("pull"),
				PushOnly: c.Bool("push"),
				Branch:   c.String("branch"),
			})
			if err != nil {
				return err
			}

			if result.Imported > 0 {
				fmt.Printf("Imported %d sessions.\n", result.Imported)
			}
			if result.Exported > 0 {
				fmt.Printf("Exported %d sessions.\n", result.Exported)
			}
			if result.Imported == 0 && result.Exported == 0 {
				fmt.Println("Already up to date.")
			}
			return nil
		},
	}
}

func resolveRepoDir() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

func detectAuthor() string {
	if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func splitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
