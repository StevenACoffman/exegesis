// Package normalize implements `exegesis normalize`: it rewrites every skill's
// `## Related skills` section into the canonical bullet format that `link` and
// `relate` write, leaving anything it cannot parse untouched. The rewrite is pure
// (internal/related); this command discovers the skills and reads/writes files.
package normalize

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/related"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/skill"
)

// Config holds the normalize command configuration.
type Config struct {
	*root.Config
	Check   bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the normalize command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("normalize").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Check, 0, "check",
		"report which skills are not canonical without writing (exit 1 if any)")
	cfg.Command = &ff.Command{
		Name:      "normalize",
		Usage:     "exegesis normalize [--check] [TREE]",
		ShortHelp: "rewrite every skill's `## Related skills` section in the canonical format",
		LongHelp: `Rewrite each skill's ` + "`## Related skills`" + ` section under TREE (default ".")
into the one format ` + "`link`" + ` and ` + "`relate`" + ` write: a canonical
"- kind: ` + "`target`" + ` — rationale" bullet per relationship, under an exact
` + "`## Related skills`" + ` heading.

Only bullets that name a skill are rewritten. A bullet whose target is prose, an
introductory sentence, fenced code, and everything outside the section are left
byte-identical, so normalizing cannot discard content it did not understand. A bullet
naming several targets becomes one bullet per target, and a relationship stated twice
collapses to one.

Normalizing does not change which edges a skill declares — only how they are written —
so INDEX.md is unaffected. With --check, report the skills that would change and exit 1
instead of writing.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	tree := "."
	switch len(args) {
	case 0:
	case 1:
		tree = args[0]
	default:
		return root.Usagef("normalize: expected at most one tree path")
	}
	dirs, err := skill.Discover(tree)
	if err != nil {
		return fmt.Errorf("normalize: %w", err)
	}
	changed := 0
	for _, dir := range dirs {
		did, oneErr := cfg.normalizeOne(dir)
		if oneErr != nil {
			return oneErr
		}
		if did {
			changed++
		}
	}
	return cfg.report(changed, len(dirs))
}

// normalizeOne rewrites one skill's related section, reporting whether it changed.
// Under --check nothing is written.
func (cfg *Config) normalizeOne(dir string) (bool, error) {
	s, err := skill.Load(dir)
	if err != nil {
		return false, fmt.Errorf("normalize: %w", err)
	}
	out, changed := related.Normalize(s.Raw)
	if !changed {
		return false, nil
	}
	verb := "would rewrite"
	if !cfg.Check {
		if writeErr := atomicfile.WriteFile(s.Path, []byte(out), 0o644); writeErr != nil {
			return false, fmt.Errorf("normalize: write %s: %w", s.Path, writeErr)
		}
		verb = "rewrote"
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s: %s `## Related skills`\n", s.Name, verb)
	return true, nil
}

// report writes the summary line and, under --check, signals staleness with a
// non-zero exit so the command can gate CI.
func (cfg *Config) report(changed, total int) error {
	if cfg.Check {
		if changed == 0 {
			_, _ = fmt.Fprintf(cfg.Stdout, "all %d skill(s) already canonical\n", total)
			return nil
		}
		_, _ = fmt.Fprintf(cfg.Stdout,
			"%d of %d skill(s) are not canonical (run: exegesis normalize)\n", changed, total)
		return root.ExitError(1)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "normalized %d of %d skill(s)\n", changed, total)
	return nil
}
