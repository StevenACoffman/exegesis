// Package testprompts is exegesis's half of the test-prompts.json contract
// shared with skillsaw. It defines the canonical on-disk schema, the composition
// gate exegesis enforces, and a scaffold generator. Parsing/validation are pure;
// only Load and Write touch the filesystem.
//
// A case carries an activation Type (exegesis gates the composition) and an
// optional Checks array (skillsaw's judge consumes it) — one file serves both
// tools. See DeriveChecks (derive.go) for how Checks are seeded from Expected.
package testprompts

import (
	"encoding/json"
	"fmt"
	"os"
)

// Case types (the activation composition exegesis gates).
const (
	TypeShouldTrigger    = "should_trigger"
	TypeShouldNotTrigger = "should_not_trigger"
	TypeEdgeCase         = "edge_case"
)

// Composition minimums (spec §Phase-4): a set without decoys and an edge case
// only ever looks "good", so all three are required.
const (
	MinTrigger = 3
	MinDecoy   = 2
	MinEdge    = 1
)

// Check is one deterministic rule for skillsaw's judge; the operator set mirrors
// skillsaw/internal/judge exactly so the file is portable between the tools.
type Check struct {
	Op  string `json:"op"`
	Arg string `json:"arg"`
}

// Case is one test prompt.
type Case struct {
	ID       int     `json:"id"`
	Type     string  `json:"type"`
	Prompt   string  `json:"prompt"`
	Expected string  `json:"expected"`
	Checks   []Check `json:"checks,omitempty"`
}

// File is a parsed test-prompts.json in canonical form.
type File struct {
	Skill string `json:"skill,omitempty"`
	Tests []Case `json:"tests"`
}

// Counts tallies cases by type.
type Counts struct {
	Trigger int
	Decoy   int
	Edge    int
}

// Load reads and parses a canonical test-prompts.json ({"tests": [...]}).
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load test-prompts %s: %w", path, err)
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse test-prompts %s: %w", path, err)
	}
	return &f, nil
}

// Write marshals f to path in canonical, indented form.
func Write(path string, f *File) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encode test-prompts: %w", err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write test-prompts %s: %w", path, err)
	}
	return nil
}

// Tally returns the per-type case counts.
func (f *File) Tally() Counts {
	var c Counts
	for _, tc := range f.Tests {
		switch tc.Type {
		case TypeShouldTrigger:
			c.Trigger++
		case TypeShouldNotTrigger:
			c.Decoy++
		case TypeEdgeCase:
			c.Edge++
		}
	}
	return c
}

// Validate returns one problem string per composition or per-case defect; an
// empty slice means the set passes the gate.
//
// Requires: f is non-nil.
// Ensures:  result is empty iff every case is well-formed and the counts meet
//
//	MinTrigger/MinDecoy/MinEdge.
func (f *File) Validate() []string {
	var problems []string
	seen := map[int]bool{}
	for _, tc := range f.Tests {
		switch tc.Type {
		case TypeShouldTrigger, TypeShouldNotTrigger, TypeEdgeCase:
		default:
			problems = append(problems, fmt.Sprintf("case %d: unknown type %q", tc.ID, tc.Type))
		}
		if tc.Prompt == "" {
			problems = append(problems, fmt.Sprintf("case %d: empty prompt", tc.ID))
		}
		if tc.Expected == "" {
			problems = append(problems, fmt.Sprintf("case %d: empty expected", tc.ID))
		}
		if seen[tc.ID] {
			problems = append(problems, fmt.Sprintf("duplicate id %d", tc.ID))
		}
		seen[tc.ID] = true
	}
	c := f.Tally()
	if c.Trigger < MinTrigger {
		problems = append(
			problems,
			fmt.Sprintf("need >=%d should_trigger, have %d", MinTrigger, c.Trigger),
		)
	}
	if c.Decoy < MinDecoy {
		problems = append(
			problems,
			fmt.Sprintf("need >=%d should_not_trigger, have %d", MinDecoy, c.Decoy),
		)
	}
	if c.Edge < MinEdge {
		problems = append(problems, fmt.Sprintf("need >=%d edge_case, have %d", MinEdge, c.Edge))
	}
	return problems
}

// Scaffold returns a minimal passing-shape File for skillName: MinTrigger
// triggers, MinDecoy decoys, and MinEdge edge cases. Each case's Checks are
// seeded from its Expected via DeriveChecks, demonstrating the seam.
func Scaffold(skillName string) *File {
	f := &File{Skill: skillName}
	id := 0
	add := func(typ, prompt, expected string) {
		id++
		f.Tests = append(f.Tests, Case{
			ID:       id,
			Type:     typ,
			Prompt:   prompt,
			Expected: expected,
			Checks:   DeriveChecks(expected),
		})
	}
	for i := 0; i < MinTrigger; i++ {
		add(TypeShouldTrigger,
			"TODO: a prompt that SHOULD activate the skill",
			`TODO: describe a good output; e.g. output contains a "Result" section`)
	}
	for i := 0; i < MinDecoy; i++ {
		add(TypeShouldNotTrigger,
			"TODO: a plausible decoy prompt that must NOT activate the skill",
			"TODO: describe why the skill should stay silent")
	}
	for i := 0; i < MinEdge; i++ {
		add(TypeEdgeCase,
			"TODO: a boundary prompt where activation is genuinely ambiguous",
			"TODO: describe the correct call at the boundary")
	}
	return f
}
