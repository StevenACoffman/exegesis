package cmd_test

import (
	"path/filepath"
	"strings"
	"testing"
)

const actLine = "semantic replay not performed"

// TestVerifyStatesTheActOnEveryGatePath pins that the statement survives gate
// selection. The line is about the run, not about the skills gate, so a --gates
// value that skips the skills path must still say what was not performed --
// otherwise the narrower the run, the less it admits to skipping.
func TestVerifyStatesTheActOnEveryGatePath(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeNamedSkill(t, tree, "skilla")
	if _, err := run(t, "tests", "--scaffold", filepath.Join(tree, "skilla")); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	for _, args := range [][]string{
		{"verify", tree},
		{"verify", "--gates", "skills", tree},
		{"verify", "--gates", "overview", tree},
	} {
		// The verdict is deliberately not asserted: --gates overview fails on a tree
		// with no BOOK_OVERVIEW.md, and the statement must hold either way. Asserting
		// only the statement is the stronger check, since a run is most tempted to
		// stay quiet about what it skipped when it is already reporting a problem.
		out, _ := run(t, args...)
		if !strings.Contains(out, actLine) {
			t.Errorf("%v: expected the act statement, got:\n%s", args, out)
		}
		if n := strings.Count(out, actLine); n != 1 {
			t.Errorf("%v: expected the act stated once, got %d times", args, n)
		}
	}
}

// TestVerifyStatesTheActOnFailure keeps the statement on the path where mistaking a
// structural verdict for a quality one is most consequential: a reader looking at a
// failure needs to know the failure is structural too.
func TestVerifyStatesTheActOnFailure(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeNamedSkill(t, tree, "skilla") // no scaffold: test-prompts.json missing
	out, err := run(t, "verify", tree)
	if err == nil {
		t.Fatalf("expected a failure for the missing test-prompts.json, got:\n%s", out)
	}
	if !strings.Contains(out, actLine) {
		t.Errorf("expected the act statement on a failing run, got:\n%s", out)
	}
}
