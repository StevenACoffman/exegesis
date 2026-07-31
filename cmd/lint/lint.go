// Package lint implements the "lint" command: validate one or more skills
// against the agentskills.io spec, body red-lines, and runtime-neutrality.
package lint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	lintlib "github.com/StevenACoffman/exegesis/internal/lint"
	"github.com/StevenACoffman/exegesis/internal/registry"
	"github.com/StevenACoffman/exegesis/internal/skill"
)

// Config holds the lint command configuration.
type Config struct {
	*root.Config
	JSON         bool
	Registry     string
	MaxBodyWords int
	MaxDescWords int
	Flags        *ff.FlagSet
	Command      *ff.Command
}

// result is one skill's findings, used for JSON output.
type result struct {
	Skill    string            `json:"skill"`
	Findings []lintlib.Finding `json:"findings"`
}

// New creates and registers the lint command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("lint").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.JSON, 0, "json", "emit findings as JSON")
	cfg.Flags.StringVar(&cfg.Registry, 0, "registry", "",
		"optional registry JSON supplying word budgets and required sections")
	cfg.Flags.IntVar(&cfg.MaxBodyWords, 0, "max-body-words", 0,
		"fail if the body exceeds this many words (0 = unlimited; overrides registry)")
	cfg.Flags.IntVar(&cfg.MaxDescWords, 0, "max-desc-words", 0,
		"fail if the description exceeds this many words (0 = unlimited; overrides registry)")
	cfg.Command = &ff.Command{
		Name:      "lint",
		Usage:     "exegesis lint [--json] [--registry PATH] [--max-body-words N] SKILL_DIR ...",
		ShortHelp: "validate a skill's frontmatter, body links, and runtime-neutrality",
		LongHelp: `Validate each SKILL_DIR against the agentskills.io spec plus book2skill's
body red-lines and the runtime-neutrality gate. With --registry (or the
--max-*-words flags) it also enforces per-skill word budgets and required
sections. Exits non-zero if any skill has an error-severity finding.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("lint: need at least one skill directory")
	}
	opts, err := cfg.options()
	if err != nil {
		return err
	}
	results := make([]result, 0, len(args))
	for _, dir := range args {
		s, loadErr := skill.Load(dir)
		if loadErr != nil {
			return fmt.Errorf("lint: %w", loadErr)
		}
		results = append(results, result{Skill: s.Name, Findings: lintlib.Check(s, opts)})
	}
	if err := cfg.render(results); err != nil {
		return err
	}
	if anyError(results) {
		return root.ExitError(1)
	}
	return nil
}

// options builds the lint Options from the optional registry, then applies any
// positive scalar flag as an override.
func (cfg *Config) options() (lintlib.Options, error) {
	var opts lintlib.Options
	if cfg.Registry != "" {
		r, err := registry.Load(cfg.Registry)
		if err != nil {
			return opts, fmt.Errorf("lint: %w", err)
		}
		opts = lintlib.Options{
			MaxBodyWords:        r.MaxBodyWords,
			MaxDescriptionWords: r.MaxDescriptionWords,
			RequiredSections:    r.RequiredSections,
		}
	}
	if cfg.MaxBodyWords > 0 {
		opts.MaxBodyWords = cfg.MaxBodyWords
	}
	if cfg.MaxDescWords > 0 {
		opts.MaxDescriptionWords = cfg.MaxDescWords
	}
	return opts, nil
}

func (cfg *Config) render(results []result) error {
	if cfg.JSON {
		b, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("lint: %w", err)
		}
		_, _ = fmt.Fprintln(cfg.Stdout, string(b))
		return nil
	}
	for _, r := range results {
		if len(r.Findings) == 0 {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s: ok\n", r.Skill)
			continue
		}
		for _, f := range r.Findings {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s: %s: %s\n", r.Skill, f.Severity, f.Message)
		}
	}
	return nil
}

func anyError(results []result) bool {
	for _, r := range results {
		for _, f := range r.Findings {
			if f.Severity == lintlib.Error {
				return true
			}
		}
	}
	return false
}
