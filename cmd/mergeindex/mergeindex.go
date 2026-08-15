// Package mergeindex implements `exegesis merge-index`: it regenerates a merged tree's
// INDEX.md — the cross-book provenance table, the source-verification summary, and the
// rejected pairs — from each merged skill's `## Provenance` section and the tree's
// source-verification/ and rejected/ files. The rendering is pure
// (internal/mergeindex); this command reads/writes the file. Distinct from `index`,
// which builds a book tree's skill list and knows nothing about merge provenance.
package mergeindex

import (
	"context"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/mergeindexgen"
	"github.com/StevenACoffman/skillet/atomicfile"
)

// Config holds the merge-index command configuration.
type Config struct {
	*root.Config
	Check   bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the merge-index command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("merge-index").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Check, 0, "check",
		"verify INDEX.md is current without writing it (exit 1 if stale)")
	cfg.Command = &ff.Command{
		Name:      "merge-index",
		Usage:     "exegesis merge-index [--check] [MERGED_TREE]",
		ShortHelp: "regenerate a merged tree's cross-book provenance INDEX.md",
		LongHelp: "Read every merged skill under MERGED_TREE (default .), build the cross-book\n" +
			"provenance table (from each skill's `## Provenance` section, marking sources that\n" +
			"feed two or more merged skills), the source-verification summary (from\n" +
			"source-verification/*.md headers, empty until a merge run writes them), and the\n" +
			"rejected pairs (from rejected/pair-*.md), and write MERGED_TREE/INDEX.md. With\n" +
			"--check, compare against the existing file and exit 1 if stale instead of writing.",
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
		return root.Usagef("merge-index: expected at most one tree path")
	}
	out, err := mergeindexgen.Generate(tree)
	if err != nil {
		return fmt.Errorf("merge-index: %w", err)
	}
	path := mergeindexgen.Path(tree)
	return cfg.writeOrCheck(path, out, readFile(path))
}

// writeOrCheck writes out to path, or under --check reports whether the existing file
// already matches and exits non-zero when it does not.
func (cfg *Config) writeOrCheck(path, out, existing string) error {
	if cfg.Check {
		if out == existing {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s is up to date\n", path)
			return nil
		}
		_, _ = fmt.Fprintf(cfg.Stdout, "%s is stale (run: exegesis merge-index)\n", path)
		return root.ExitError(1)
	}
	if err := atomicfile.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("merge-index: write %s: %w", path, err)
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
