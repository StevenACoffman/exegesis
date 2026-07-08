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
	"github.com/StevenACoffman/exegesis/internal/mergedoc"
	"github.com/StevenACoffman/exegesis/internal/render"
	"github.com/StevenACoffman/exegesis/internal/store"
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
	model, err := assemble(tree, sources)
	if err != nil {
		return err
	}
	out := render.MergeIndex(model)
	if cfg.Check {
		return cfg.check(tree, out)
	}
	return cfg.write(tree, out)
}

// assemble builds the merge-index model from the merged tree and source books.
func assemble(tree string, sources []string) (*book2skill.MergeIndex, error) {
	runSlug := filepath.Base(filepath.Clean(tree))
	merged, err := store.GatherSkills(tree)
	if err != nil {
		return nil, fmt.Errorf("merge-index: %w", err)
	}
	parents := make(map[string][]book2skill.MergeParent)
	model := &book2skill.MergeIndex{RunSlug: runSlug}
	for _, srcDir := range sources {
		book, err := readSourceBook(srcDir, runSlug, parents)
		if err != nil {
			return nil, err
		}
		model.Sources = append(model.Sources, book)
	}
	for i := range merged {
		model.Merges = append(model.Merges, book2skill.MergeRecord{
			Slug: merged[i].Slug, Title: merged[i].Title, Parents: parents[merged[i].Slug],
		})
	}
	return model, nil
}

// readSourceBook reads one source book and records, per merged skill, the source
// skills whose ledger says they merged into it during runSlug.
func readSourceBook(
	srcDir, runSlug string, parents map[string][]book2skill.MergeParent,
) (book2skill.MergeSourceBook, error) {
	slug := filepath.Base(filepath.Clean(srcDir))
	book := book2skill.MergeSourceBook{Slug: slug, Title: slug, Superseded: map[string]bool{}}
	if o, ok, _ := store.ReadOverview(srcDir); ok {
		book.Title, book.Author = o.Title, o.Author
	}
	skills, err := store.GatherSkills(srcDir)
	if err != nil {
		return book, fmt.Errorf("merge-index: %w", err)
	}
	for i := range skills {
		sk := skills[i].Slug
		book.Skills = append(book.Skills, sk)
		data, readErr := os.ReadFile(filepath.Join(srcDir, sk, store.SkillFile))
		if readErr != nil {
			continue
		}
		entries, parseErr := mergedoc.Parse(string(data))
		if parseErr != nil {
			return book, fmt.Errorf("merge-index: %w", parseErr)
		}
		recordParents(entries, runSlug, slug, sk, book.Superseded, parents)
	}
	return book, nil
}

func recordParents(
	entries []book2skill.MergeStatusEntry,
	runSlug, bookSlug, skillSlug string,
	superseded map[string]bool,
	parents map[string][]book2skill.MergeParent,
) {
	for j := range entries {
		e := entries[j]
		if e.Run != runSlug || e.Into == "" {
			continue
		}
		if e.State != book2skill.StateMerged && e.State != book2skill.StatePartial {
			continue
		}
		superseded[skillSlug] = true
		parents[e.Into] = append(parents[e.Into], book2skill.MergeParent{
			BookSlug: bookSlug, SkillSlug: skillSlug, State: e.State,
		})
	}
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
