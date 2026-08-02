// Package tests implements the "tests" command: check a skill's
// test-prompts.json composition, or scaffold a starter set with derived checks.
package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/skillet/testprompts"
)

// Config holds the tests command configuration.
type Config struct {
	*root.Config
	Scaffold bool
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
	cfg.Command = &ff.Command{
		Name:      "tests",
		Usage:     "exegesis tests [--scaffold] SKILL_DIR ...",
		ShortHelp: "check a skill's test-prompts.json composition; --scaffold writes a starter",
		LongHelp: `Without --scaffold: load each SKILL_DIR/test-prompts.json and enforce the
composition gate (>=3 should_trigger, >=2 should_not_trigger, >=1 edge_case).
Exits non-zero if any set fails.

With --scaffold: write a starter SKILL_DIR/test-prompts.json whose cases carry a
"checks" array seeded from each case's "expected" text, ready for skillsaw's
judge. Refuses to overwrite an existing file.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("tests: need at least one skill directory")
	}
	if cfg.Scaffold {
		return cfg.scaffold(args)
	}
	return cfg.gate(args)
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

func (cfg *Config) gate(dirs []string) error {
	failed := false
	for _, dir := range dirs {
		path := filepath.Join(dir, "test-prompts.json")
		f, err := testprompts.Load(path)
		if err != nil {
			return fmt.Errorf("tests: %w", err)
		}
		c := f.Tally()
		problems := f.Validate()
		_, _ = fmt.Fprintf(
			cfg.Stdout,
			"%s: %d should_trigger, %d should_not_trigger, %d edge_case\n",
			filepath.Base(dir),
			c.Trigger,
			c.Decoy,
			c.Edge,
		)
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
