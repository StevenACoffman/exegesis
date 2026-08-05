// Package relate implements the "relate" command: it applies a centralized relations
// edge table across a book's skills — writing each source skill's `## Related skills`
// section through the same path as `link` — then regenerates INDEX.md. It is the bulk
// counterpart to `link` (one edge) and reuses `index`'s regeneration.
package relate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/indexgen"
	relatelib "github.com/StevenACoffman/exegesis/internal/relate"
	"github.com/StevenACoffman/exegesis/internal/related"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/skill"
)

// Config holds the relate command's flag values and ff wiring. It embeds *root.Config
// for shared I/O.
type Config struct {
	*root.Config
	Edges   string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the relate command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("relate").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Edges, 0, "edges", "",
		"JSON relations edge table: {\"edges\":[{from,kind,to,rationale}]}")
	cfg.Command = &ff.Command{
		Name:      "relate",
		Usage:     "exegesis relate --edges edges.json TREE",
		ShortHelp: "apply a relations edge table across a book's skills, then rebuild INDEX",
		LongHelp: `Read a JSON relations edge table and write each source skill's
` + "`## Related skills`" + ` section — the same idempotent write path as ` + "`link`" + `,
batched across the whole book — then regenerate INDEX.md.

Each edge is {from, kind, to, rationale}; kind is depends-on, contrasts-with, or
composes-with. Re-running the same table is a no-op on the sections. A missing source
skill is an error. Replaces hand-editing every skill to cold-start a book's graph.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec parses the edge table, writes each source skill's related section, and
// regenerates INDEX.md. Flags are already parsed.
func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("relate: need exactly one tree directory")
	}
	if cfg.Edges == "" {
		return errors.New("relate: --edges is required")
	}
	tree := args[0]
	data, err := os.ReadFile(cfg.Edges)
	if err != nil {
		return fmt.Errorf("relate: read edges %s: %w", cfg.Edges, err)
	}
	groups, err := relatelib.Parse(data)
	if err != nil {
		return fmt.Errorf("relate: %w", err)
	}
	linked, changed := 0, 0
	for i := range groups {
		g := &groups[i]
		wroteIt, applyErr := cfg.applyGroup(tree, g)
		if applyErr != nil {
			return applyErr
		}
		if wroteIt {
			changed++
		}
		linked += len(g.Edges)
	}
	if err := cfg.rebuildIndex(tree); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "relate: linked %d edge(s) across %d skill(s); wrote %s\n",
		linked, changed, indexgen.Path(tree))
	return nil
}

// applyGroup writes one source skill's related section from its edges, reporting whether
// the file changed. A missing source skill is an error.
func (cfg *Config) applyGroup(tree string, g *relatelib.Group) (bool, error) {
	s, err := skill.Load(filepath.Join(tree, g.Slug))
	if err != nil {
		return false, fmt.Errorf("relate: source skill %q: %w", g.Slug, err)
	}
	out, changed := related.UpsertAll(s.Raw, g.Edges)
	if !changed {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: unchanged (%d edge(s))\n", g.Slug, len(g.Edges))
		return false, nil
	}
	if err := atomicfile.WriteFile(s.Path, []byte(out), 0o644); err != nil {
		return false, fmt.Errorf("relate: write %s: %w", s.Path, err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "%s: linked %d edge(s)\n", g.Slug, len(g.Edges))
	return true, nil
}

// rebuildIndex regenerates INDEX.md from the skills' related sections (title/author
// derived from BOOK_OVERVIEW.md, as `index` does).
func (cfg *Config) rebuildIndex(tree string) error {
	content, err := indexgen.Generate(tree, "", "")
	if err != nil {
		return fmt.Errorf("relate: generate index: %w", err)
	}
	if err := atomicfile.WriteFile(indexgen.Path(tree), []byte(content), 0o644); err != nil {
		return fmt.Errorf("relate: write index: %w", err)
	}
	return nil
}
