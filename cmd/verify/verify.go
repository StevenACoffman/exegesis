// Package verify implements the "verify" command: run every structural gate
// over a skill tree and emit skills-manifest.json for the skillsaw hand-off.
package verify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	lintlib "github.com/StevenACoffman/exegesis/internal/lint"
	"github.com/StevenACoffman/exegesis/internal/overview"
	"github.com/StevenACoffman/exegesis/internal/registry"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/manifest"
	"github.com/StevenACoffman/skillet/skill"
	"github.com/StevenACoffman/skillet/testprompts"
)

// Config holds the verify command configuration.
type Config struct {
	*root.Config
	Manifest string
	Registry string
	Gates    string
	Check    string
	Flags    *ff.FlagSet
	Command  *ff.Command
}

// skillReport is one skill's gate outcome.
type skillReport struct {
	dir      string
	slug     string
	hash     string
	findings []finding.Diagnostic
	problems []string // test-prompts problems (incl. "missing test-prompts.json")
	hasTests bool
}

// gateSet is which verify gates to run. The zero value runs none; all=true (the
// default, when --gates is empty) runs every gate. overview and skills are set
// when named explicitly.
type gateSet struct{ all, overview, skills bool }

func (g gateSet) runOverview() bool { return g.all || g.overview }

func (g gateSet) runSkills() bool { return g.all || g.skills }

// strictOverview reports whether the overview gate was named explicitly, in
// which case a missing BOOK_OVERVIEW.md is a failure rather than skipped.
func (g gateSet) strictOverview() bool { return g.overview }

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
	cfg.Flags.StringVar(&cfg.Gates, 0, "gates", "",
		"comma-separated gates to run: overview, skills (default: all)")
	cfg.Flags.StringVar(&cfg.Check, 0, "check", "",
		"extra per-skill checks: redlines (or all) enforces the mechanical Quality Red Lines")
	cfg.Command = &ff.Command{
		Name:      "verify",
		Usage:     "exegesis verify [--gates LIST] [--check redlines] [--manifest PATH] [--registry PATH] [TREE]",
		ShortHelp: "run every gate over a skill tree and emit skills-manifest.json",
		LongHelp: `Run the Stage-0 overview gate (if TREE/BOOK_OVERVIEW.md exists), then lint
and the test-prompts composition gate for every skill under TREE (default "."):
each immediate subdirectory containing a SKILL.md.

--gates selects a subset: "overview" runs only the Stage-0 BOOK_OVERVIEW.md gate
(and requires the file to exist); "skills" runs only the per-skill gates. The
default (no --gates) runs both. Only a run that includes the skills gate writes
skills-manifest.json.

With --registry, also enforce per-skill word budgets and required sections and
check the discovered skills against the expected catalog.

On completion a skills run writes skills-manifest.json (structure_verified
reflects whether every gate passed, and each entry carries the skill's sha256)
for the skillsaw-skill hand-off, and exits non-zero if any gate failed.`,
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
	gates, err := parseGates(cfg.Gates)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	ok, err := cfg.run(tree, gates)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	if !ok {
		return root.ExitError(1)
	}
	return nil
}

// run executes the selected gates and reports whether all of them passed. The
// overview outcome is threaded into the skills run so the manifest's
// structure_verified reflects every gate that ran.
func (cfg *Config) run(tree string, gates gateSet) (bool, error) {
	overviewPass := true
	if gates.runOverview() {
		problems, ran := checkOverview(tree, gates.strictOverview())
		if ran {
			cfg.renderOverview(problems)
		}
		overviewPass = len(problems) == 0
	}
	if !gates.runSkills() {
		return overviewPass, nil
	}
	return cfg.runSkills(tree, overviewPass)
}

// runSkills lints and gates every skill under tree, writes the manifest, and
// reports whether the skills (with the overview result folded in) passed.
func (cfg *Config) runSkills(tree string, overviewPass bool) (bool, error) {
	opts, expected, err := cfg.loadRegistry()
	if err != nil {
		return false, err
	}
	redlines, err := lintlib.ParseCheck(cfg.Check)
	if err != nil {
		return false, fmt.Errorf("verify: %w", err)
	}
	opts.Redlines = redlines
	reports, err := verifySkills(tree, opts)
	if err != nil {
		return false, err
	}
	catalogProblems := checkCatalog(expected, reports)
	verified := overviewPass && len(catalogProblems) == 0 && allPass(reports)
	if err := cfg.writeManifest(tree, reports, verified); err != nil {
		return false, err
	}
	cfg.renderSkills(catalogProblems, reports)
	return verified, nil
}

// parseGates turns the --gates value into a gateSet. An empty value selects all
// gates (the default). Unknown names are rejected.
//
// Ensures: err != nil iff s names a gate other than "overview" or "skills".
func parseGates(s string) (gateSet, error) {
	if strings.TrimSpace(s) == "" {
		return gateSet{all: true}, nil
	}
	var g gateSet
	for _, name := range strings.Split(s, ",") {
		switch name = strings.TrimSpace(name); name {
		case "":
			continue // tolerate stray/trailing commas
		case "overview":
			g.overview = true
		case "skills":
			g.skills = true
		default:
			return gateSet{}, fmt.Errorf("unknown gate %q (known: overview, skills)", name)
		}
	}
	return g, nil
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

// checkOverview runs the Stage-0 gate on TREE/BOOK_OVERVIEW.md. ran reports
// whether the gate evaluated anything. When the file is missing: a lenient run
// skips it (nil, false) because a bare skill tree need not have one; a strict
// run (the overview gate named explicitly) fails it (problem, true).
func checkOverview(tree string, strict bool) (problems []string, ran bool) {
	b, err := os.ReadFile(filepath.Join(tree, "BOOK_OVERVIEW.md"))
	if err != nil {
		if strict {
			return []string{"not found (required for --gates overview)"}, true
		}
		return nil, false
	}
	return overview.Check(string(b)), true
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
		r.hash = s.Hash()
		r.findings = lintlib.Check(s, opts)
	} else {
		r.findings = []finding.Diagnostic{{Severity: finding.SeverityError, Message: err.Error()}}
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
	b, err := manifest.Build("exegesis", tree, entries, verified).Marshal()
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

// renderOverview reports the overview gate outcome: one line per problem, or a
// single "ok" line when the gate ran and passed.
func (cfg *Config) renderOverview(problems []string) {
	if len(problems) == 0 {
		_, _ = fmt.Fprintln(cfg.Stdout, "BOOK_OVERVIEW.md: ok")
		return
	}
	for _, p := range problems {
		_, _ = fmt.Fprintf(cfg.Stdout, "BOOK_OVERVIEW.md: %s\n", p)
	}
}

func (cfg *Config) renderSkills(catalogProblems []string, reports []skillReport) {
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
		if f.Severity == finding.SeverityError {
			return false
		}
	}
	return r.hasTests && len(r.problems) == 0
}
