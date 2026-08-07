// Package tests implements the "tests" command: check a skill's
// test-prompts.json composition, or scaffold a starter set with derived checks.
package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/skillet/testprompts"
)

// preferMerged is the case category unique to a merged skill: a prompt where the merged
// skill must be chosen over either source skill it came from. It is the whole point of
// gating a merged set differently, and skillet does not know it exists -- exegesis owns
// this policy and passes it in.
const preferMerged = "prefer_merged_over_source"

// mergedEdgeMinimum is the edge_case floor for a merged skill.
//
// Two, against the standard one: a merged skill inherits the boundaries of both parents,
// so the places it can go wrong are exactly the places one parent's rule meets the
// other's. One edge case cannot cover that.
const mergedEdgeMinimum = 2

// Config holds the tests command configuration.
type Config struct {
	*root.Config
	Scaffold bool
	Merge    bool
	Migrate  bool
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// New creates and registers the tests command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("tests").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Scaffold, 0, "scaffold",
		"write a starter test-prompts.json (with checks derived from expected) instead of checking")
	cfg.Flags.BoolVar(&cfg.Merge, 0, "merge",
		"gate against the merged-skill composition, which adds "+preferMerged)
	cfg.Flags.BoolVar(&cfg.Migrate, 0, "migrate",
		"rewrite each test-prompts.json into canonical form, reporting every change")
	cfg.Command = &ff.Command{
		Name:      "tests",
		Usage:     "exegesis tests [--scaffold|--merge|--migrate] SKILL_DIR ...",
		ShortHelp: "check a skill's test-prompts.json composition, scaffold one, or migrate one",
		LongHelp: `Load each SKILL_DIR/test-prompts.json and enforce the composition gate
(>=3 should_trigger, >=2 should_not_trigger, >=1 edge_case). Exits non-zero if any
set fails.

With --merge: gate against the merged-skill composition instead -- the same three
categories with the edge_case floor raised to 2, plus >=2 ` + preferMerged + `,
prompts where the merged skill must be chosen over either source skill it came
from. That last category is the quality gate unique to a merged skill, and the
plain gate rejects it as an unknown case type, so a merged set must be checked
with --merge or not at all.

With --scaffold: write a starter SKILL_DIR/test-prompts.json whose cases carry a
"checks" array seeded from each case's "expected" text, ready for skillsaw's
judge. Refuses to overwrite an existing file.

With --migrate: rewrite each file into canonical form and report every change --
a bare top-level array or a legacy "test_cases" key becomes "tests", legacy
"expected_behavior" becomes "expected", and non-numeric ids are renumbered. A file
already canonical is left untouched. A file carrying both "tests" and "test_cases"
is refused rather than migrated: the reader keeps only "tests", so rewriting it
would delete cases that are still on disk.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return root.Usagef("tests: need at least one skill directory")
	}
	switch {
	case cfg.Scaffold && cfg.Migrate:
		return root.Usagef("tests: --scaffold and --migrate both write; pick one")
	case cfg.Scaffold:
		return cfg.scaffold(args)
	case cfg.Migrate:
		return cfg.migrate(args)
	}
	return cfg.gate(args, cfg.composition())
}

func (cfg *Config) scaffold(dirs []string) error {
	for _, dir := range dirs {
		path := filepath.Join(dir, "test-prompts.json")
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("tests: %s already exists; refusing to overwrite", path)
		}
		f := testprompts.Scaffold(filepath.Base(dir))
		if err := testprompts.Write(path, f); err != nil {
			return fmt.Errorf("tests: %w", err)
		}
		_, _ = fmt.Fprintf(cfg.Stdout, "scaffolded %s\n", path)
	}
	return nil
}

// composition is the case mix to gate against: the merged-skill mix under --merge,
// otherwise skillet's standard one.
//
// The merged mix is assembled here rather than in skillet because merging is exegesis's
// workflow; a shared package that knew about ` + preferMerged + ` would be carrying one
// consumer's vocabulary for everyone.
func (cfg *Config) composition() testprompts.Composition {
	want := testprompts.Standard()
	if cfg.Merge {
		want[testprompts.TypeEdgeCase] = mergedEdgeMinimum
		want[preferMerged] = 2
	}
	return want
}

// displayOrder lists a composition's categories for reporting: the three standard ones
// first, in the order a set is built rather than alphabetically, then anything else.
//
// Alphabetical order alone would silently reshuffle every existing report -- edge_case
// would lead -- losing the trigger, decoy, edge progression that makes a tally readable
// at a glance. Categories a caller adds go at the end, sorted, so the order is total and
// stable whatever vocabulary is in play.
func displayOrder(want testprompts.Composition) []string {
	standard := []string{
		testprompts.TypeShouldTrigger,
		testprompts.TypeShouldNotTrigger,
		testprompts.TypeEdgeCase,
	}
	out := make([]string, 0, len(want))
	seen := make(map[string]bool, len(want))
	for _, caseType := range standard {
		if _, ok := want[caseType]; ok {
			out = append(out, caseType)
			seen[caseType] = true
		}
	}
	extra := make([]string, 0, len(want))
	for caseType := range want {
		if !seen[caseType] {
			extra = append(extra, caseType)
		}
	}
	slices.Sort(extra)
	return append(out, extra...)
}

// gate reports each set's composition against want and fails if any set falls short.
func (cfg *Config) gate(dirs []string, want testprompts.Composition) error {
	failed := false
	for _, dir := range dirs {
		path := filepath.Join(dir, "test-prompts.json")
		f, err := testprompts.Load(path)
		if err != nil {
			return fmt.Errorf("tests: %w", err)
		}
		// Counted from the composition rather than from a fixed list of three, so a
		// gate that gains a category cannot leave the tally behind reporting the old one.
		counts := make([]string, 0, len(want))
		for _, caseType := range displayOrder(want) {
			counts = append(counts, fmt.Sprintf("%d %s", f.CountOf(caseType), caseType))
		}
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: %s\n",
			filepath.Base(dir), strings.Join(counts, ", "))
		problems := f.ValidateAgainst(want)
		for _, p := range problems {
			_, _ = fmt.Fprintf(cfg.Stdout, "  - %s\n", p)
		}
		if len(problems) > 0 {
			failed = true
		}
	}
	if failed {
		return root.ExitError(1)
	}
	return nil
}

// migrate rewrites each set into canonical form, reporting every change it makes.
func (cfg *Config) migrate(dirs []string) error {
	for _, dir := range dirs {
		path := filepath.Join(dir, "test-prompts.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("tests: read %s: %w", path, err)
		}
		if err := refuseIfCasesWouldBeLost(path, raw); err != nil {
			return err
		}
		f, err := testprompts.Parse(raw)
		if err != nil {
			return fmt.Errorf("tests: parse %s: %w", path, err)
		}
		if len(f.Rewrites) == 0 {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s: already canonical\n", path)
			continue
		}
		for _, r := range f.Rewrites {
			_, _ = fmt.Fprintf(cfg.Stdout, "  - %s\n", r)
		}
		if err := testprompts.Write(path, f); err != nil {
			return fmt.Errorf("tests: %w", err)
		}
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: migrated (%d changes)\n", path, len(f.Rewrites))
	}
	return nil
}

// refuseIfCasesWouldBeLost rejects a file carrying both "tests" and "test_cases".
//
// The reader keeps "tests" and drops the rest, so migrating such a file writes back only
// half of it and deletes cases the author can still see on disk. That is the one rewrite
// which destroys work rather than reshaping it, so it is refused rather than reported.
// The two keys are re-read here rather than matched out of File.Rewrites: pattern-matching
// a human-readable string to decide whether to destroy data would break the first time
// that wording changed.
func refuseIfCasesWouldBeLost(path string, raw []byte) error {
	var both struct {
		Tests     []json.RawMessage `json:"tests"`
		TestCases []json.RawMessage `json:"test_cases"`
	}
	// A decode failure needs no handling here: it leaves both fields empty, so there is
	// no pair of keys to weigh, and Parse reports the real problem a moment later.
	_ = json.Unmarshal(raw, &both)
	if len(both.Tests) > 0 && len(both.TestCases) > 0 {
		return fmt.Errorf(
			`tests: %s has both "tests" (%d cases) and "test_cases" (%d cases); `+
				`migrating would keep only "tests" and delete the rest -- merge them by hand first`,
			path, len(both.Tests), len(both.TestCases))
	}
	return nil
}
