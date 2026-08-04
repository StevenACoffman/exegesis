package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/overview"
)

// validStage0 is a JSON overview reply that passes the Stage-0 gate.
const validStage0 = `{
  "title": "Demo Book",
  "summary": "The book argues one clear thing.",
  "skeleton": ["one", "two", "three"],
  "key_terms": ["a", "b", "c", "d", "e"],
  "era_limitations": ["dated"],
  "author_blind_spots": ["blind"],
  "unproven_assumptions": ["unproven"]
}`

// distillOutcome mirrors the fields of the JSON distill prints that the tests
// assert on.
type distillOutcome struct {
	Status  string `json:"status"`
	Stage   string `json:"stage"`
	Prompts []struct {
		ID           string `json:"id"`
		ResponsePath string `json:"response_path"`
	} `json:"prompts"`
	Resume string `json:"resume"`
}

func distillArgs(book, out string) []string {
	return []string{"distill", "--driver", "agent", "--title", "Demo Book", "--out", out, book}
}

func TestDistillAgentLoopStage0(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	book := filepath.Join(dir, "book.txt")
	if err := os.WriteFile(book, []byte("Some book text."), 0o644); err != nil {
		t.Fatalf("write book: %v", err)
	}
	out := filepath.Join(dir, "books")
	args := distillArgs(book, out)

	// Round 1: distill emits the overview prompt.
	s1, err := run(t, args...)
	if err != nil {
		t.Fatalf("round 1: %v\n%s", err, s1)
	}
	var o1 distillOutcome
	if err := json.Unmarshal([]byte(s1), &o1); err != nil {
		t.Fatalf("round 1 output is not JSON: %v\n%s", err, s1)
	}
	if o1.Status != "needs_prompts" || o1.Stage != "overview" || len(o1.Prompts) != 1 {
		t.Fatalf("want one overview prompt, got %+v", o1)
	}
	if !strings.Contains(o1.Resume, "distill") {
		t.Errorf("resume should be the re-run command, got %q", o1.Resume)
	}

	// The agent answers the prompt at its response_path.
	if err := os.WriteFile(o1.Prompts[0].ResponsePath, []byte(validStage0), 0o644); err != nil {
		t.Fatalf("answer prompt: %v", err)
	}

	// Round 2: distill completes and the gated overview exists.
	s2, err := run(t, args...)
	if err != nil {
		t.Fatalf("round 2: %v\n%s", err, s2)
	}
	var o2 distillOutcome
	if err := json.Unmarshal([]byte(s2), &o2); err != nil {
		t.Fatalf("round 2 output is not JSON: %v\n%s", err, s2)
	}
	if o2.Status != "complete" {
		t.Fatalf("want complete, got %+v", o2)
	}
	overviewPath := filepath.Join(out, "demo-book", "BOOK_OVERVIEW.md")
	md, readErr := os.ReadFile(overviewPath)
	if readErr != nil {
		t.Fatalf("expected %s to be written: %v", overviewPath, readErr)
	}
	// The generated overview is valid per the Stage-0 gate.
	if problems := overview.Check(string(md)); len(problems) != 0 {
		t.Errorf("generated overview should pass the Stage-0 gate, got %v", problems)
	}
}

func TestDistillFlagErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	book := filepath.Join(dir, "book.txt")
	if err := os.WriteFile(book, []byte("x"), 0o644); err != nil {
		t.Fatalf("write book: %v", err)
	}
	cases := map[string]struct {
		args    []string
		wantSub string
	}{
		"missing title": {[]string{"distill", book}, "--title is required"},
		"missing book":  {[]string{"distill", "--title", "X"}, "need exactly one book file"},
		"http unsupported": {
			[]string{"distill", "--driver", "http", "--title", "X", book},
			"not yet implemented",
		},
		"unknown driver": {
			[]string{"distill", "--driver", "bogus", "--title", "X", book},
			"unknown --driver",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := run(t, tc.args...)
			if err == nil {
				t.Fatalf("expected an error for %q", name)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error should contain %q, got: %v", tc.wantSub, err)
			}
		})
	}
}
