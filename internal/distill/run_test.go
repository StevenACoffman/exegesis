package distill_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/distill"
	"github.com/StevenACoffman/exegesis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/skill"
	"github.com/StevenACoffman/skillet/testprompts"
)

var extractorTypes = []string{"frameworks", "principles", "cases", "counter-examples", "glossary"}

func validResponse() *distill.Stage0Response {
	return &distill.Stage0Response{
		Title:          "Demo Book",
		Author:         "A. Author",
		Summary:        "The book argues one clear thing.",
		Skeleton:       []string{"one", "two", "three"},
		KeyTerms:       []string{"a", "b", "c", "d", "e"},
		EraLimitations: []string{"dated"},
		BlindSpots:     []string{"blind"},
		Assumptions:    []string{"unproven"},
	}
}

func sparseResponse() *distill.Stage0Response {
	r := validResponse()
	r.Skeleton = []string{"only one", "only two"} // fails the 3-7 skeleton bound
	return r
}

func extractResponse() *distill.Stage1Response {
	return &distill.Stage1Response{Units: []distill.CandidateUnit{
		{
			ID:          "u1",
			Title:       "Unit One",
			Type:        "framework",
			SourceQuote: "quote",
			Body:        "in my own words",
		},
	}}
}

// sixPrompts is a gate-passing test-prompt set: 3 should_trigger, 2
// should_not_trigger, 1 edge_case.
func sixPrompts() []distill.TestPromptSpec {
	return []distill.TestPromptSpec{
		{Type: "should_trigger", Prompt: "p1", Expected: "e1"},
		{Type: "should_trigger", Prompt: "p2", Expected: "e2"},
		{Type: "should_trigger", Prompt: "p3", Expected: "e3"},
		{Type: "should_not_trigger", Prompt: "p4", Expected: "e4"},
		{Type: "should_not_trigger", Prompt: "p5", Expected: "e5"},
		{Type: "edge_case", Prompt: "p6", Expected: "e6"},
	}
}

// cleanSkill is a SkillSpec whose rendered SKILL.md passes lint.Check.
func cleanSkill(slug string) distill.SkillSpec {
	return distill.SkillSpec{
		Slug: slug,
		Description: "Invoke when a plan looks obviously correct and it should be " +
			"stress-tested from the opposite direction.",
		Body: "## R\n\nquote\n\n## I\n\nmethod in own words\n\n## A1\n\nauthor example\n\n" +
			"## A2\n\nwhen to use\n\n## E\n\n1. step 2. step 3. step\n\n## B\n\nwhen it does not apply",
		TestPrompts: sixPrompts(),
	}
}

func constructResponse() *distill.Stage2Response {
	a := cleanSkill("reverse-thinking")
	a.Related = []distill.RelatedSpec{
		{Kind: "depends-on", Target: "first-principles", Rationale: "builds on base reasoning"},
	}
	return &distill.Stage2Response{Skills: []distill.SkillSpec{a, cleanSkill("first-principles")}}
}

// writeJSON marshals v and writes it to path, simulating the agent answering a
// prompt at its response_path.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

func writeBook(t *testing.T) (tree, bookPath string) {
	t.Helper()
	root := t.TempDir()
	tree = filepath.Join(root, "books", "demo")
	bookPath = filepath.Join(root, "book.txt")
	if err := os.WriteFile(bookPath, []byte("Some book text."), 0o644); err != nil {
		t.Fatalf("write book: %v", err)
	}
	return tree, bookPath
}

func mustRun(t *testing.T, tree, bookPath string) distill.Outcome {
	t.Helper()
	o, err := distill.Run(tree, bookPath, "resume")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return o
}

// advanceToConstruct answers the overview and extract rounds and returns the
// construct-stage Outcome.
func advanceToConstruct(t *testing.T, tree, bookPath string) distill.Outcome {
	t.Helper()
	o := mustRun(t, tree, bookPath)
	writeJSON(t, o.Prompts[0].ResponsePath, validResponse())
	o = mustRun(t, tree, bookPath)
	for _, p := range o.Prompts {
		writeJSON(t, p.ResponsePath, extractResponse())
	}
	return mustRun(t, tree, bookPath)
}

