// Package verify implements the "verify" command: run every structural gate
// over a skill tree and emit skills-manifest.json for the skillsaw hand-off.
package verify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	lintlib "github.com/StevenACoffman/exegesis/internal/lint"
	"github.com/StevenACoffman/exegesis/internal/manifest"
	"github.com/StevenACoffman/exegesis/internal/overview"
	"github.com/StevenACoffman/exegesis/internal/registry"
	"github.com/StevenACoffman/exegesis/internal/skill"
	"github.com/StevenACoffman/exegesis/internal/testprompts"
)

// Config holds the verify command configuration.
type Config struct {
	*root.Config
	Manifest string
	Registry string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// skillReport is one skill's gate outcome.
type skillReport struct {
	dir      string
	slug     string
	hash     string
	findings []lintlib.Finding
	problems []string // test-prompts problems (incl. "missing test-prompts.json")
	hasTests bool
}

// New creates and registers the verify command.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("verify").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Manifest, 0, "manifest", "",
		"manifest output path (default <tree>/skills-manifest.json)")
	cfg.Flags.StringVar(
		&cfg.Registry,
		0,
		"registry",
		"",
		"optional registry JSON: expected_skills, max_body_words, max_description_words, required_sections",
	)
	cfg.Command = &ff.Command{
		Name:      "verify",
		Usage:     "exegesis verify [--manifest PATH] [--registry PATH] [TREE]",
		ShortHelp: "run every gate over a skill tree and emit skills-manifest.json",
		LongHelp: `Run the Stage-0 overview gate (if TREE/BOOK_OVERVIEW.md exists), then lint
and the test-prompts composition gate for every skill under TREE (default "."):
each immediate subdirectory containing a SKILL.md.

With --registry, also enforce per-skill word budgets and required sections and
check the discovered skills against the expected catalog.

On completion it writes skills-manifest.json (structure_verified reflects whether
every gate passed, and each entry carries the skill's sha256) for the
skillsaw-skill hand-off, and exits non-zero if any gate failed.`,
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
		return errors.New("verify: expected at most one tree path")
	}

	opts, expected, err := cfg.loadRegistry()
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	overviewProblems := checkOverview(tree)
	reports, err := verifySkills(tree, opts)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	catalogProblems := checkCatalog(expected, reports)

	verified := len(overviewProblems) == 0 && len(catalogProblems) == 0 && allPass(reports)
	if err := cfg.writeManifest(tree, reports, verified); err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	cfg.render(overviewProblems, catalogProblems, reports)
	if !verified {
		return root.ExitError(1)
	}
	return nil
}

// loadRegistry turns the optional --registry file into lint options plus the
// expected-skill catalog. No flag means no enforcement.
func (cfg *Config) loadRegistry() (lintlib.Options, []string, error) {
	if cfg.Registry == "" {
		return lintlib.Options{}, nil, nil
	}
	r, err := registry.Load(cfg.Registry)
	if err != nil {
		return lintlib.Options{}, nil, fmt.Errorf("load registry: %w", err)
	}
	return lintlib.Options{
		MaxBodyWords:        r.MaxBodyWords,
		MaxDescriptionWords: r.MaxDescriptionWords,
		RequiredSections:    r.RequiredSections,
	}, r.ExpectedSkills, nil
}

// checkCatalog compares discovered skills against an expected catalog; an empty
// expected list disables the check.
func checkCatalog(expected []string, reports []skillReport) []string {
	if len(expected) == 0 {
		return nil
	}
	found := make(map[string]bool, len(reports))
	for i := range reports {
		found[reports[i].slug] = true
	}
	want := make(map[string]bool, len(expected))
	var problems []string
	for _, e := range expected {
		want[e] = true
		if !found[e] {
			problems = append(problems, fmt.Sprintf("catalog: expected skill %q not found", e))
		}
	}
	for i := range reports {
		if slug := reports[i].slug; !want[slug] {
			problems = append(
				problems,
				fmt.Sprintf("catalog: unexpected skill %q not in registry", slug),
			)
		}
	}
	return problems
}

// checkOverview runs the Stage-0 gate when a BOOK_OVERVIEW.md is present. A
// missing overview is not a failure (a bare skill tree need not have one).
func checkOverview(tree string) []string {
	b, err := os.ReadFile(filepath.Join(tree, "BOOK_OVERVIEW.md"))
	if err != nil {
		return nil
	}
	return overview.Check(string(b))
}

func verifySkills(tree string, opts lintlib.Options) ([]skillReport, error) {
	dirs, err := skill.Discover(tree)
	if err != nil {
		return nil, fmt.Errorf("discover skills: %w", err)
	}
	reports := make([]skillReport, 0, len(dirs))
	for _, dir := range dirs {
		reports = append(reports, verifyOne(dir, opts))
	}
	return reports, nil
}

func verifyOne(dir string, opts lintlib.Options) skillReport {
	r := skillReport{dir: dir, slug: filepath.Base(dir)}
	if s, err := skill.Load(dir); err == nil {
		r.hash = skill.Hash(s.Raw)
		r.findings = lintlib.Check(s, opts)
	} else {
		r.findings = []lintlib.Finding{{Severity: lintlib.Error, Message: err.Error()}}
	}
	tpPath := filepath.Join(dir, "test-prompts.json")
	f, err := testprompts.Load(tpPath)
	switch {
	case err != nil:
		r.problems = []string{"missing or unreadable test-prompts.json"}
	default:
		r.hasTests = true
		r.problems = f.Validate()
	}
	return r
}

func (cfg *Config) writeManifest(tree string, reports []skillReport, verified bool) error {
	entries := make([]manifest.Skill, 0, len(reports))
	for _, r := range reports {
		e := manifest.Skill{Slug: r.slug, Dir: r.dir, Hash: r.hash}
		if r.hasTests {
			e.TestPrompts = filepath.Join(r.dir, "test-prompts.json")
		}
		entries = append(entries, e)
	}
	b, err := manifest.Build(tree, entries, verified).Marshal()
	if err != nil {
		return fmt.Errorf("build manifest: %w", err)
	}
	path := cfg.Manifest
	if path == "" {
		path = filepath.Join(tree, "skills-manifest.json")
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write manifest %s: %w", path, err)
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "wrote %s (structure_verified=%t)\n", path, verified)
	return nil
}

func (cfg *Config) render(overviewProblems, catalogProblems []string, reports []skillReport) {
	for _, p := range overviewProblems {
		_, _ = fmt.Fprintf(cfg.Stdout, "BOOK_OVERVIEW.md: %s\n", p)
	}
	for _, p := range catalogProblems {
		_, _ = fmt.Fprintf(cfg.Stdout, "%s\n", p)
	}
	for i := range reports {
		r := &reports[i]
		if skillPasses(r) {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s: ok\n", r.slug)
			continue
		}
		for _, f := range r.findings {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s: %s: %s\n", r.slug, f.Severity, f.Message)
		}
		for _, p := range r.problems {
			_, _ = fmt.Fprintf(cfg.Stdout, "%s: test-prompts: %s\n", r.slug, p)
		}
	}
}

func allPass(reports []skillReport) bool {
	for i := range reports {
		if !skillPasses(&reports[i]) {
			return false
		}
	}
	return true
}

func skillPasses(r *skillReport) bool {
	for _, f := range r.findings {
		if f.Severity == lintlib.Error {
			return false
		}
	}
	return r.hasTests && len(r.problems) == 0
}
