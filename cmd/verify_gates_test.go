package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd/root"
)

// validOverview passes overview.Check: a non-empty summary, 3 skeleton items,
// 5 key terms, and 3 critique items.
const validOverview = `# Demo Book

## One-sentence summary

The book argues one clear thing.

## Skeleton

- part one
- part two
- part three

## Key terms

- term a
- term b
- term c
- term d
- term e

## Era limitations

- dated example

## Author blind spots

- missing perspective

## Unproven assumptions

- unverified claim
`

// invalidOverview fails the gate on the skeleton count (2, want 3-7).
const invalidOverview = `## One-sentence summary

A summary.

## Skeleton

- only one
- only two

## Key terms

- a
- b
- c
- d
- e
`

func writeOverview(t *testing.T, tree, content string) {
	t.Helper()
	if err := os.WriteFile(
		filepath.Join(tree, "BOOK_OVERVIEW.md"),
		[]byte(content),
		0o644,
	); err != nil {
		t.Fatalf("write overview: %v", err)
	}
}

func manifestExists(tree string) bool {
	_, err := os.Stat(filepath.Join(tree, "skills-manifest.json"))
	return err == nil
}

func TestVerifyGatesOverviewPasses(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeOverview(t, tree, validOverview)

	out, err := run(t, "verify", "--gates", "overview", tree)
	if err != nil {
		t.Fatalf("verify --gates overview: %v\n%s", err, out)
	}
	if !strings.Contains(out, "BOOK_OVERVIEW.md: ok") {
		t.Errorf("expected an ok line, got:\n%s", out)
	}
	if manifestExists(tree) {
		t.Error("an overview-only run must not write skills-manifest.json")
	}
}

func TestVerifyGatesOverviewFailsOnBadOverview(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeOverview(t, tree, invalidOverview)

	out, err := run(t, "verify", "--gates", "overview", tree)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if !strings.Contains(out, "skeleton has 2") {
		t.Errorf("expected the skeleton problem, got:\n%s", out)
	}
	if manifestExists(tree) {
		t.Error("a failed overview-only run must not write a manifest")
	}
}

func TestVerifyGatesOverviewMissingFileFails(t *testing.T) {
	t.Parallel()
	tree := t.TempDir() // no BOOK_OVERVIEW.md

	out, err := run(t, "verify", "--gates", "overview", tree)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected ExitError for a missing overview, got %v", err)
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected a 'not found' line, got:\n%s", out)
	}
}

func TestVerifyGatesSkillsSkipsOverview(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeOverview(t, tree, invalidOverview) // bad, but skills gate must ignore it
	skillDir := filepath.Join(tree, "skilla")
	writeSkill(t, skillDir)
	if _, err := run(t, "tests", "--scaffold", skillDir); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	out, err := run(t, "verify", "--gates", "skills", tree)
	if err != nil {
		t.Fatalf("verify --gates skills should ignore the bad overview: %v\n%s", err, out)
	}
	if strings.Contains(out, "BOOK_OVERVIEW.md") {
		t.Errorf("skills gate must not read the overview, got:\n%s", out)
	}
	if !manifestExists(tree) {
		t.Error("a skills run must write skills-manifest.json")
	}
}

func TestVerifyDefaultFoldsOverviewIntoResult(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeOverview(t, tree, invalidOverview)
	skillDir := filepath.Join(tree, "skilla")
	writeSkill(t, skillDir)
	if _, err := run(t, "tests", "--scaffold", skillDir); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	// Default run (no --gates): a bad overview must fail the whole verify even
	// though the skill itself is fine.
	out, err := run(t, "verify", tree)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected a failing default verify, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "structure_verified=false") {
		t.Errorf("manifest should record structure_verified=false, got:\n%s", out)
	}
}

func TestVerifyUnknownGate(t *testing.T) {
	t.Parallel()
	_, err := run(t, "verify", "--gates", "bogus", t.TempDir())
	if err == nil {
		t.Fatal("expected an error for an unknown gate")
	}
	if !strings.Contains(err.Error(), "unknown gate") {
		t.Errorf("error should name the problem, got: %v", err)
	}
}
