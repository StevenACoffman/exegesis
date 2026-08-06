// Package scaffold implements the "scaffold" command: it writes a tree of
// structurally-valid skill frames offline from a JSON schema, then verifies each on
// write — removing any newly-created skill that fails the structural gates so it never
// leaves a failing tree. It calls no model; it is the fast, deterministic counterpart to
// the agent-driven `distill`.
package scaffold

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd/root"
	lintlib "github.com/StevenACoffman/exegesis/internal/lint"
	scaffoldlib "github.com/StevenACoffman/exegesis/internal/scaffold"
	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/skill"
	"github.com/StevenACoffman/skillet/testprompts"
)

// The outcome of scaffolding one skill.
const (
	wrote outcome = iota
	skipped
	failed
)

// outcome is what scaffoldOne did with one skill.
type outcome int

// Config holds the scaffold command's flag values and ff wiring. It embeds
// *root.Config for shared I/O.
type Config struct {
	*root.Config
	Schema  string
	Output  string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the scaffold command under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("scaffold").SetParent(parent.Flags)
	cfg.Flags.StringVar(&cfg.Schema, 0, "schema", "",
		"JSON schema of candidate skills to scaffold")
	cfg.Flags.StringVar(&cfg.Output, 0, "output-dir", "",
		"directory to write the skill tree into")
	cfg.Command = &ff.Command{
		Name:      "scaffold",
		Usage:     "exegesis scaffold --schema candidates.json --output-dir DIR",
		ShortHelp: "write a tree of skill frames offline from a schema, gated on write",
		LongHelp: `Read a JSON schema of candidate skills and, in one offline pass, write
each skill's directory with a SKILL.md frame (frontmatter + the six RIA-TV++ segment
headings for the author to fill) and a gate-passing test-prompts.json.

Unlike distill (agent-driven), scaffold calls no model. It verifies on write: each
newly-created skill is run through lint --check redlines and the test-prompts
composition gate, and any that fails is removed — so scaffold never leaves a failing
tree. Existing skill directories are skipped, not overwritten.

The schema is {"skills": [{slug, description, related?, test_prompts?}]}.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

// exec loads the schema, scaffolds each skill, and tallies the outcomes. Flags are
// already parsed. It returns ExitError(1) if any skill failed its gates.
func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.Schema == "" {
		return root.Usagef("scaffold: --schema is required")
	}
	if cfg.Output == "" {
		return root.Usagef("scaffold: --output-dir is required")
	}
	schema, err := loadSchema(cfg.Schema)
	if err != nil {
		return err
	}
	if len(schema.Skills) == 0 {
		return errors.New("scaffold: schema has no skills")
	}
	tally := map[outcome]int{}
	for i := range schema.Skills {
		oc, oneErr := cfg.scaffoldOne(&schema.Skills[i])
		if oneErr != nil {
			return oneErr
		}
		tally[oc]++
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "scaffold: wrote %d, skipped %d, failed %d\n",
		tally[wrote], tally[skipped], tally[failed])
	if tally[failed] > 0 {
		return root.ExitError(1)
	}
	return nil
}

// scaffoldOne skips an existing skill, else writes and verifies it, removing it and
// reporting its problems when it fails the gates. It returns a non-nil error only on an
// empty slug or an I/O/schema failure.
func (cfg *Config) scaffoldOne(s *scaffoldlib.Skill) (outcome, error) {
	slug := skill.Slug(s.Slug)
	if slug == "" {
		return failed, errors.New("scaffold: a skill has an empty slug")
	}
	dir := filepath.Join(cfg.Output, slug)
	if _, statErr := os.Stat(dir); statErr == nil {
		_, _ = fmt.Fprintf(cfg.Stdout, "skipped %s (exists)\n", slug)
		return skipped, nil
	}
	problems, err := cfg.writeSkill(dir, s)
	if err != nil {
		return failed, err
	}
	if len(problems) == 0 {
		_, _ = fmt.Fprintf(cfg.Stdout, "wrote %s\n", slug)
		return wrote, nil
	}
	_, _ = fmt.Fprintf(cfg.Stderr, "failed %s (removed):\n", slug)
	for _, p := range problems {
		_, _ = fmt.Fprintf(cfg.Stderr, "  %s\n", p)
	}
	_ = os.RemoveAll(dir)
	return failed, nil
}

// writeSkill renders and writes s into dir, then verifies it. It returns the structural
// problems found (empty = clean) and a non-nil error only on an I/O or schema failure.
func (cfg *Config) writeSkill(dir string, s *scaffoldlib.Skill) ([]string, error) {
	content, err := scaffoldlib.RenderSkill(s)
	if err != nil {
		return nil, fmt.Errorf("scaffold: %w", err)
	}
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return nil, fmt.Errorf("scaffold: create dir %s: %w", dir, mkErr)
	}
	if wErr := atomicfile.WriteFile(
		filepath.Join(dir, "SKILL.md"),
		[]byte(content),
		0o644,
	); wErr != nil {
		return nil, fmt.Errorf("scaffold: write SKILL.md in %s: %w", dir, wErr)
	}
	if wErr := testprompts.Write(
		filepath.Join(dir, "test-prompts.json"),
		scaffoldlib.BuildTests(s),
	); wErr != nil {
		return nil, fmt.Errorf("scaffold: write test-prompts in %s: %w", dir, wErr)
	}
	return verifySkill(dir)
}

// verifySkill loads the just-written skill and returns its structural problems: blocking
// lint findings (redlines on) plus test-prompts composition problems.
func verifySkill(dir string) ([]string, error) {
	loaded, err := skill.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("scaffold: reload skill %s: %w", dir, err)
	}
	var problems []string
	for _, d := range lintlib.Check(loaded, lintlib.Options{Redlines: true}) {
		if d.Severity.Blocking() {
			problems = append(problems, d.Message)
		}
	}
	f, err := testprompts.Load(filepath.Join(dir, "test-prompts.json"))
	if err != nil {
		return nil, fmt.Errorf("scaffold: reload test-prompts %s: %w", dir, err)
	}
	return append(problems, f.Validate()...), nil
}

// loadSchema reads and parses the candidate-skills schema.
func loadSchema(path string) (scaffoldlib.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return scaffoldlib.Schema{}, fmt.Errorf("scaffold: read schema %s: %w", path, err)
	}
	var schema scaffoldlib.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return scaffoldlib.Schema{}, fmt.Errorf("scaffold: parse schema: %w", err)
	}
	return schema, nil
}
