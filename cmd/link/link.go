// Package link implements the "link" command: it appends a Zettelkasten
// relationship bullet to a skill's `## Related Skills` section. It is used by
// book2skill Phase 3 (depends-on / contrasts-with / composes-with) and
// merge-skills Phase 3 (superseded-by). The append is idempotent.
package link

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const (
	skillFile = "SKILL.md"
	filePerm  = 0o644
)

// Config holds the flags and wiring for the link command.
type Config struct {
	*root.Config
	Kind      string
	To        string
	Rationale string
	Flags     *ff.FlagSet
	Command   *ff.Command
}

// New creates and registers the link command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("link").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Kind, 0, "kind", "",
		"relationship: depends-on|contrasts-with|composes-with|superseded-by (required)")
	cfg.Flags.StringVar(&cfg.To, 0, "to", "", "target skill slug (required)")
	cfg.Flags.StringVar(&cfg.Rationale, 0, "rationale", "", "short reason for the link")
	cfg.Command = &ff.Command{
		Name:      "link",
		Usage:     "exegesis link --kind <kind> --to <slug> [--rationale <text>] <skill-dir>",
		ShortHelp: "append a relationship to a skill's `## Related Skills` section",
		LongHelp: `Append a "- <kind>: ` + "`<slug>`" + ` — <rationale>" bullet to
<skill-dir>/SKILL.md's ## Related Skills section (created if absent). Idempotent:
re-linking the same kind and target is a no-op. Flags come before the directory.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return einval("link: a skill directory is required")
	}
	kind := book2skill.RelationshipKind(cfg.Kind)
	if !kind.Valid() {
		return einval("link: unknown --kind '" + cfg.Kind + "'")
	}
	if cfg.To == "" {
		return einval("link: --to <slug> is required")
	}
	path := filepath.Join(args[0], skillFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return einval("link: cannot read " + path)
	}
	out, changed := book2skill.AppendRelated(string(data), book2skill.Relationship{
		Kind: kind, To: cfg.To, Rationale: cfg.Rationale,
	})
	if !changed {
		_, _ = fmt.Fprintf(cfg.Stdout, "already present: %s -> %s in %s\n", cfg.Kind, cfg.To, path)
		return nil
	}
	if err := os.WriteFile(path, []byte(out), filePerm); err != nil {
		return &book2skill.Error{Op: "link.write", Err: err}
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "linked: %s -> %s in %s\n", cfg.Kind, cfg.To, path)
	return nil
}

func einval(msg string) error {
	return &book2skill.Error{Code: book2skill.EINVALID, Message: msg}
}
