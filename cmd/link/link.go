// Package link implements `exegesis link`: it idempotently records a
// related-skill edge in a skill's `## Related skills` section, the format
// `exegesis index` reads back. Parsing, serialization, and idempotency live in
// internal/related; this command does the flag handling and file I/O.
package link

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/indexgen"
	"github.com/StevenACoffman/exegesis/internal/related"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/skill"
)

// Config holds the link command configuration.
type Config struct {
	*root.Config
	Kind      string
	To        string
	Rationale string
	Flags     *ff.FlagSet
	Command   *ff.Command
}

// New creates and registers the link command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("link").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Kind, 0, "kind", "",
		"edge kind: depends-on, contrasts-with, or composes-with")
	cfg.Flags.StringVar(&cfg.To, 0, "to", "", "target skill slug")
	cfg.Flags.StringVar(&cfg.Rationale, 0, "rationale", "", "one-line reason for the edge")
	cfg.Command = &ff.Command{
		Name:      "link",
		Usage:     "exegesis link --kind K --to SLUG --rationale WHY SKILL_DIR",
		ShortHelp: "record a related-skill edge in a skill's `## Related skills` section",
		LongHelp: "Append an edge to SKILL_DIR/SKILL.md's `## Related skills` section, creating\n" +
			"the section if absent. Idempotent by (kind, target): re-running with the same\n" +
			"kind and target updates the rationale in place instead of duplicating the edge.\n" +
			"`exegesis index` reads these edges back to build INDEX.md.",
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("link: need exactly one skill directory")
	}
	edge, err := cfg.edge()
	if err != nil {
		return err
	}
	s, err := skill.Load(args[0])
	if err != nil {
		return fmt.Errorf("link: %w", err)
	}
	cfg.warnUnknownTarget(s.Dir, edge.Target)
	out, changed := related.Upsert(s.Raw, edge)
	if !changed {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: unchanged (%s `%s`)\n", s.Name, edge.Kind, edge.Target)
		return nil
	}
	if err := atomicfile.WriteFile(s.Path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("link: write %s: %w", s.Path, err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s: linked %s `%s`\n", s.Name, edge.Kind, edge.Target)
	return nil
}

// edge builds and validates the edge from the flags. The target is normalized to
// a slug so it matches how `index` keys skills.
func (cfg *Config) edge() (related.Edge, error) {
	kind := related.Kind(cfg.Kind)
	if !kind.Valid() {
		return related.Edge{}, fmt.Errorf(
			"link: --kind must be depends-on, contrasts-with, or composes-with (got %q)", cfg.Kind)
	}
	if cfg.To == "" {
		return related.Edge{}, errors.New("link: --to is required")
	}
	if cfg.Rationale == "" {
		return related.Edge{}, errors.New("link: --rationale is required")
	}
	return related.Edge{Kind: kind, Target: skill.Slug(cfg.To), Rationale: cfg.Rationale}, nil
}

// warnUnknownTarget reports that target is not a skill alongside dir, which means
// `index` will silently drop the edge being recorded.
//
// It warns rather than failing because the tree is inferred as dir's parent: right for
// a skill in a book, not necessarily for a standalone one. The warning holds either
// way — it names the tree it checked, and that tree's INDEX.md is the one that drops
// the edge. `relate`, which is given the tree explicitly, treats the same condition as
// an error. A tree that cannot be walked is reported as unchecked rather than passed
// over in silence.
func (cfg *Config) warnUnknownTarget(dir, target string) {
	tree := filepath.Dir(dir)
	nodes, err := indexgen.CollectNodes(tree)
	if err != nil {
		_, _ = fmt.Fprintf(cfg.Stdout, "link: target %q not checked: %v\n", target, err)
		return
	}
	if len(related.UnknownSlugs(nodes, []string{target})) > 0 {
		_, _ = fmt.Fprintf(cfg.Stdout,
			"link: warning: no skill %q in %s — index will drop this edge\n", target, tree)
	}
}
