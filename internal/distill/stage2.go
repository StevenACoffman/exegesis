package distill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/exegesis/internal/related"
	"github.com/StevenACoffman/skillet/identity"
	"github.com/StevenACoffman/skillet/ruleset/synthesize"
	"github.com/StevenACoffman/skillet/skill"
	"github.com/StevenACoffman/skillet/testprompts"
)

// stageConstruct is the stage name for the RIA++ construction round.
const stageConstruct = "construct"

// constructTmpl is the synthesize template: its {{RULESETS}} marker is replaced
// with one block per candidate lens.
const constructTmpl = "Candidate units, one block per extraction lens:\n\n" + synthesize.Marker

// constructSystem instructs the model to build atomic RIA skills that clear the
// exegesis gates.
const constructSystem = "You construct atomic, reusable skills from a book's candidate units. " +
	"Reply ONLY with JSON matching the schema. Each skill needs: a kebab-case slug; a description that " +
	"states the trigger condition in the third person as plain text (no angle brackets or XML), at most " +
	"1024 characters; a Markdown body with the six RIA sections — R (a source citation of at most 150 " +
	"words), I (the method in your own words), A1 (an example the author uses), A2 (when a user should " +
	"reach for this), E (1-2-3 executable steps), and B (when it does NOT apply); optional related edges " +
	"to other skills (kind depends-on, contrasts-with, or composes-with); and 5-10 test prompts with at " +
	"least 3 should_trigger, 2 should_not_trigger, and 1 edge_case."

// constructSchema is the JSON Schema the construct reply must satisfy.
const constructSchema = `{
  "type": "object",
  "required": ["skills"],
  "properties": {
    "skills": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["slug", "description", "body", "test_prompts"],
        "properties": {
          "slug": {"type": "string"},
          "description": {"type": "string"},
          "body": {"type": "string"},
          "related": {"type": "array", "items": {"type": "object", "properties": {
            "kind": {"type": "string", "enum": ["depends-on", "contrasts-with", "composes-with"]},
            "target": {"type": "string"},
            "rationale": {"type": "string"}
          }}},
          "test_prompts": {"type": "array", "minItems": 5, "maxItems": 10, "items": {"type": "object",
            "required": ["type", "prompt", "expected"], "properties": {
            "type": {"type": "string", "enum": ["should_trigger", "should_not_trigger", "edge_case"]},
            "prompt": {"type": "string"},
            "expected": {"type": "string"}
          }}}
        }
      }
    }
  }
}`

// RelatedSpec is one related-skill edge in a construct reply.
type RelatedSpec struct {
	Kind      string `json:"kind"`
	Target    string `json:"target"`
	Rationale string `json:"rationale"`
}

// TestPromptSpec is one test prompt in a construct reply.
type TestPromptSpec struct {
	Type     string `json:"type"`
	Prompt   string `json:"prompt"`
	Expected string `json:"expected"`
}

// SkillSpec is one constructed skill: its slug, description, RIA body, related
// edges, and test prompts.
type SkillSpec struct {
	Slug        string           `json:"slug"`
	Description string           `json:"description"`
	Body        string           `json:"body"`
	Related     []RelatedSpec    `json:"related"`
	TestPrompts []TestPromptSpec `json:"test_prompts"`
}

// Stage2Response is the agent's construct reply.
type Stage2Response struct {
	Skills []SkillSpec `json:"skills"`
}

// constructPrompt assembles the candidate inputs into one construct prompt. It
// offloads the assembly to synthesize.FillTemplate (which validates the marker
// and non-empty inputs) and folds overview+assembled into the content-address.
func constructPrompt(overviewText string, inputs []synthesize.Input) (PromptRequest, error) {
	assembled, err := synthesize.FillTemplate(constructTmpl, inputs)
	if err != nil {
		return PromptRequest{}, fmt.Errorf("assemble candidates: %w", err)
	}
	user := "Overview:\n\n" + overviewText + "\n\n" + assembled
	id := identity.Hash(stageConstruct + "\x00" + overviewText + "\x00" + assembled)
	return PromptRequest{
		ID: id,
		Messages: []Message{
			{Role: "system", Content: constructSystem},
			{Role: "user", Content: user},
		},
		Schema: json.RawMessage(constructSchema),
	}, nil
}

