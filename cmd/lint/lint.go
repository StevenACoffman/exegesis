// Package lint implements the "lint" command: it validates agent-skill
// directories against the agentskills.io spec (a native reimplementation of
// skillscheck) and, opt-in, the book2skill Quality Red Lines — without an
// external process.
package lint

import (
	"context"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/skilllint"
)

const (
	formatText = "text"
	formatJSON = "json"
)

// Config holds the flags and wiring for the lint command.
type Config struct {
	*root.Config
	Check   string
	Agents  string
	Exclude string
	Format  string
	Strict  bool
	Fix     bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the lint command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("lint").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Check, 0, "check", "",
		"categories: spec,quality,disclosure,agents,redlines,all (default: all but redlines)")
	cfg.Flags.StringVar(&cfg.Agents, 0, "agents", "",
		"comma-separated agent adapters or 'all' (default: auto-detect)")
	cfg.Flags.StringVar(&cfg.Exclude, 0, "exclude", "",
		"comma-separated base-name globs to prune from walks (e.g. node_modules,dist)")
	cfg.Flags.StringVar(&cfg.Format, 0, "format", formatText, "output format: text or json")
	cfg.Flags.BoolVar(&cfg.Strict, 0, "strict", "treat warnings as errors (exit 1)")
	cfg.Flags.BoolVar(&cfg.Fix, 0, "fix", "apply safe mechanical fixes before reporting")
	cfg.Command = &ff.Command{
		Name:      "lint",
		Usage:     "exegesis lint [FLAGS] <dir>",
		ShortHelp: "validate agent skills (agentskills.io spec + optional Quality Red Lines)",
		LongHelp: `Validate every skill under <dir> against the agentskills.io
specification (spec, quality, disclosure, and per-agent checks). Add
--check redlines (or all) to also enforce the book2skill Quality Red Lines.
Exit code is 1 when errors are found, or when --strict and warnings are found.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return &book2skill.Error{
			Code:    book2skill.EINVALID,
			Message: "lint: a directory is required",
		}
	}
	if cfg.Format != formatText && cfg.Format != formatJSON && cfg.Format != "" {
		return &book2skill.Error{
			Code:    book2skill.EINVALID,
			Message: "lint: unknown --format " + cfg.Format,
		}
	}
	cats, err := parseCategories(cfg.Check)
	if err != nil {
		return err
	}
	opts := skilllint.Options{
		Categories: cats,
		AgentNames: splitCSV(cfg.Agents),
		Exclude:    splitCSV(cfg.Exclude),
	}

	if tokenChecks(cats) && !skilllint.ExactTokenizer() {
		_, _ = fmt.Fprintln(cfg.Stderr,
			"warning: exact cl100k_base tokenizer unavailable; token-budget checks are approximate")
	}

	result, fixes, err := cfg.lintOrFix(args[0], opts)
	if err != nil {
		return err
	}
	return cfg.report(result, fixes)
}

func (cfg *Config) lintOrFix(
	dir string,
	opts skilllint.Options,
) (*skilllint.Result, []string, error) {
	if cfg.Fix {
		result, fixes, err := skilllint.Fix(dir, opts.Categories)
		if err != nil {
			return nil, nil, fmt.Errorf("lint: %w", err)
		}
		return result, fixes, nil
	}
	result, err := skilllint.Run(dir, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("lint: %w", err)
	}
	return result, nil, nil
}

func (cfg *Config) report(result *skilllint.Result, fixes []string) error {
	if cfg.Format == formatJSON {
		if err := skilllint.WriteJSON(cfg.Stdout, result, fixes); err != nil {
			return fmt.Errorf("lint: %w", err)
		}
	} else {
		for _, f := range fixes {
			_, _ = fmt.Fprintln(cfg.Stdout, "fixed: "+f)
		}
		skilllint.WriteText(cfg.Stdout, result)
	}
	if code := result.ExitCode(cfg.Strict); code != 0 {
		return root.ExitError(code)
	}
	return nil
}

// parseCategories turns the --check flag into a category set. An empty flag uses
// the default set (everything but redlines); "all" enables every category.
func parseCategories(check string) (map[skilllint.Category]bool, error) {
	if strings.TrimSpace(check) == "" {
		return skilllint.DefaultCategories(), nil
	}
	cats := make(map[skilllint.Category]bool)
	for _, name := range splitCSV(check) {
		if name == "all" {
			return skilllint.AllCategories(), nil
		}
		switch cat := skilllint.Category(name); cat {
		case skilllint.CategoryRedlines, skilllint.CategorySpec, skilllint.CategoryQuality,
			skilllint.CategoryDisclosure, skilllint.CategoryAgents:
			cats[cat] = true
		default:
			return nil, &book2skill.Error{
				Code:    book2skill.EINVALID,
				Message: "lint: unknown --check category '" + name + "'",
			}
		}
	}
	return cats, nil
}

// tokenChecks reports whether any selected category performs token-budget checks.
func tokenChecks(cats map[skilllint.Category]bool) bool {
	return cats[skilllint.CategorySpec] || cats[skilllint.CategoryDisclosure]
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
