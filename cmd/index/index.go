// Package index implements `exegesis index`: it regenerates a skill tree's
// INDEX.md — the skill list, the Mermaid relationship graph, and a
// dependency-ordered learning path — from every skill's `## Related skills`
// section. The rendering is pure (internal/related); this command discovers the
// skills, resolves the header, and reads/writes files.
package index

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/indexgen"
	"github.com/StevenACoffman/skillet/atomicfile"
)

// Config holds the index command configuration.
type Config struct {
	*root.Config
	Check   bool
	Title   string
	Author  string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the index command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("index").SetParent(parent.Flags)
	cfg.Flags.BoolVar(
		&cfg.Check,
		0,
		"check",
		"verify INDEX.md is current without writing it (exit 1 if stale)",
	)
	cfg.Flags.StringVar(
		&cfg.Title,
		0,
		"title",
		"",
		"header title (default: BOOK_OVERVIEW.md's first heading)",
	)
	cfg.Flags.StringVar(&cfg.Author, 0, "author", "", "header author")
	cfg.Command = &ff.Command{
		Name:      "index",
		Usage:     "exegesis index [--check] [--title T] [--author A] [TREE]",
		ShortHelp: "regenerate INDEX.md from every skill's `## Related skills` section",
		LongHelp: "Read every skill under TREE (default .), build the skill list, the Mermaid\n" +
			"relationship graph, and a learning path topologically ordered on depends-on\n" +
			"edges, and write TREE/INDEX.md. Sections you add below the generated block are\n" +
			"preserved. With --check, compare against the existing file and exit 1 if stale\n" +
			"instead of writing.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	tree := "."
	switch len(args) {
	case 0:
	case 1:
		tree = args[0]
	default:
		return errors.New("index: expected at most one tree path")
	}
	out, err := indexgen.Generate(tree, cfg.Title, cfg.Author)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	path := indexgen.Path(tree)
	return cfg.writeOrCheck(path, out, readFile(path))
}

// writeOrCheck writes out to path, or under --check reports whether the existing
// file already matches and exits non-zero when it does not.
func (cfg *Config) writeOrCheck(path, out, existing string) error {
	if cfg.Check {
		if out == existing {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s is up to date\n", path)
			return nil
		}
		_, _ = fmt.Fprintf(cfg.Stdout, "%s is stale (run: exegesis index)\n", path)
		return root.ExitError(1)
	}
	if err := atomicfile.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("index: write %s: %w", path, err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "wrote %s\n", path)
	return nil
}

// readFile returns the file's contents, or "" when it does not exist.
func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}
