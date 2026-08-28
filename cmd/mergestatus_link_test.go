package cmd_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd/root"
)

// TestMergeStatusLinkWritesBothRecords drives the shape merge-skills prescribes: one call
// records what the run decided about a source skill and the pointer a reader of that dead
// skill needs, so the two cannot be written apart and drift.
func TestMergeStatusLinkWritesBothRecords(t *testing.T) {
	t.Parallel()
	books := t.TempDir()
	book := filepath.Join(books, "some-book")
	source := writeNamedSkill(t, book, "source-skill")
	writeNamedSkill(t, filepath.Join(books, "merged", "all-books-v1"), "combined")

	out, err := run(t, "merge-status", "append", "--run", "all-books-v1",
		"--state", "merged", "--into", "combined", "--link", source)
	if err != nil {
		t.Fatalf("append --link: %v\n%s", err, out)
	}
	if strings.Contains(out, "warning") {
		t.Errorf("the merged skill exists in the sibling tree, so nothing should warn:\n%s", out)
	}

	got := readFileString(t, filepath.Join(source, "SKILL.md"))
	for _, want := range []string{
		"## Merge Status",
		"run: all-books-v1",
		"state: merged",
		"into: combined",
		"- superseded-by: `merged/all-books-v1/combined` — " +
			"superseded by the all-books-v1 merge run",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the skill, got:\n%s", want, got)
		}
	}
	// The ledger keeps the bare slug it has always written; only the edge is qualified.
	if strings.Contains(got, "into: merged/all-books-v1/combined") {
		t.Errorf("the ledger's into: must stay a bare slug, got:\n%s", got)
	}

	// The tree-qualified target must not read as dangling to the graph gate.
	out, _ = run(t, "verify", book)
	if strings.Contains(out, "graph:") {
		t.Errorf("a qualified target was reported as dangling:\n%s", out)
	}

	// merge-status check must still accept the ledger it just wrote.
	if out, err = run(t, "merge-status", "check", book); err != nil {
		t.Errorf("the written ledger does not validate: %v\n%s", err, out)
	}
}

// TestMergeStatusLinkIsIdempotentOnTheEdgeOnly pins the asymmetry between the two
// records: the ledger is an audit trail and records every run, the edge is a pointer and
// there is only one place to point.
func TestMergeStatusLinkIsIdempotentOnTheEdgeOnly(t *testing.T) {
	t.Parallel()
	books := t.TempDir()
	book := filepath.Join(books, "some-book")
	source := writeNamedSkill(t, book, "source-skill")
	writeNamedSkill(t, filepath.Join(books, "merged", "all-books-v1"), "combined")

	var last string
	for range 2 {
		out, err := run(t, "merge-status", "append", "--run", "all-books-v1",
			"--state", "merged", "--into", "combined", "--link", source)
		if err != nil {
			t.Fatalf("append --link: %v\n%s", err, out)
		}
		last = out
	}
	// The second run must not claim to have added an edge that was already there.
	if !strings.Contains(last, "already linked superseded-by") {
		t.Errorf("expected the repeat to report the edge as already present, got:\n%s", last)
	}

	got := readFileString(t, filepath.Join(source, "SKILL.md"))
	if n := strings.Count(got, "- run: all-books-v1"); n != 2 {
		t.Errorf("expected 2 ledger entries, got %d:\n%s", n, got)
	}
	if n := strings.Count(got, "- superseded-by:"); n != 1 {
		t.Errorf("expected exactly 1 edge, got %d:\n%s", n, got)
	}
}

func TestMergeStatusLinkWarnsWhenTheMergedSkillIsAbsent(t *testing.T) {
	t.Parallel()
	books := t.TempDir()
	source := writeNamedSkill(t, filepath.Join(books, "some-book"), "source-skill")

	out, err := run(t, "merge-status", "append", "--run", "all-books-v1",
		"--state", "merged", "--into", "absent", "--link", source)
	if err != nil {
		t.Fatalf("an unresolved target warns rather than failing: %v\n%s", err, out)
	}
	if !strings.Contains(out, `no skill "merged/all-books-v1/absent"`) {
		t.Errorf("expected a warning naming the unresolved target, got:\n%s", out)
	}
	// The entry is still recorded: what a run decided is true whether or not the merged
	// skill is on this disk.
	if got := readFileString(t, filepath.Join(source, "SKILL.md")); !strings.Contains(
		got, "state: merged",
	) {
		t.Errorf("the ledger entry must be written anyway, got:\n%s", got)
	}
}

func TestMergeStatusLinkNeedsAStateThatMergesSomething(t *testing.T) {
	t.Parallel()
	books := t.TempDir()
	source := writeNamedSkill(t, filepath.Join(books, "some-book"), "source-skill")

	_, err := run(t, "merge-status", "append", "--run", "all-books-v1",
		"--state", "no-candidate", "--link", source)
	var usage root.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("expected a usage error for --link on no-candidate, got %v", err)
	}
	if !strings.Contains(err.Error(), "merged, partial") {
		t.Errorf("the message must name the states that do take --into, got: %v", err)
	}
	// Nothing was written: the usage error is raised before the file is read.
	if got := readFileString(t, filepath.Join(source, "SKILL.md")); strings.Contains(
		got, "Merge Status",
	) {
		t.Errorf("a rejected call must not write a ledger, got:\n%s", got)
	}
}
