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

// validExtract is a JSON extractor reply.
const validExtract = `{"units": [{"title": "Unit One", "type": "framework", "body": "in my own words"}]}`

// validConstruct is a JSON construct reply with one gate-passing skill.
const validConstruct = `{"skills": [{
  "slug": "reverse-thinking",
  "description": "Invoke when a plan looks obviously correct and it should be stress-tested in reverse.",
  "body": "## R\n\nquote\n\n## I\n\nmethod\n\n## A1\n\nexample\n\n## A2\n\nwhen\n\n## E\n\n1. a 2. b 3. c\n\n## B\n\nnot applicable when",
  "test_prompts": [
    {"type": "should_trigger", "prompt": "p1", "expected": "e1"},
    {"type": "should_trigger", "prompt": "p2", "expected": "e2"},
    {"type": "should_trigger", "prompt": "p3", "expected": "e3"},
    {"type": "should_not_trigger", "prompt": "p4", "expected": "e4"},
    {"type": "should_not_trigger", "prompt": "p5", "expected": "e5"},
    {"type": "edge_case", "prompt": "p6", "expected": "e6"}
  ]
}]}`

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

func TestDistillAgentLoop(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	book := filepath.Join(dir, "book.txt")
	if err := os.WriteFile(book, []byte("Some book text."), 0o644); err != nil {
		t.Fatalf("write book: %v", err)
	}
	out := filepath.Join(dir, "books")
	args := distillArgs(book, out)

	// Round 1: the overview prompt.
	o1 := runRound(t, args)
	if o1.Status != "needs_prompts" || o1.Stage != "overview" || len(o1.Prompts) != 1 {
		t.Fatalf("want one overview prompt, got %+v", o1)
	}
	if !strings.Contains(o1.Resume, "distill") {
		t.Errorf("resume should be the re-run command, got %q", o1.Resume)
	}
	answerAll(t, o1, validStage0)

	// Round 2: the extract batch.
	o2 := runRound(t, args)
	if o2.Status != "needs_prompts" || o2.Stage != "extract" || len(o2.Prompts) != 5 {
		t.Fatalf("want the 5-prompt extract batch, got %+v", o2)
	}
	answerAll(t, o2, validExtract)

	// Round 3: the construct prompt.
	o3 := runRound(t, args)
	if o3.Status != "needs_prompts" || o3.Stage != "construct" || len(o3.Prompts) != 1 {
		t.Fatalf("want one construct prompt, got %+v", o3)
	}
	answerAll(t, o3, validConstruct)

	// Round 4: complete, with the overview, candidates, and a skill written.
	o4 := runRound(t, args)
	if o4.Status != "complete" {
		t.Fatalf("want complete, got %+v", o4)
	}
	tree := filepath.Join(out, "demo-book")
	assertOverviewValid(t, tree)
	for _, p := range []string{"candidates/frameworks.md", "reverse-thinking/SKILL.md", "reverse-thinking/test-prompts.json"} {
		if _, statErr := os.Stat(filepath.Join(tree, p)); statErr != nil {
			t.Errorf("expected %s: %v", p, statErr)
		}
	}
}

// runRound runs the args through cmd.Run and returns the parsed Outcome.
func runRound(t *testing.T, args []string) distillOutcome {
	t.Helper()
	out, err := run(t, args...)
	if err != nil {
		t.Fatalf("run %v: %v\n%s", args, err, out)
	}
	var o distillOutcome
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("outcome is not JSON: %v\n%s", err, out)
	}
	return o
}

// answerAll writes body to every prompt's response_path.
func answerAll(t *testing.T, o distillOutcome, body string) {
	t.Helper()
	for _, p := range o.Prompts {
		if err := os.WriteFile(p.ResponsePath, []byte(body), 0o644); err != nil {
			t.Fatalf("answer prompt: %v", err)
		}
	}
}

// assertOverviewValid reads tree/BOOK_OVERVIEW.md and checks it passes the gate.
func assertOverviewValid(t *testing.T, tree string) {
	t.Helper()
	md, err := os.ReadFile(filepath.Join(tree, "BOOK_OVERVIEW.md"))
	if err != nil {
		t.Fatalf("expected the overview to be written: %v", err)
	}
	if problems := overview.Check(string(md)); len(problems) != 0 {
		t.Errorf("overview should pass the Stage-0 gate, got %v", problems)
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
