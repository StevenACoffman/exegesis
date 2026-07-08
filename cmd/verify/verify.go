// Package verify implements the "verify" command: it walks a distilled
// books/<slug>/ tree and runs every mechanical Quality Red Line in one pass —
// the Stage-0 overview gate, per-skill lint (spec/quality/redlines), the
// per-skill test-prompts structural gate, and INDEX.md staleness — reporting a
// single pass/fail. Judgment gates (triple verification, darwin scoring) stay
// with the agent.
package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/mergetree"
	"github.com/StevenACoffman/exegesis/internal/render"
	"github.com/StevenACoffman/exegesis/internal/skilllint"
	"github.com/StevenACoffman/exegesis/internal/store"
)

const (
	formatText        = "text"
	formatJSON        = "json"
	testPromptsFile   = "test-prompts.json"
	mergeOverviewFile = "MERGE_OVERVIEW.md"

	gateOverviewKey = "overview"
	gateLintKey     = "lint"
	gateTestsKey    = "tests"
	gateIndexKey    = "index"
)

// gateOutcome is the result of one verification gate. An advisory gate prints
// its problems as warnings without failing the run unless --strict is set.
type gateOutcome struct {
	Name     string   `json:"gate"`
	Pass     bool     `json:"pass"`
	Advisory bool     `json:"advisory,omitempty"`
	Problems []string `json:"problems,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// Config holds the flags and wiring for the verify command.
type Config struct {
	*root.Config
	Format  string
	Gates   string
	Source  string
	Strict  bool
	Merge   bool
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the verify command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("verify").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Format, 0, "format", formatText, "output format: text or json")
	cfg.Flags.StringVar(&cfg.Gates, 0, "gates", "",
		"comma-separated gates to run: overview,lint,tests,index (default: all)")
	cfg.Flags.BoolVar(&cfg.Strict, 0, "strict", "treat lint warnings as failures")
	cfg.Flags.BoolVar(&cfg.Merge, 0, "merge",
		"verify a merged tree: MERGE_OVERVIEW.md + lint + the merge test gate")
	cfg.Flags.StringVar(&cfg.Source, 0, "source", "",
		"comma-separated source book dirs; enables the A2-sharpness advisory gate (--merge)")
	cfg.Command = &ff.Command{
		Name:      "verify",
		Usage:     "exegesis verify [FLAGS] <book-dir>",
		ShortHelp: "run every mechanical Quality Red Line over a distilled book tree",
		LongHelp: `Verify a books/<slug>/ tree against the mechanical Quality Red
Lines in one pass: the Stage-0 BOOK_OVERVIEW.md gate, per-skill lint
(spec/quality/redlines), the per-skill test-prompts structural gate, and
INDEX.md staleness. Exit code is 1 if any gate fails (or, with --strict, if lint
warnings are present). INDEX.md staleness assumes the file was generated without
--title/--author header overrides. --merge verifies a books/merged/<slug>/ tree:
MERGE_OVERVIEW.md presence, per-skill lint, and the merge test gate (adds
prefer_merged_over_source); the single-book overview and INDEX gates are skipped.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	if len(args) == 0 {
		return einval("verify: a book directory is required")
	}
	if cfg.Format != formatText && cfg.Format != formatJSON && cfg.Format != "" {
		return einval("verify: unknown --format " + cfg.Format)
	}
	dir := args[0]
	skills, err := store.GatherSkills(dir)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	if cfg.Merge {
		return cfg.report(cfg.mergeOutcomes(dir, skills), skills)
	}
	outcomes, err := cfg.bookOutcomes(dir, skills)
	if err != nil {
		return err
	}
	return cfg.report(outcomes, skills)
}

// bookOutcomes runs the single-book gates selected by --gates.
func (cfg *Config) bookOutcomes(dir string, skills []book2skill.Skill) ([]gateOutcome, error) {
	gates, err := parseGates(cfg.Gates)
	if err != nil {
		return nil, err
	}
	overview, hasOverview, _ := store.ReadOverview(dir)
	var outcomes []gateOutcome
	if gates[gateOverviewKey] {
		outcomes = append(outcomes, gateOverview(overview, hasOverview))
	}
	if gates[gateLintKey] {
		outcomes = append(outcomes, cfg.gateLint(dir))
	}
	if gates[gateTestsKey] {
		outcomes = append(outcomes, gateTests(dir, skills, false))
	}
	if gates[gateIndexKey] {
		outcomes = append(outcomes, gateIndex(dir, overview, skills))
	}
	return outcomes, nil
}