// renderSkill renders spec into SKILL.md text: frontmatter (name == slug so it
// matches the folder), the RIA body, and a "## Related skills" section built from
// related.Bullet for each known-kind edge. Pure; spec is not mutated.
func renderSkill(spec *SkillSpec) string {
	slug := skill.Slug(spec.Slug)
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: %q\n---\n\n", slug, spec.Description)
	b.WriteString(strings.TrimSpace(spec.Body) + "\n")
	if edges := validEdges(spec.Related); len(edges) > 0 {
		b.WriteString("\n## Related skills\n\n")
		for i := range edges {
			b.WriteString(related.Bullet(edges[i]) + "\n")
		}
	}
	return b.String()
}

// validEdges maps the reply's edges to related.Edge, dropping unknown kinds and
// normalizing targets to slugs.
func validEdges(specs []RelatedSpec) []related.Edge {
	var out []related.Edge
	for _, s := range specs {
		e := related.Edge{
			Kind:      related.Kind(s.Kind),
			Target:    skill.Slug(s.Target),
			Rationale: s.Rationale,
		}
		if e.Kind.Valid() && e.Target != "" {
			out = append(out, e)
		}
	}
	return out
}

// buildTestFile turns spec's test prompts into a *testprompts.File with 1-based ids.
func buildTestFile(spec *SkillSpec) *testprompts.File {
	f := &testprompts.File{Skill: skill.Slug(spec.Slug)}
	for i := range spec.TestPrompts {
		tp := &spec.TestPrompts[i]
		f.Tests = append(f.Tests, testprompts.Case{
			ID: i + 1, Type: tp.Type, Prompt: tp.Prompt, Expected: tp.Expected,
		})
	}
	return f
}

// stage2Step is the construct step: it emits the assembled construct prompt, and
// on the answer writes each skill's SKILL.md and test-prompts.json. It returns
// nil once the prompt is answered (the skills are written).
func stage2Step(pc *pipeline) ([]PromptRequest, error) {
	overviewText, err := os.ReadFile(filepath.Join(pc.tree, overviewFile))
	if err != nil {
		return nil, fmt.Errorf("read overview: %w", err)
	}
	inputs, err := candidateInputs(pc.tree)
	if err != nil {
		return nil, err
	}
	p, err := constructPrompt(string(overviewText), inputs)
	if err != nil {
		return nil, err
	}
	p.ResponsePath = pc.cache.Path(p.ID)
	if !pc.cache.Has(p.ID) {
		return []PromptRequest{p}, nil
	}
	r, err := decode[Stage2Response](pc.cache, p.ID, stageConstruct)
	if err != nil {
		return nil, err
	}
	return nil, writeSkills(pc.tree, r.Skills)
}

// candidateInputs reads the five candidate files (in extractor order) into
// synthesize inputs.
func candidateInputs(tree string) ([]synthesize.Input, error) {
	var inputs []synthesize.Input
	for _, e := range extractors() {
		b, err := os.ReadFile(filepath.Join(tree, candidatesDir, e.typ+".md"))
		if err != nil {
			return nil, fmt.Errorf("read candidates %s: %w", e.typ, err)
		}
		inputs = append(inputs, synthesize.Input{Title: e.typ, Body: string(b)})
	}
	return inputs, nil
}

// writeSkills writes each skill's SKILL.md and test-prompts.json under its slug
// directory, leaving any already-written file untouched.
func writeSkills(tree string, skills []SkillSpec) error {
	for i := range skills {
		s := &skills[i]
		slug := skill.Slug(s.Slug)
		if slug == "" {
			continue
		}
		dir := filepath.Join(tree, slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create skill dir %s: %w", slug, err)
		}
		if err := writeIfAbsent(filepath.Join(dir, "SKILL.md"), renderSkill(s)); err != nil {
			return err
		}
		tpPath := filepath.Join(dir, "test-prompts.json")
		if _, statErr := os.Stat(tpPath); statErr != nil {
			if err := testprompts.Write(tpPath, buildTestFile(s)); err != nil {
				return fmt.Errorf("write test-prompts %s: %w", slug, err)
			}
		}
	}
	return nil
}

// writeIfAbsent writes content to path only when the file does not already exist.
func writeIfAbsent(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return writeArtifact(path, content)
}
