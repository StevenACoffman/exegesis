// Package index implements the "index" command: it regenerates INDEX.md for a
// distilled books/<slug>/ tree from each skill's "## Related skills" section —
// the skill list, a Mermaid relationship graph, and a dependency-ordered
// learning path — deterministically, without an LLM.
package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/render"
	"github.com/StevenACoffman/exegesis/internal/store"
)

const filePerm = 0o644

// Config holds the flags and wiring for the index command.
type Config struct {
	*root.Config
	Title   string
	Author  string
	Check   bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the index command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("index").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Title, 0, "title", "", "book title (default: from BOOK_OVERVIEW.md)")
	cfg.Flags.StringVar(
		&cfg.Author,
		0,
		"author",
		"",
		"book author (default: from BOOK_OVERVIEW.md)",
	)
	cfg.Flags.BoolVar(&cfg.Check, 0, "check", "verify INDEX.md is current; exit 1 if stale")
	cfg.Command = &ff.Command{
		Name:      "index",
		Usage:     "exegesis index [FLAGS] <book-dir>",
		ShortHelp: "regenerate INDEX.md for a distilled book tree",
		LongHelp: `Regenerate <book-dir>/INDEX.md from the skills under <book-dir>:
the skill list, a Mermaid relationship graph, and a dependency-ordered learning
path, all recovered from each skill's "## Related skills" section. --check
verifies the file is current without writing (exit 1 if stale); --title /
--author override the header derived from BOOK_OVERVIEW.md.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return einval("index: a book directory is required")
	}
	dir := args[0]
	skills, err := store.GatherSkills(dir)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	if len(skills) == 0 {
		return einval("index: no skills (SKILL.md directories) found under " + dir)
	}
	if msg := book2skill.RelationshipCountAdvice(skills); msg != "" {
		_, _ = fmt.Fprintln(cfg.Stderr, "warning: "+msg)
	}
	existing := readOrEmpty(filepath.Join(dir, store.IndexFile))
	out := book2skill.AppendCustomSections(render.Index(cfg.overview(dir), skills), existing)
	if cfg.Check {
		return cfg.check(existing, out)
	}
	return cfg.write(dir, out)
}

// overview derives the book header from BOOK_OVERVIEW.md, with flag overrides
// and a directory-name fallback for the title. A read error is non-fatal — the
// header is cosmetic — so it falls back like an absent file.
func (cfg *Config) overview(dir string) *book2skill.BookOverview {
	o, ok, _ := store.ReadOverview(dir)
	if !ok || o == nil {
		o = &book2skill.BookOverview{}
	}
	if cfg.Title != "" {
		o.Title = cfg.Title
	}
	if cfg.Author != "" {
		o.Author = cfg.Author
	}
	if o.Title == "" {
		o.Title = filepath.Base(filepath.Clean(dir))
	}
	return o
}

func (cfg *Config) check(existing, want string) error {
	if existing != want {
		_, _ = fmt.Fprintln(cfg.Stderr,
			"index: "+store.IndexFile+" is stale; run `exegesis index` to regenerate")
		return root.ExitError(1)
	}
	_, _ = fmt.Fprintln(cfg.Stdout, store.IndexFile+" is up to date")
	return nil
}

// readOrEmpty returns the file's contents, or "" if it cannot be read.
func readOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func (cfg *Config) write(dir, out string) error {
	path := filepath.Join(dir, store.IndexFile)
	if err := os.WriteFile(path, []byte(out), filePerm); err != nil {
		return &book2skill.Error{Op: "index.write", Err: err}
	}
	_, _ = fmt.Fprintln(cfg.Stdout, "wrote "+path)
	return nil
}

func einval(msg string) error {
	return &book2skill.Error{Code: book2skill.EINVALID, Message: msg}
}