// mergeOutcomes runs the merged-tree gates: MERGE_OVERVIEW.md presence, per-skill
// lint, and the merge test gate. The single-book overview and cross-book INDEX
// gates do not apply (the merge index is a separate, deferred renderer), and
// merge-status lives on the source skills, checked by `merge-status check`.
func (cfg *Config) mergeOutcomes(dir string, skills []book2skill.Skill) []gateOutcome {
	outcomes := []gateOutcome{
		gateMergeOverview(dir),
		cfg.gateLint(dir),
		gateTests(dir, skills, true),
	}
	if sources := cfg.mergeSources(dir); len(sources) > 0 {
		outcomes = append(outcomes, cfg.gateA2Sharpness(dir, sources))
	}
	return outcomes
}

// gateA2Sharpness is an advisory gate: for each merged skill it reads the source
// skills it was built from (via the ledgers) and flags A2 triggers that are not
// structurally sharper than both sources. It never fails unless --strict.
func (cfg *Config) gateA2Sharpness(tree string, sources []string) gateOutcome {
	model, err := mergetree.Assemble(tree, sources)
	if err != nil {
		return gateOutcome{Name: "a2-sharpness", Problems: []string{err.Error()}}
	}
	bookDir := make(map[string]string, len(sources))
	for _, d := range sources {
		bookDir[filepath.Base(filepath.Clean(d))] = d
	}
	var notes []string
	for i := range model.Merges {
		m := model.Merges[i]
		if len(m.Parents) == 0 {
			continue
		}
		unique := book2skill.A2Sharpness(
			readBody(filepath.Join(tree, m.Slug)),
			parentBodies(m.Parents, bookDir),
		)
		if len(unique) < book2skill.MinSharpSignals {
			notes = append(notes, m.Slug+": only "+strconv.Itoa(len(unique))+
				" unique A2 signal(s) vs sources (want ≥"+strconv.Itoa(book2skill.MinSharpSignals)+")")
		}
	}
	return gateOutcome{
		Name: "a2-sharpness", Advisory: true, Problems: notes,
		Pass: len(notes) == 0 || !cfg.Strict,
	}
}

// mergeSources returns the explicit --source dirs, or — when none are given —
// the dirs discovered under the books/merged/ root. Discovery failure is
// silent: the A2-sharpness gate is simply skipped (its source mapping is
// unavailable) rather than failing the run.
func (cfg *Config) mergeSources(tree string) []string {
	if explicit := splitCSV(cfg.Source); len(explicit) > 0 {
		return explicit
	}
	discovered, err := mergetree.DiscoverSources(tree)
	if err != nil {
		return nil
	}
	return discovered
}

func parentBodies(parents []book2skill.MergeParent, bookDir map[string]string) []string {
	var bodies []string
	for _, p := range parents {
		if d, ok := bookDir[p.BookSlug]; ok {
			bodies = append(bodies, readBody(filepath.Join(d, p.SkillSlug)))
		}
	}
	return bodies
}

