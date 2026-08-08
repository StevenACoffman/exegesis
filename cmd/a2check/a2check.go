// Package a2check implements `exegesis a2check`: the A2-sharpness gate. It reports the
// language signals a merged skill states that none of its sources do, and fails when
// there are too few to call the merge sharper than what it replaced. The counting is
// pure and shared (internal/a2check); this command reads the files and renders the
// report.
package a2check

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	checker "github.com/StevenACoffman/exegesis/internal/a2check"
	"github.com/StevenACoffman/skillet/skill"
)

// Config holds the a2check command configuration.
type Config struct {
	*root.Config
	SourceSkill string
	Strict      bool
	Flags       *ff.FlagSet
	Command     *ff.Command
}

// New creates and registers the a2check command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("a2check").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.SourceSkill, 0, "source-skill", "",
		"comma-separated source skill directories the merged skill was built from")
	cfg.Flags.BoolVar(&cfg.Strict, 0, "strict",
		"exit non-zero when the merged skill adds too few signals")
	cfg.Command = &ff.Command{
		Name:      "a2check",
		Usage:     "exegesis a2check --source-skill A,B [--strict] MERGED_SKILL_DIR",
		ShortHelp: "report the language signals a merged skill adds to its sources'",
		LongHelp: `Read the "` + checker.Segment + `" segment of MERGED_SKILL_DIR and of every --source-skill, and
report the language signals the merged skill states that no source does. A merged skill
is worth having when it reaches situations its sources did not, and ` + checker.Segment + ` is where that
claim is written down; one that only restates its sources' signals adds a third skill
competing for the same triggers.

A signal is a double-quoted phrase in ` + checker.Segment + `. That is the shape they take under a
"### Language Signals" heading in some skills and a bold label in others -- and two
thirds of the merged skills in this corpus have neither, so matching the quotation
rather than the subsection is what makes the check work at all.

Advisory by default: it prints the count and the new signals and exits 0. With --strict
it exits non-zero when too few signals are new.

The counting is structural. Two signals worded differently that mean the same thing are
two here and one to a reader, so a passing count is evidence and not a verdict.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) != 1 {
		return root.Usagef("a2check: need exactly one merged skill directory")
	}
	sources, err := cfg.sourceSignals()
	if err != nil {
		return err
	}
	merged, err := signalsOf(args[0])
	if err != nil {
		return err
	}
	added := checker.New(merged, sources)
	cfg.report(filepath.Base(args[0]), merged, added)
	if cfg.Strict && len(added) < checker.MinNew {
		return root.ExitError(1)
	}
	return nil
}

// sourceSignals reads every --source-skill's signals. A missing source is fatal rather
// than skipped: dropping one silently would make its signals look new.
func (cfg *Config) sourceSignals() ([][]string, error) {
	var out [][]string
	for _, dir := range strings.Split(cfg.SourceSkill, ",") {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue // tolerate stray/trailing commas
		}
		signals, err := signalsOf(dir)
		if err != nil {
			return nil, err
		}
		out = append(out, signals)
	}
	if len(out) == 0 {
		return nil, root.Usagef("a2check: --source-skill names no skill directory")
	}
	return out, nil
}

// signalsOf loads one skill and returns the language signals its A2 segment states.
func signalsOf(dir string) ([]string, error) {
	s, err := skill.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("a2check: %w", err)
	}
	return checker.Signals(s.Body), nil
}

// report prints the tally and the new signals themselves, because a bare count tells a
// reader nothing about whether the two it found are really distinct.
func (cfg *Config) report(name string, merged, added []string) {
	if len(merged) == 0 {
		// Measured on the real merged tree: 5 of 24 skills state no quoted signal at
		// all. Reporting those as "0 of 0 are new" reads as a sharpness failure when
		// the finding is that there is nothing to be sharp with — a different defect,
		// in a different place, fixed by a different edit.
		_, _ = fmt.Fprintf(cfg.Stdout,
			"%s: states no %s language signals at all — nothing to compare\n",
			name, checker.Segment)
		return
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s: %d of %d %s signal(s) are new (want at least %d)\n",
		name, len(added), len(merged), checker.Segment, checker.MinNew)
	for _, signal := range added {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: NEW %q\n", name, signal)
	}
}
