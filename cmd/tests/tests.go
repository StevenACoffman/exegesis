// Package tests implements the "tests" command: it validates and normalizes a
// skill's test-prompts.json against the structural Phase-4 gate, scaffolds a
// template, and prints the darwin-skill handoff. Runtime trigger scoring is
// delegated to darwin-skill, which consumes the emitted test-prompts.json.
package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const (
	formatText      = "text"
	formatJSON      = "json"
	testPromptsFile = "test-prompts.json"
	filePerm        = 0o644
)

// Config holds the flags and wiring for the tests command.
type Config struct {
	*root.Config
	Format   string
	Scaffold bool
	Fix      bool
	Merge    bool
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// New creates and registers the tests command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("tests").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Format, 0, "format", formatText, "output format: text or json")
	cfg.Flags.BoolVar(&cfg.Scaffold, 0, "scaffold",
		"write a template test-prompts.json when none exists")
	cfg.Flags.BoolVar(&cfg.Fix, 0, "fix", "rewrite test-prompts.json in canonical form")
	cfg.Flags.BoolVar(&cfg.Merge, 0, "merge",
		"use the merge-skills gate (adds prefer_merged_over_source; edge_case≥2)")
	cfg.Command = &ff.Command{
		Name:      "tests",
		Usage:     "exegesis tests [FLAGS] <skill-dir>",
		ShortHelp: "validate, normalize, or scaffold a skill's test-prompts.json",
		LongHelp: `Validate <skill-dir>/test-prompts.json against the structural
Phase-4 gate (at least 3 should_trigger, 2 should_not_trigger, and 1 edge_case),
then print the darwin-skill handoff. --scaffold writes a template when none
exists; --fix rewrites an existing file in canonical form. --merge applies the
merge-skills gate (also requires prefer_merged_over_source and edge_case≥2).
Exit code is 1 when the gate fails.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return einval("tests: a skill directory is required")
	}
	if cfg.Format != formatText && cfg.Format != formatJSON && cfg.Format != "" {
		return einval("tests: unknown --format " + cfg.Format)
	}
	path := filepath.Join(args[0], testPromptsFile)
	if cfg.Scaffold {
		return cfg.scaffold(path)
	}
	return cfg.validate(args[0], path)
}

func (cfg *Config) scaffold(path string) error {
	if _, err := os.Stat(path); err == nil {
		return einval("tests: " + path + " already exists")
	}
	template := book2skill.TemplateTestCases()
	if cfg.Merge {
		template = book2skill.TemplateMergedTestCases()
	}
	if err := writeCanonical(path, template); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(cfg.Stdout, "scaffolded "+path)
	return nil
}

func (cfg *Config) validate(dir, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return einval("tests: cannot read " + path + " (use --scaffold to create it)")
	}
	cases, err := book2skill.DecodeTestPrompts(raw)
	if err != nil {
		return fmt.Errorf("tests: %w", err)
	}
	if cfg.Fix {
		if err := writeCanonical(path, cases); err != nil {
			return err
		}
	}
	problems := book2skill.ValidateTestSet(cases)
	if cfg.Merge {
		problems = book2skill.ValidateMergedTestSet(cases)
	}
	if err := cfg.report(dir, cases, problems); err != nil {
		return err
	}
	if len(problems) > 0 {
		return root.ExitError(1)
	}
	return nil
}

func (cfg *Config) report(dir string, cases []book2skill.TestCase, problems []string) error {
	counts := book2skill.CountByType(cases)
	if cfg.Format == formatJSON {
		return cfg.reportJSON(dir, cases, counts, problems)
	}
	cfg.reportText(dir, cases, counts, problems)
	return nil
}

func (cfg *Config) reportText(
	dir string,
	cases []book2skill.TestCase,
	counts map[book2skill.TestType]int,
	problems []string,
) {
	out := cfg.Stdout
	_, _ = fmt.Fprintf(
		out,
		"%s: %d test cases (%d should_trigger, %d should_not_trigger, %d edge_case)\n",
		dir,
		len(cases),
		counts[book2skill.ShouldTrigger],
		counts[book2skill.ShouldNotTrigger],
		counts[book2skill.EdgeCase],
	)
	if cfg.Merge {
		_, _ = fmt.Fprintf(out, "  + %d prefer_merged_over_source\n",
			counts[book2skill.PreferMergedOverSource])
	}
	if len(problems) == 0 {
		_, _ = fmt.Fprintln(out, "gate: PASS")
	} else {
		_, _ = fmt.Fprintln(out, "gate: FAIL")
		for _, p := range problems {
			_, _ = fmt.Fprintln(out, "  - "+p)
		}
	}
	_, _ = fmt.Fprintf(out, "darwin: run `darwin evolve %s/` to score and evolve this skill\n", dir)
}

func (cfg *Config) reportJSON(
	dir string,
	cases []book2skill.TestCase,
	counts map[book2skill.TestType]int,
	problems []string,
) error {
	report := struct {
		Skill    string   `json:"skill"`
		Total    int      `json:"total"`
		Trigger  int      `json:"should_trigger"`
		Decoy    int      `json:"should_not_trigger"`
		Edge     int      `json:"edge_case"`
		Pass     bool     `json:"pass"`
		Problems []string `json:"problems,omitempty"`
	}{
		Skill:    dir,
		Total:    len(cases),
		Trigger:  counts[book2skill.ShouldTrigger],
		Decoy:    counts[book2skill.ShouldNotTrigger],
		Edge:     counts[book2skill.EdgeCase],
		Pass:     len(problems) == 0,
		Problems: problems,
	}
	if err := json.NewEncoder(cfg.Stdout).Encode(report); err != nil {
		return fmt.Errorf("tests: %w", err)
	}
	return nil
}

// writeCanonical encodes cases as darwin-shaped JSON and writes them to path.
func writeCanonical(path string, cases []book2skill.TestCase) error {
	data, err := book2skill.EncodeTestPrompts(cases)
	if err != nil {
		return fmt.Errorf("tests: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), filePerm); err != nil {
		return &book2skill.Error{Op: "tests.writeCanonical", Err: err}
	}
	return nil
}

func einval(msg string) error {
	return &book2skill.Error{Code: book2skill.EINVALID, Message: msg}
}