func readBody(skillDir string) string {
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// parseGates turns the --gates flag into a gate set; empty selects all four.
func parseGates(flag string) (map[string]bool, error) {
	all := map[string]bool{
		gateOverviewKey: true, gateLintKey: true, gateTestsKey: true, gateIndexKey: true,
	}
	if strings.TrimSpace(flag) == "" {
		return all, nil
	}
	selected := make(map[string]bool)
	for _, name := range strings.Split(flag, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if !all[name] {
			return nil, einval("verify: unknown gate '" + name + "'")
		}
		selected[name] = true
	}
	return selected, nil
}

func (cfg *Config) report(outcomes []gateOutcome, skills []book2skill.Skill) error {
	if cfg.Format == formatJSON {
		if err := cfg.reportJSON(outcomes); err != nil {
			return err
		}
	} else {
		cfg.reportText(outcomes, skills)
	}
	for _, o := range outcomes {
		if !o.Pass {
			return root.ExitError(1)
		}
	}
	return nil
}

func (cfg *Config) reportText(outcomes []gateOutcome, skills []book2skill.Skill) {
	out := cfg.Stdout
	for i := range outcomes {
		o := outcomes[i]
		warn := o.Pass && o.Advisory && len(o.Problems) > 0
		status := "PASS"
		switch {
		case !o.Pass:
			status = "FAIL"
		case warn:
			status = "WARN"
		}
		_, _ = fmt.Fprintf(out, "%-16s %s\n", o.Name, status)
		if !o.Pass || warn {
			for _, p := range o.Problems {
				_, _ = fmt.Fprintln(out, "  - "+p)
			}
			if o.Detail != "" {
				_, _ = fmt.Fprintln(out, indent(o.Detail))
			}
		}
	}
	if msg := book2skill.RelationshipCountAdvice(skills); msg != "" {
		_, _ = fmt.Fprintln(out, "note: "+msg)
	}
}

func (cfg *Config) reportJSON(outcomes []gateOutcome) error {
	pass := true
	for _, o := range outcomes {
		pass = pass && o.Pass
	}
	report := struct {
		Pass  bool          `json:"pass"`
		Gates []gateOutcome `json:"gates"`
	}{Pass: pass, Gates: outcomes}
	if err := json.NewEncoder(cfg.Stdout).Encode(report); err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	return nil
}

func gateOverview(overview *book2skill.BookOverview, ok bool) gateOutcome {
	if !ok || overview == nil {
		return gateOutcome{Name: store.OverviewFile, Problems: []string{"file not found"}}
	}
	problems := overview.QualityGate()
	return gateOutcome{Name: store.OverviewFile, Pass: len(problems) == 0, Problems: problems}
}

func (cfg *Config) gateLint(dir string) gateOutcome {
	res, err := skilllint.Run(dir, skilllint.Options{Categories: map[skilllint.Category]bool{
		skilllint.CategorySpec:     true,
		skilllint.CategoryQuality:  true,
		skilllint.CategoryRedlines: true,
	}})
	if err != nil {
		return gateOutcome{Name: "skills (lint)", Problems: []string{err.Error()}}
	}
	c := res.Counts()
	outcome := gateOutcome{
		Name: "skills (lint)",
		Pass: res.ExitCode(cfg.Strict) == 0,
		Problems: []string{
			strconv.Itoa(c.Errors) + " errors, " + strconv.Itoa(c.Warnings) + " warnings",
		},
	}
	if !outcome.Pass {
		var b strings.Builder
		skilllint.WriteText(&b, res)
		outcome.Detail = strings.TrimRight(b.String(), "\n")
	}
	return outcome
}

func gateTests(dir string, skills []book2skill.Skill, merged bool) gateOutcome {
	var problems []string
	for i := range skills {
		slug := skills[i].Slug
		data, err := os.ReadFile(filepath.Join(dir, slug, testPromptsFile))
		if err != nil {
			problems = append(problems, slug+": "+testPromptsFile+" not found")
			continue
		}
		cases, err := book2skill.DecodeTestPrompts(data)
		if err != nil {
			problems = append(problems, slug+": "+err.Error())
			continue
		}
		for _, p := range validateSet(cases, merged) {
			problems = append(problems, slug+": "+p)
		}
	}
	return gateOutcome{Name: "test-prompts", Pass: len(problems) == 0, Problems: problems}
}

func validateSet(cases []book2skill.TestCase, merged bool) []string {
	if merged {
		return book2skill.ValidateMergedTestSet(cases)
	}
	return book2skill.ValidateTestSet(cases)
}

// gateMergeOverview checks that a merged tree has a MERGE_OVERVIEW.md (its
// content is judgment, so only presence is gated).
func gateMergeOverview(dir string) gateOutcome {
	if _, err := os.Stat(filepath.Join(dir, mergeOverviewFile)); err != nil {
		return gateOutcome{Name: mergeOverviewFile, Problems: []string{"file not found"}}
	}
	return gateOutcome{Name: mergeOverviewFile, Pass: true}
}

func gateIndex(
	dir string,
	overview *book2skill.BookOverview,
	skills []book2skill.Skill,
) gateOutcome {
	want := render.Index(headerFor(dir, overview), skills)
	got, err := os.ReadFile(filepath.Join(dir, store.IndexFile))
	if err == nil && string(got) == want {
		return gateOutcome{Name: store.IndexFile, Pass: true}
	}
	return gateOutcome{
		Name:     store.IndexFile,
		Problems: []string{"stale or missing; run `exegesis index " + dir + "`"},
	}
}

// headerFor derives the INDEX.md header the same way `exegesis index` does with
// no flag overrides: BOOK_OVERVIEW.md title/author, falling back to the
// directory name for the title.
func headerFor(dir string, overview *book2skill.BookOverview) *book2skill.BookOverview {
	o := &book2skill.BookOverview{}
	if overview != nil {
		o.Title, o.Author = overview.Title, overview.Author
	}
	if o.Title == "" {
		o.Title = filepath.Base(filepath.Clean(dir))
	}
	return o
}

func indent(s string) string {
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
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
