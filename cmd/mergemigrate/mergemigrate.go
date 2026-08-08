// Package mergemigrate implements `exegesis merge-migrate`: it moves a merged skill's
// provenance out of frontmatter and into a body `## Provenance` section, so a merged
// tree passes `exegesis lint` and records its composition where the spec says it goes.
// The rewrite is pure (internal/mergemigrate); this command discovers the skills and
// reads/writes files.
package mergemigrate

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	migrate "github.com/StevenACoffman/exegesis/internal/mergemigrate"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/skill"
)

// Config holds the merge-migrate command configuration.
type Config struct {
	*root.Config
	Check   bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the merge-migrate command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("merge-migrate").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Check, 0, "check",
		"report which skills would be migrated without writing (exit 1 if any)")
	cfg.Command = &ff.Command{
		Name:      "merge-migrate",
		Usage:     "exegesis merge-migrate [--check] [MERGED_TREE]",
		ShortHelp: "move a merged skill's provenance from frontmatter into the body",
		LongHelp: `Rewrite each merged skill under MERGED_TREE (default ".") into the provenance model
merge-skills specifies: spec-allowed frontmatter only, and the composition recorded in
a body "` + migrate.Heading + `" section as prose plus a fenced yaml block a generator reads.

Moved out of the frontmatter: id, title, type, source_skills, related_skills. Those are
not spec-allowed keys, which is why a merged tree fails "exegesis lint" on every skill.
"name" is set from the folder, and description, tags and any spec key are copied through
line for line, so a description's own wrapping and quoting are not reformatted.

Nothing is restated twice. A "supersedes" relation names exactly the skills already
listed in source_skills, so it is dropped rather than written again as an edge; a
"composes-with" relation becomes an ordinary related-skill bullet. The one key whose
removal would lose content is title: where a body has no heading of its own, the title
becomes that heading instead of being deleted.

A skill carrying none of the moved keys is left untouched, so running this over a mixed
tree rewrites only what needs it, and running it twice changes nothing the second time.
With --check, report the skills that would change and exit 1 instead of writing.`,
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
		return root.Usagef("merge-migrate: expected at most one tree path")
	}
	dirs, err := skill.Discover(tree)
	if err != nil {
		return fmt.Errorf("merge-migrate: %w", err)
	}
	changed := 0
	for _, dir := range dirs {
		did, oneErr := cfg.migrateOne(dir)
		if oneErr != nil {
			return oneErr
		}
		if did {
			changed++
		}
	}
	return cfg.report(changed, len(dirs))
}

// migrateOne rewrites one skill, reporting whether it changed. Under --check nothing
// is written.
func (cfg *Config) migrateOne(dir string) (bool, error) {
	s, err := skill.Load(dir)
	if err != nil {
		return false, fmt.Errorf("merge-migrate: %w", err)
	}
	folder := skill.Slug(filepath.Base(dir))
	out, changed, err := migrate.Migrate(s.Raw, folder)
	if err != nil {
		return false, fmt.Errorf("merge-migrate: %s: %w", dir, err)
	}
	if !changed {
		return false, nil
	}
	verb := "would migrate"
	if !cfg.Check {
		if writeErr := atomicfile.WriteFile(s.Path, []byte(out), 0o644); writeErr != nil {
			return false, fmt.Errorf("merge-migrate: write %s: %w", s.Path, writeErr)
		}
		verb = "migrated"
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s: %s provenance into the body\n", folder, verb)
	return true, nil
}

// report writes the summary line and, under --check, signals a stale tree with a
// non-zero exit so the command can gate CI.
func (cfg *Config) report(changed, total int) error {
	if cfg.Check {
		if changed == 0 {
			_, _ = fmt.Fprintf(cfg.Stdout, "all %d skill(s) already migrated\n", total)
			return nil
		}
		_, _ = fmt.Fprintf(cfg.Stdout,
			"%d of %d skill(s) carry provenance in frontmatter (run: exegesis merge-migrate)\n",
			changed, total)
		return root.ExitError(1)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "migrated %d of %d skill(s)\n", changed, total)
	return nil
}