func TestRunAdvancesThroughStages(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)

	construct := advanceToConstruct(t, tree, bookPath)
	if construct.Status != distill.StatusNeedsPrompts || construct.Stage != "construct" ||
		len(construct.Prompts) != 1 {
		t.Fatalf("want one construct prompt, got %+v", construct)
	}
	writeJSON(t, construct.Prompts[0].ResponsePath, constructResponse())

	final := mustRun(t, tree, bookPath)
	if final.Status != distill.StatusComplete {
		t.Fatalf("want complete, got %+v", final)
	}
	for _, slug := range []string{"reverse-thinking", "first-principles"} {
		for _, name := range []string{"SKILL.md", "test-prompts.json"} {
			if _, err := os.Stat(filepath.Join(tree, slug, name)); err != nil {
				t.Errorf("expected %s/%s: %v", slug, name, err)
			}
		}
	}
	// Stage 3 wrote INDEX.md linking the two constructed skills.
	index, err := os.ReadFile(filepath.Join(tree, "INDEX.md"))
	if err != nil {
		t.Fatalf("expected INDEX.md: %v", err)
	}
	if !strings.Contains(string(index), "reverse-thinking -->|depends-on| first-principles") {
		t.Errorf("INDEX.md missing the depends-on edge:\n%s", index)
	}
}

func TestConstructOutputPassesGates(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)
	construct := advanceToConstruct(t, tree, bookPath)
	writeJSON(t, construct.Prompts[0].ResponsePath, constructResponse())
	mustRun(t, tree, bookPath) // completes, writing the skills

	dir := filepath.Join(tree, "reverse-thinking")
	s, err := skill.Load(dir)
	if err != nil {
		t.Fatalf("load generated skill: %v", err)
	}
	for _, f := range lint.Check(s, lint.Options{}) {
		if f.Severity == finding.SeverityError {
			t.Errorf("generated SKILL.md must pass lint, got: %s", f.Message)
		}
	}
	f, err := testprompts.Load(filepath.Join(dir, "test-prompts.json"))
	if err != nil {
		t.Fatalf("load generated test-prompts: %v", err)
	}
	if problems := f.Validate(); len(problems) != 0 {
		t.Errorf("generated test-prompts must pass composition gate, got %v", problems)
	}
}

func TestRunRePromptsOnOverviewGateFailure(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)

	first := mustRun(t, tree, bookPath)
	writeJSON(t, first.Prompts[0].ResponsePath, sparseResponse())

	second := mustRun(t, tree, bookPath)
	if second.Status != distill.StatusNeedsPrompts || second.Stage != "overview" {
		t.Fatalf("a gate-failing answer should re-prompt the overview, got %+v", second)
	}
	if second.Prompts[0].ID == first.Prompts[0].ID {
		t.Error("the correction prompt must have a distinct content-address")
	}
	if _, err := os.Stat(filepath.Join(tree, "BOOK_OVERVIEW.md")); err == nil {
		t.Error("a failing overview must not be written")
	}

	writeJSON(t, second.Prompts[0].ResponsePath, validResponse())
	third := mustRun(t, tree, bookPath)
	if third.Status != distill.StatusNeedsPrompts || third.Stage != "extract" {
		t.Errorf("want the extract stage after a valid overview, got %+v", third)
	}
}

func TestRunExtractPartialBatch(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)
	first := mustRun(t, tree, bookPath)
	writeJSON(t, first.Prompts[0].ResponsePath, validResponse())
	batch := mustRun(t, tree, bookPath)

	// Answer only the first extractor; the rest must still be pending.
	writeJSON(t, batch.Prompts[0].ResponsePath, extractResponse())
	again := mustRun(t, tree, bookPath)
	if again.Status != distill.StatusNeedsPrompts || again.Stage != "extract" {
		t.Fatalf("want still-pending extract prompts, got %+v", again)
	}
	if len(again.Prompts) != len(extractorTypes)-1 {
		t.Errorf("want %d remaining prompts, got %d", len(extractorTypes)-1, len(again.Prompts))
	}
}
