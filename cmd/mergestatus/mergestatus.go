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
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	ledger "github.com/StevenACoffman/exegesis/internal/mergestatus"
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

It is an audit trail, so appending is the only write this command offers. An entry
already on disk is never re-rendered -- a new one is spliced in ahead of the closing
fence -- so an append cannot reformat, reorder or lose an earlier run's record.

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
	fs.BoolVar(&cfg.Link, 0, "link", "also write the superseded-by bullet (not yet available)")
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
	if cfg.Link {
		return root.Usagef("merge-status append: --link is not available yet; it writes a " +
			"superseded-by edge, and that edge kind is still an open decision. Append " +
			"without it, then record the edge with \"exegesis link\" once it lands")
	}
	if len(args) != 1 {
		return root.Usagef("merge-status append: pass exactly one skill directory")
	}
	if problems := cfg.Entry.Validate(); len(problems) > 0 {
		for _, p := range problems {
			_, _ = fmt.Fprintf(cfg.Stdout, "  - %s\n", p)
		}
		return root.ExitError(1)
	}
	dir := args[0]
	path := filepath.Join(dir, skill.FileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("merge-status append: read %s: %w", path, err)
	}
	out, err := ledger.Append(string(raw), &cfg.Entry)
	if err != nil {
		return fmt.Errorf("merge-status append: %w", err)
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("merge-status append: write %s: %w", path, err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s: recorded %s for run %s\n",
		filepath.Base(dir), cfg.Entry.State, cfg.Entry.Run)
	return nil
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
