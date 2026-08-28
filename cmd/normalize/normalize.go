// Package normalize implements `exegesis normalize`: it rewrites every skill's
// `## Related skills` section into the canonical bullet format that `link` and
// `relate` write, leaving anything it cannot parse untouched. The rewrite is pure
// (internal/related); this command discovers the skills and reads/writes files.
package normalize

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/indexgen"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/related"
	"github.com/StevenACoffman/skillet/skill"
)

// Config holds the normalize command configuration.
type Config struct {
	*root.Config
	Check         bool
	ResolveTitles bool
	Write         bool
	Flags         *ff.FlagSet
	Command       *ff.Command
}

// New creates and registers the normalize command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("normalize").SetParent(parent.Flags)
	cfg.Flags.BoolVar(&cfg.Check, 0, "check",
		"report which skills are not canonical without writing (exit 1 if any)")
	cfg.Flags.BoolVar(&cfg.ResolveTitles, 0, "resolve-titles",
		"report bullets naming a skill by display title, and what each resolves to")
	cfg.Flags.BoolVar(&cfg.Write, 0, "write",
		"with --resolve-titles, rewrite the exactly-and-unambiguously resolved ones")
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
	if cfg.ResolveTitles {
		return cfg.reportTitles(tree)
	}
	if cfg.Write {
		// --write only qualifies --resolve-titles. Accepting it alone would be silently
		// ignoring a flag whose whole meaning is "also change files", which is the one
		// kind of no-op a caller must not be left to discover from the diff.
		return root.Usagef("normalize: --write applies to --resolve-titles; " +
			"the plain rewrite writes unless --check is given")
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

// reportTitles lists the bullets that name a skill by display title, and what each
// resolves to in this tree. With --write it rewrites the ones that resolve.
//
// **Reporting was the whole command until skillet v0.26.0, and the reason is worth
// keeping.** Resolving a title was necessary but not sufficient: the bullets carrying
// titles also use an italic kind and an arrow separator, which the parser did not read,
// so substituting the correct slug left them exactly as unreadable. That was confirmed by
// handing Normalize a bullet whose bold token was already a valid slug and watching it
// pass through untouched. v0.26.0's dialect tolerance is what made rewriting worth doing,
// so --write is downstream of a parser change rather than of a decision here.
//
// Reporting stays the default because an unresolved title is not a defect this command
// can fix: it is a bullet naming something no skill in the tree is called, and the report
// is the only place that says so.
func (cfg *Config) reportTitles(tree string) error {
	nodes, err := indexgen.CollectNodes(tree)
	if err != nil {
		return fmt.Errorf("normalize: %w", err)
	}
	if cfg.Write {
		return cfg.writeTitles(tree, nodes)
	}
	refs := related.TitleRefs(nodes)
	counts := map[related.Resolution]int{}
	for _, r := range refs {
		counts[r.Resolution]++
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\t%s\t%s\t%s\n",
			r.From, r.Resolution, r.Title, r.Slug)
	}
	_, _ = fmt.Fprintf(
		cfg.Stdout,
		"%d title-named bullet(s): %d resolved, %d unknown, %d ambiguous\n",
		len(refs),
		counts[related.TitleResolved],
		counts[related.TitleUnknown],
		counts[related.TitleAmbiguous],
	)
	return nil
}

// writeTitles rewrites the bullets whose display title resolves exactly and
// unambiguously, then canonicalises the sections it touched.
//
// **Two passes, and the split is the safety.** ResolveTitles only substitutes the bold
// token; Normalize then reads the bullet the same way it reads every other one, so a
// resolved title produces an edge through exactly the path an authored slug does. Doing
// it in one step would mean the parser trusting a lookup, and a wrong lookup would then
// become an edge with nothing recording that a substitution happened.
//
// An unknown or ambiguous title is left byte-identical and still reported by the
// report-only mode, so nothing is silently dropped by choosing to write.
func (cfg *Config) writeTitles(tree string, nodes []related.Node) error {
	titles := related.NewTitles(nodes)
	known := make(map[string]bool, len(nodes))
	for i := range nodes {
		known[nodes[i].Slug] = true
	}
	dirs, err := skill.Discover(tree)
	if err != nil {
		return fmt.Errorf("normalize: %w", err)
	}
	resolved, files := 0, 0
	for _, dir := range dirs {
		s, loadErr := skill.Load(dir)
		if loadErr != nil {
			return fmt.Errorf("normalize: %w", loadErr)
		}
		out, n := related.ResolveTitles(s.Raw, titles, known)
		if n == 0 {
			continue
		}
		out, _ = related.Normalize(out)
		// 0o644, as every other write path here does. A skill is a readable document
		// and this is one write among several over the same file; tightening the mode
		// on the subset of files that happen to carry a display title would leave a
		// tree with two permission regimes and no reason recorded for either.
		if writeErr := atomicfile.WriteFile(s.Path, []byte(out), 0o644); writeErr != nil {
			return fmt.Errorf("normalize: %w", writeErr)
		}
		resolved += n
		files++
		_, _ = fmt.Fprintf(cfg.Stdout, "%s: resolved %d title(s)\n", filepath.Base(dir), n)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "resolved %d title(s) across %d skill(s)\n", resolved, files)
	return nil
}
