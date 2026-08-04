package distill_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/distill"
	"github.com/StevenACoffman/exegesis/internal/overview"
)

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

func answer(t *testing.T, path string, r *distill.Stage0Response) {
	t.Helper()
	b, err := json.Marshal(r)
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

func TestRunEmitsOverviewPromptThenCompletes(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)

	out, err := distill.Run(tree, bookPath, "resume-cmd")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if out.Status != distill.StatusNeedsPrompts || len(out.Prompts) != 1 {
		t.Fatalf("want one needs_prompts, got %+v", out)
	}
	if out.Stage != "overview" || out.Resume != "resume-cmd" {
		t.Errorf("stage/resume wrong: %+v", out)
	}
	if filepath.Dir(out.Prompts[0].ResponsePath) != filepath.Join(tree, ".exegesis") {
		t.Errorf("response path not under the tree cache: %s", out.Prompts[0].ResponsePath)
	}

	answer(t, out.Prompts[0].ResponsePath, validResponse())

	out2, err := distill.Run(tree, bookPath, "resume-cmd")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if out2.Status != distill.StatusComplete {
		t.Fatalf("want complete, got %+v", out2)
	}
	md, err := os.ReadFile(filepath.Join(tree, "BOOK_OVERVIEW.md"))
	if err != nil {
		t.Fatalf("overview not written: %v", err)
	}
	if p := overview.Check(string(md)); len(p) != 0 {
		t.Errorf("written overview must pass the gate, got %v", p)
	}
}

func TestRunRePromptsOnGateFailure(t *testing.T) {
	t.Parallel()
	tree, bookPath := writeBook(t)

	first, err := distill.Run(tree, bookPath, "resume-cmd")
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	answer(t, first.Prompts[0].ResponsePath, sparseResponse())

	second, err := distill.Run(tree, bookPath, "resume-cmd")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if second.Status != distill.StatusNeedsPrompts {
		t.Fatalf("a gate-failing answer should re-prompt, got %+v", second)
	}
	if second.Prompts[0].ID == first.Prompts[0].ID {
		t.Error("the correction prompt must have a distinct content-address")
	}
	if _, err := os.Stat(filepath.Join(tree, "BOOK_OVERVIEW.md")); err == nil {
		t.Error("a failing overview must not be written")
	}

	// Answering the correction with a valid response completes.
	answer(t, second.Prompts[0].ResponsePath, validResponse())
	third, err := distill.Run(tree, bookPath, "resume-cmd")
	if err != nil {
		t.Fatalf("third run: %v", err)
	}
	if third.Status != distill.StatusComplete {
		t.Errorf("want complete after correction, got %+v", third)
	}
}
