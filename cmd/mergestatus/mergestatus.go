// Package mergestatus implements the "merge-status" command group: append a verdict to
// a source skill's merge ledger, or validate every ledger under a tree. The schema and
// the append-only splice are pure and shared (internal/mergestatus); this command does
// the file I/O.
package mergestatus

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/indexgen"
	ledger "github.com/StevenACoffman/exegesis/internal/mergestatus"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/related"
	"github.com/StevenACoffman/skillet/skill"
)

// Config holds the merge-status command configuration.
type Config struct {
	*root.Config
	Entry   ledger.Entry
	Link    bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the merge-status command group.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("merge-status").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "merge-status",
		Usage:     "exegesis merge-status <append|check> ...",
		ShortHelp: "append to, or validate, a source skill's merge ledger",
		LongHelp: `A source skill's merge ledger is the ` + ledger.Heading + ` section: one entry per
merge run that evaluated the skill, recording the fate that run assigned and why.

It is an audit trail, so an entry already on disk is never re-rendered -- a new one is
spliced in ahead of the closing fence -- so an append cannot reformat, reorder or lose
an earlier run's record.

With --link, one append also writes the superseded-by edge that names the merged skill
in the source skill's ` + "`## Related Skills`" + ` section. The two records have
distinct jobs and are written together on purpose: the ledger is authoritative for a
source skill's fate (including the states no merged skill exists to record), the edge is
the navigation pointer a reader of the dead skill needs. Neither is hand-edited.

The ledger is a body section rather than frontmatter because "merge_status" is not a
spec-allowed frontmatter key and would fail "exegesis lint" on the source skill.`,
		Flags:       cfg.Flags,
		Subcommands: []*ff.Command{cfg.appendCommand(), cfg.checkCommand()},
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// appendCommand builds the "append" subcommand.
func (cfg *Config) appendCommand() *ff.Command {
	fs := ff.NewFlagSet("append").SetParent(cfg.Flags)
	fs.StringVar(&cfg.Entry.Run, 0, "run", "", "the merge run's slug")
	fs.StringVar(&cfg.Entry.State, 0, "state", "",
		"the fate assigned: "+strings.Join(stateNames(), ", "))
	fs.StringVar(&cfg.Entry.Pair, 0, "pair", "",
		"the pair id; required for surface-resemblance, complementary and rejected")
	fs.StringVar(&cfg.Entry.Into, 0, "into", "",
		"the merged skill's slug; required for merged and partial")
	fs.StringVar(&cfg.Entry.Reason, 0, "reason", "",
		"why it was rejected; required for rejected only")
	fs.StringVar(&cfg.Entry.Excluded, 0, "excluded", "",
		"what content was left out; required for partial")
	fs.BoolVar(&cfg.Link, 0, "link", "also record the superseded-by edge in the skill's "+
		"`## Related Skills` section; needs a state that takes --into")
	return &ff.Command{
		Name:      "append",
		Usage:     "exegesis merge-status append --run SLUG --state STATE [flags] SKILL_DIR",
		ShortHelp: "record one merge run's verdict on a source skill",
		LongHelp: `Append one entry to SKILL_DIR's ledger, creating the section when absent.

The state determines which other flags are required, and equally which are refused:
a rejected entry naming what it merged into would be two contradictory accounts of
one decision, so a flag the state has no use for is an error rather than ignored.

  no-candidate         (nothing further)
  surface-resemblance  --pair
  complementary        --pair
  rejected             --pair --reason
  merged               --into
  partial              --into --excluded

With --link the same call also records a superseded-by edge pointing at
merged/<run>/<into>, the tree-qualified form the merged skill is written under. Only
the two states that take --into can imply that edge, so --link with any other state is
an error rather than a silent no-op. The ledger and the edge are written in one pass
over the file, so a failure leaves neither.

Flags come before the directory: flag parsing stops at the first positional.`,
		Flags: fs,
		Exec:  cfg.execAppend,
	}
}

// checkCommand builds the "check" subcommand.
func (cfg *Config) checkCommand() *ff.Command {
	fs := ff.NewFlagSet("check").SetParent(cfg.Flags)
	return &ff.Command{
		Name:      "check",
		Usage:     "exegesis merge-status check TREE",
		ShortHelp: "validate every merge ledger under a tree",
		LongHelp: `Read every skill under TREE and validate its ledger against the schema, reporting
one line per problem and exiting non-zero if any skill fails.

A skill with no ledger has never been evaluated in any merge run. That is the normal
state for most of a tree and is not reported.`,
		Flags: fs,
		Exec:  cfg.execCheck,
	}
}

func (cfg *Config) execAppend(_ context.Context, args []string) error {
	if len(args) != 1 {
		return root.Usagef("merge-status append: pass exactly one skill directory")
	}
	if problems := cfg.Entry.Validate(); len(problems) > 0 {
		for _, p := range problems {
			_, _ = fmt.Fprintf(cfg.Stdout, "  - %s\n", p)
		}
		return root.ExitError(1)
	}
	edge, err := cfg.linkEdge()
	if err != nil {
		return err
	}
	dir := args[0]
	path := filepath.Join(dir, skill.FileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("merge-status append: read %s: %w", path, err)
	}
	out, linked, err := record(string(raw), &cfg.Entry, edge, cfg.Link)
	if err != nil {
		return err
	}
	if err := atomicfile.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("merge-status append: write %s: %w", path, err)
	}
	name := filepath.Base(dir)
	_, _ = fmt.Fprintf(cfg.Stdout, "%s: recorded %s for run %s\n",
		name, cfg.Entry.State, cfg.Entry.Run)
	if cfg.Link {
		// "linked" on a re-run would claim an edge was added that was already there.
		verb := "linked"
		if !linked {
			verb = "already linked"
		}
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: %s %s `%s`\n", name, verb, edge.Kind, edge.Target)
		cfg.warnUnresolvedTarget(dir, edge.Target)
	}
	return nil
}

