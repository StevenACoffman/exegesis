// Package scaffold builds structurally-valid skill frames offline from a schema of
// candidate skills: a SKILL.md with honest frontmatter and the six RIA-TV++ segment
// headings, plus a gate-passing test-prompts.json. RenderSkill and BuildTests are pure;
// the command shell writes the tree and verifies what it wrote. It never calls a model —
// scaffold stays in exegesis's structure tier.
package scaffold

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/StevenACoffman/exegesis/internal/related"
	"github.com/StevenACoffman/skillet/skill"
	"github.com/StevenACoffman/skillet/testprompts"
)

// Schema is the top-level scaffold input: the list of candidate skills.
type Schema struct {
	Skills []Skill `json:"skills"`
}

// Skill is one candidate to scaffold. Slug and Description are required; Related and
// TestPrompts are optional.
type Skill struct {
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	Related     []Edge   `json:"related,omitempty"`
	TestPrompts []Prompt `json:"test_prompts,omitempty"`
}

// Edge is a related-skill edge in the schema; Kind is one of related's kinds.
type Edge struct {
	Kind      string `json:"kind"`
	Target    string `json:"target"`
	Rationale string `json:"rationale,omitempty"`
}

// Prompt is one schema-supplied test prompt; its checks are derived from Expected.
type Prompt struct {
	Type     string `json:"type"`
	Prompt   string `json:"prompt"`
	Expected string `json:"expected"`
}

// RenderSkill returns the SKILL.md content for s: frontmatter (name + description), the
// RIA stub frame the author fills, and a Related skills section for any edges. It errors
// on an unknown related kind.
func RenderSkill(s *Skill) (string, error) {
	edges, err := toEdges(s.Related)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: %q\n---\n\n", skill.Slug(s.Slug), s.Description)
	b.WriteString(riaFrame())
	if len(edges) > 0 {
		b.WriteString("\n## Related skills\n\n")
		for i := range edges {
			b.WriteString(related.Bullet(edges[i]) + "\n")
		}
	}
	return b.String(), nil
}

// BuildTests returns the test-prompts file for s: the schema-supplied prompts with checks
// derived from each Expected, or a gate-passing Scaffold stub when none are supplied.
func BuildTests(s *Skill) *testprompts.File {
	slug := skill.Slug(s.Slug)
	if len(s.TestPrompts) == 0 {
		return testprompts.Scaffold(slug)
	}
	f := &testprompts.File{Skill: slug, Tests: make([]testprompts.Case, 0, len(s.TestPrompts))}
	for i := range s.TestPrompts {
		p := &s.TestPrompts[i]
		f.Tests = append(f.Tests, testprompts.Case{
			ID:       i + 1,
			Type:     p.Type,
			Prompt:   p.Prompt,
			Expected: p.Expected,
			Checks:   testprompts.DeriveChecks(p.Expected),
		})
	}
	return f
}

// riaFrame renders the six RIA-TV++ segment headings, each with a placeholder for the
// author to fill. The segment semantics are the author's; the scaffold only guarantees
// the headings that `lint --check redlines` requires are present.
func riaFrame() string {
	var b strings.Builder
	for _, seg := range []string{"R", "I", "A1", "A2", "E", "B"} {
		fmt.Fprintf(&b, "## %s\n\n<!-- TODO: fill the %s segment. -->\n\n", seg, seg)
	}
	return b.String()
}

// toEdges maps schema edges to related.Edge, validating each kind against related's
// closed set (an unknown kind is an error, never a silent drop).
func toEdges(in []Edge) ([]related.Edge, error) {
	out := make([]related.Edge, 0, len(in))
	for i := range in {
		kind := related.Kind(in[i].Kind)
		if !kind.Valid() {
			return nil, errors.New("unknown related kind " + strconv.Quote(in[i].Kind))
		}
		out = append(out, related.Edge{
			Kind:      kind,
			Target:    skill.Slug(in[i].Target),
			Rationale: in[i].Rationale,
		})
	}
	return out, nil
}
