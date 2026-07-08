// Package a2check implements the "a2check" command: it measures the sharpness of
// a merged skill's A2 trigger against its source skills — how many of its
// language signals neither source has. This is the structural half of
// merge-skills Red Line #3; whether the unique signals are genuinely distinct
// stays the agent's judgment, so it is advisory by default.
package a2check

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const skillFile = "SKILL.md"

// Config holds the flags and wiring for the a2check command.
type Config struct {
	*root.Config
	Source  string
	Strict  bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the a2check command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("a2check").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Source, 0, "source-skill", "",
		"comma-separated source skill directories the merged skill was built from")
	cfg.Flags.BoolVar(&cfg.Strict, 0, "strict", "exit 1 (not just warn) when the gate is not met")
	cfg.Command = &ff.Command{
		Name:      "a2check",
		Usage:     "exegesis a2check --source-skill <srcA-dir>,<srcB-dir> <merged-skill-dir>",
		ShortHelp: "measure a merged skill's A2 sharpness against its source skills",
		LongHelp: `Report the language signals in the merged skill's A2 that neither
source skill has. At least 2 unique signals satisfies the structural half of
merge-skills Red Line #3. Advisory by default (exit 0, prints WARN when under the
floor — semantic distinctness is your call); --strict exits 1. Flags come before
the directory.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return einval("a2check: a merged skill directory is required")
	}
	sources := splitCSV(cfg.Source)
	if len(sources) == 0 {
		return einval("a2check: at least one --source-skill directory is required")
	}
	merged, err := readSkill(args[0])
	if err != nil {
		return err
	}
	bodies := make([]string, 0, len(sources))
	for _, s := range sources {
		body, err := readSkill(s)
		if err != nil {
			return err
		}
		bodies = append(bodies, body)
	}
	return cfg.report(book2skill.A2Sharpness(merged, bodies))
}

func (cfg *Config) report(unique []string) error {
	_, _ = fmt.Fprintln(cfg.Stdout, strconv.Itoa(len(unique))+
		" unique A2 language signal(s) vs sources:")
	for _, s := range unique {
		_, _ = fmt.Fprintln(cfg.Stdout, "  - "+s)
	}
	if len(unique) >= book2skill.MinSharpSignals {
		_, _ = fmt.Fprintln(cfg.Stdout, "OK: A2 is structurally sharper than both sources")
		return nil
	}
	_, _ = fmt.Fprintf(cfg.Stdout,
		"WARN: only %d unique signal(s); Red Line #3 wants ≥%d — re-evaluate V4 or dissolve\n",
		len(unique), book2skill.MinSharpSignals)
	if cfg.Strict {
		return root.ExitError(1)
	}
	return nil
}

func readSkill(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, skillFile))
	if err != nil {
		return "", einval("a2check: cannot read " + filepath.Join(dir, skillFile))
	}
	return string(data), nil
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