// record returns the document with the ledger entry appended and, under link, the
// superseded-by edge recorded; linked reports whether the edge was added or its rationale
// updated, as against already being there.
//
// Both edits are made to one string and written once, so a failure between them cannot
// leave a ledger claiming a merge that no edge points at, or the reverse. The ledger goes
// first because it is the command's primary act.
//
// The two records are idempotent in different ways, which is not an inconsistency: an
// audit trail records every run, so a repeated append is a second entry, while an edge is
// a pointer and there is one place to point, so Upsert keys on (kind, target) and a
// repeat changes nothing.
//
// Ensures: it is pure; the ledger entry is appended whether or not link is set.
func record(
	raw string,
	e *ledger.Entry,
	edge related.Edge,
	link bool,
) (out string, linked bool, err error) {
	out, err = ledger.Append(raw, e)
	if err != nil {
		return "", false, fmt.Errorf("merge-status append: %w", err)
	}
	if !link {
		return out, false, nil
	}
	out, linked = related.Upsert(out, edge)
	return out, linked, nil
}

// linkEdge returns the edge --link should record, or the zero edge when --link was not
// given.
//
// The states that imply a supersession are the states that take --into, and
// internal/mergestatus derives that from its own vocabulary; this only turns the "no"
// into a usage error, because --link on a state that merges nothing is a
// misunderstanding of the flag rather than a condition to pass over in silence.
func (cfg *Config) linkEdge() (related.Edge, error) {
	if !cfg.Link {
		return related.Edge{}, nil
	}
	edge, ok := cfg.Entry.SupersededBy()
	if !ok {
		return related.Edge{}, root.Usagef(
			"merge-status append: --link needs a state that names a merged skill "+
				"with --into (%s); state %q does not",
			strings.Join(linkableStates(), ", "), cfg.Entry.State)
	}
	return edge, nil
}

// warnUnresolvedTarget reports that the composed target names no skill on disk.
//
// A warning rather than a failure, for the reason `link` warns: this command is handed a
// skill directory and infers the tree as its parent, so it cannot be certain which tree
// it is resolving against. The ledger entry is a record of what a run decided and stays
// true regardless; what the warning protects is the edge, which `index` would silently
// drop. A tree that cannot be walked is reported as unchecked rather than passed over.
func (cfg *Config) warnUnresolvedTarget(dir, target string) {
	tree := filepath.Dir(dir)
	if len(indexgen.MissingQualified(tree, []string{target})) > 0 {
		_, _ = fmt.Fprintf(cfg.Stdout,
			"merge-status append: warning: no skill %q under %s — index will drop this edge\n",
			target, filepath.Dir(tree))
	}
}

// linkableStates lists the states that imply a superseded-by edge, for a usage message.
//
// Derived from the schema rather than written out, so a state that later gains `into`
// cannot leave this message naming the old set. Sorted: map iteration is randomized.
func linkableStates() []string {
	var names []string
	for name, required := range ledger.States() {
		if slices.Contains(required, "into") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (cfg *Config) execCheck(_ context.Context, args []string) error {
	if len(args) != 1 {
		return root.Usagef("merge-status check: pass exactly one tree")
	}
	dirs, err := skill.Discover(args[0])
	if err != nil {
		return fmt.Errorf("merge-status check: %w", err)
	}
	failed := false
	checked := 0
	for _, dir := range dirs {
		s, err := skill.Load(dir)
		if err != nil {
			return fmt.Errorf("merge-status check: %w", err)
		}
		entries, err := ledger.Parse(s.Raw)
		if err != nil {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s: %v\n", filepath.Base(dir), err)
			failed = true
			continue
		}
		if len(entries) == 0 {
			continue // never evaluated in a merge run; the normal state
		}
		checked++
		for i := range entries {
			for _, p := range entries[i].Validate() {
				_, _ = fmt.Fprintf(cfg.Stdout, "%s: entry %d: %s\n",
					filepath.Base(dir), i+1, p)
				failed = true
			}
		}
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "checked %d ledger(s) across %d skill(s)\n",
		checked, len(dirs))
	if failed {
		return root.ExitError(1)
	}
	return nil
}

// stateNames lists the state vocabulary for a flag's help text.
//
// Sorted: map iteration is randomized, and help text that reordered itself between runs
// would make every diff of a captured --help unreadable.
func stateNames() []string {
	names := make([]string, 0, len(ledger.States()))
	for name := range ledger.States() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
