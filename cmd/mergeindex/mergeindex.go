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
	cfg.Flags.StringVar(&cfg.Source, 0, "source-book", "",
		"comma-separated source book dirs (optional under books/merged/: auto-discovered)")
	cfg.Flags.BoolVar(&cfg.Check, 0, "check", "verify INDEX.md is current; exit 1 if stale")
	cfg.Command = &ff.Command{
		Name:      "merge-index",
		Usage:     "exegesis merge-index [--source-book <bookA>,<bookB>] <merged-tree>",
		ShortHelp: "regenerate the cross-book INDEX.md for a merged-skills tree",
		LongHelp: `Regenerate <merged-tree>/INDEX.md from the merge-status ledgers on
the source skills: source-books table, provenance table, cross-book graph,
superseded source skills, and (from the artifact headers) the source-verification
summary. When <merged-tree> is under a books/merged/ root, --source-book is optional —
the contributing source books are discovered automatically. Hand-added sections
below the generated ones are preserved. --check compares without writing (exit 1
if stale), padding-normalized so a formatted table still matches.`,
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
	tree := args[0]
	sources, err := cfg.sources(tree)
	if err != nil {
		return err
	}
	model, err := mergetree.Assemble(tree, sources)
	if err != nil {
		return fmt.Errorf("merge-index: %w", err)
	}
	existing := readOrEmpty(filepath.Join(tree, indexFile))
	out := book2skill.AppendCustomSections(render.MergeIndex(model), existing)
	if cfg.Check {
		return cfg.check(existing, out)
	}
	return cfg.write(tree, out)
}

// sources returns the explicit --source dirs, or — when none are given — the
// dirs discovered under the books/merged/ root. It errors when neither yields
// any source book.
func (cfg *Config) sources(tree string) ([]string, error) {
	if explicit := splitCSV(cfg.Source); len(explicit) > 0 {
		return explicit, nil
	}
	discovered, err := mergetree.DiscoverSources(tree)
	if err != nil {
		return nil, einval(
			"merge-index: no --source-book given and could not infer them (" + err.Error() + ")",
		)
	}
	if len(discovered) == 0 {
		return nil, einval(
			"merge-index: no --source-book given and no contributing source books found",
		)
	}
	return discovered, nil
}

func (cfg *Config) check(existing, want string) error {
	if !tableEqual(existing, want) {
		_, _ = fmt.Fprintln(cfg.Stderr,
			"merge-index: "+indexFile+" is stale; run `exegesis merge-index` to regenerate")
		return root.ExitError(1)
	}
	_, _ = fmt.Fprintln(cfg.Stdout, indexFile+" is up to date")
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
