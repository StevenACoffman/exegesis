// Package mergeindex implements the "merge-index" command: it regenerates the
// cross-book INDEX.md for a books/merged/<slug>/ tree from the merge-status
// ledgers on the source skills — the source-books table, provenance table,
// cross-book relationship graph, and superseded source skills — deterministically.
// The judgment-only sections (source-verification summary, notes) are left to the
// agent.
package mergeindex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/mergetree"
	"github.com/StevenACoffman/exegesis/internal/render"
)

const (
	indexFile = "INDEX.md"
	filePerm  = 0o644
)

// Config holds the flags and wiring for the merge-index command.
type Config struct {
	*root.Config
	Source  string
	Check   bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the merge-index command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("merge-index").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Source, 0, "source", "",
		"comma-separated source book directories the merge drew from (required)")
	cfg.Flags.BoolVar(&cfg.Check, 0, "check", "verify INDEX.md is current; exit 1 if stale")
	cfg.Command = &ff.Command{
		Name:      "merge-index",
		Usage:     "exegesis merge-index --source <bookA>,<bookB> <merged-tree>",
		ShortHelp: "regenerate the cross-book INDEX.md for a merged-skills tree",
		LongHelp: `Regenerate <merged-tree>/INDEX.md from the merge-status ledgers on
the source skills: source-books table, provenance table, cross-book graph, and
superseded source skills. --check compares without writing (exit 1 if stale),
padding-normalized so a formatted table still matches. Flags come before the
directory. Judgment sections (source-verification summary, notes) are left to you.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return einval("merge-index: a merged-tree directory is required")
	}
	sources := splitCSV(cfg.Source)
	if len(sources) == 0 {
		return einval("merge-index: at least one --source book directory is required")
	}
	tree := args[0]
	model, err := mergetree.Assemble(tree, sources)
	if err != nil {
		return fmt.Errorf("merge-index: %w", err)
	}
	out := render.MergeIndex(model)
	if cfg.Check {
		return cfg.check(tree, out)
	}
	return cfg.write(tree, out)
}

func (cfg *Config) check(tree, want string) error {
	got, err := os.ReadFile(filepath.Join(tree, indexFile))
	if err != nil || !tableEqual(string(got), want) {
		_, _ = fmt.Fprintln(cfg.Stderr,
			"merge-index: "+indexFile+" is stale; run `exegesis merge-index` to regenerate")
		return root.ExitError(1)
	}
	_, _ = fmt.Fprintln(cfg.Stdout, indexFile+" is up to date")
	return nil
}

func (cfg *Config) write(tree, out string) error {
	path := filepath.Join(tree, indexFile)
	if err := os.WriteFile(path, []byte(out), filePerm); err != nil {
		return &book2skill.Error{Op: "mergeindex.write", Err: err}
	}
	_, _ = fmt.Fprintln(cfg.Stdout, "wrote "+path)
	return nil
}

// tableEqual compares two documents ignoring the two deterministic changes a
// markdown formatter makes to a rendered index: table-cell padding (and
// delimiter dash widths) and heading title-casing. Both are cosmetic — the
// provenance, graph, and superseded content are unaffected — so a formatted
// INDEX.md still matches the freshly rendered single-space one.
func tableEqual(a, b string) bool {
	return normalizeForCompare(a) == normalizeForCompare(b)
}

func normalizeForCompare(md string) string {
	dashRun := regexp.MustCompile(`-{2,}`)
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "|"):
			collapsed := strings.Join(strings.Fields(line), " ")
			lines[i] = dashRun.ReplaceAllString(collapsed, "-")
		case strings.HasPrefix(trimmed, "#"):
			lines[i] = strings.ToLower(strings.Join(strings.Fields(line), " "))
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func einval(msg string) error {
	return &book2skill.Error{Code: book2skill.EINVALID, Message: msg}
}
