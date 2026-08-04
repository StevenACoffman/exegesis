package distill_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/distill"
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

func TestRunAdvancesThroughStages(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)

	// Round 1: the overview prompt.
	o1, err := distill.Run(tree, bookPath, "resume")
	if err != nil {
		t.Fatalf("round 1: %v", err)
	}
	if o1.Status != distill.StatusNeedsPrompts || o1.Stage != "overview" || len(o1.Prompts) != 1 {
		t.Fatalf("want one overview prompt, got %+v", o1)
	}
	writeJSON(t, o1.Prompts[0].ResponsePath, validResponse())

	// Round 2: the extract batch (one prompt per extractor).
	o2, err := distill.Run(tree, bookPath, "resume")
	if err != nil {
		t.Fatalf("round 2: %v", err)
	}
	if o2.Status != distill.StatusNeedsPrompts || o2.Stage != "extract" {
		t.Fatalf("want the extract batch, got %+v", o2)
	}
	if len(o2.Prompts) != len(extractorTypes) {
		t.Fatalf("want %d extractor prompts, got %d", len(extractorTypes), len(o2.Prompts))
	}
	for _, p := range o2.Prompts {
		writeJSON(t, p.ResponsePath, extractResponse())
	}

	// Round 3: complete, with every candidate file written.
	o3, err := distill.Run(tree, bookPath, "resume")
	if err != nil {
		t.Fatalf("round 3: %v", err)
	}
	if o3.Status != distill.StatusComplete {
		t.Fatalf("want complete, got %+v", o3)
	}
	for _, typ := range extractorTypes {
		path := filepath.Join(tree, "candidates", typ+".md")
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("expected candidate file %s: %v", path, statErr)
		}
	}
}

func TestRunRePromptsOnOverviewGateFailure(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)

	first, err := distill.Run(tree, bookPath, "resume")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	writeJSON(t, first.Prompts[0].ResponsePath, sparseResponse())

	second, err := distill.Run(tree, bookPath, "resume")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second.Status != distill.StatusNeedsPrompts || second.Stage != "overview" {
		t.Fatalf("a gate-failing answer should re-prompt the overview, got %+v", second)
	}
	if second.Prompts[0].ID == first.Prompts[0].ID {
		t.Error("the correction prompt must have a distinct content-address")
	}
	if _, err := os.Stat(filepath.Join(tree, "BOOK_OVERVIEW.md")); err == nil {
		t.Error("a failing overview must not be written")
	}

	// A valid correction advances past Stage 0 into the extract stage.
	writeJSON(t, second.Prompts[0].ResponsePath, validResponse())
	third, err := distill.Run(tree, bookPath, "resume")
	if err != nil {
		t.Fatalf("third run: %v", err)
	}
	if third.Status != distill.StatusNeedsPrompts || third.Stage != "extract" {
		t.Errorf("want the extract stage after a valid overview, got %+v", third)
	}
}

func TestRunExtractPartialBatch(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)
	first, err := distill.Run(tree, bookPath, "resume")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	writeJSON(t, first.Prompts[0].ResponsePath, validResponse())
	batch, err := distill.Run(tree, bookPath, "resume")
	if err != nil {
		t.Fatalf("extract round: %v", err)
	}

	// Answer only the first extractor; the rest must still be pending.
	writeJSON(t, batch.Prompts[0].ResponsePath, extractResponse())
	again, err := distill.Run(tree, bookPath, "resume")
	if err != nil {
		t.Fatalf("partial extract round: %v", err)
	}
	if again.Status != distill.StatusNeedsPrompts || again.Stage != "extract" {
		t.Fatalf("want still-pending extract prompts, got %+v", again)
	}
	if len(again.Prompts) != len(extractorTypes)-1 {
		t.Errorf("want %d remaining prompts, got %d", len(extractorTypes)-1, len(again.Prompts))
	}
}
