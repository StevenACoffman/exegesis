package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd/root"
)

// mergedSkill is the frontmatter shape the real merged tree has: five keys the spec
// calls unknown fields, and no `name`.
const mergedSkill = `---
id: merged-thing
title: Merged Thing
description: Use this skill when the user needs the merged thing done in a particular way.
type: merged-skill
source_skills:
  - slug: book-a/source-one
    book: "Book A"
    author: Author A
related_skills:
  - slug: book-a/source-one
    relation: supersedes
    note: adds what the source lacked
tags: [x]
---

# Merged Thing

## A2 — Trigger

- "the merged skill has its own trigger phrase here"
- "and a second one nobody else states"
`

func TestMergeMigrateMakesAMergedTreeLintClean(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	dir := filepath.Join(tree, "merged-thing")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(mergedSkill), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Before: the disallowed keys are exactly what lint reports.
	out, err := run(t, "lint", dir)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected lint to fail on the pre-migration shape, got %v\n%s", err, out)
	}
	if !strings.Contains(out, `disallowed key "source_skills"`) {
		t.Errorf("expected a disallowed-key finding, got:\n%s", out)
	}

	// --check reports it without writing.
	out, err = run(t, "merge-migrate", "--check", tree)
	if !errors.As(err, &exit) {
		t.Fatalf("expected --check to exit non-zero, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "would migrate") {
		t.Errorf("expected --check to name the skill, got:\n%s", out)
	}

	if out, err = run(t, "merge-migrate", tree); err != nil {
		t.Fatalf("merge-migrate: %v\n%s", err, out)
	}
	if out, err = run(t, "lint", dir); err != nil {
		t.Errorf("a migrated skill must lint clean: %v\n%s", err, out)
	}

	got := readFileString(t, filepath.Join(dir, "SKILL.md"))
	for _, want := range []string{
		"name: merged-thing",
		"## Provenance",
		"- `book-a/source-one` — *Book A* by Author A",
		"note: adds what the source lacked",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the migrated skill, got:\n%s", want, got)
		}
	}

	// A second run changes nothing.
	if out, err = run(t, "merge-migrate", "--check", tree); err != nil {
		t.Errorf("migration is not idempotent: %v\n%s", err, out)
	}
}

func TestA2CheckReportsAndGates(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	merged := filepath.Join(tree, "merged-thing")
	if err := os.MkdirAll(merged, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(merged, "SKILL.md"),
		[]byte(mergedSkill),
		0o644,
	); err != nil {
		t.Fatalf("write: %v", err)
	}
	source := writeNamedSkill(t, tree, "source-one")
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte(
		"---\nname: source-one\ndescription: Invoke when the user needs the source thing.\n---\n"+
			"# Source\n\n## A2 — Trigger\n\n- \"the merged skill has its own trigger phrase here\"\n",
	), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// One of the merged skill's two signals is its own, so it is below the bar.
	out, err := run(t, "a2check", "--source-skill", source, merged)
	if err != nil {
		t.Fatalf("a2check should be advisory by default: %v\n%s", err, out)
	}
	if !strings.Contains(out, "1 of 2 A2 signal(s) are new") {
		t.Errorf("expected the tally, got:\n%s", out)
	}
	if !strings.Contains(out, `NEW "and a second one nobody else states"`) {
		t.Errorf("expected the new signal to be named, got:\n%s", out)
	}

	var exit root.ExitError
	if _, err = run(
		t,
		"a2check",
		"--strict",
		"--source-skill",
		source,
		merged,
	); !errors.As(
		err,
		&exit,
	) {
		t.Errorf("--strict must fail below the threshold, got %v", err)
	}
}

func TestA2CheckSaysWhenThereAreNoSignalsAtAll(t *testing.T) {
	t.Parallel()
	// 5 of the 24 checkable merged skills state no quoted signal. "0 of 0 are new"
	// would read as a sharpness failure; it is a different defect.
	tree := t.TempDir()
	merged := writeNamedSkill(t, tree, "quiet-skill")
	source := writeNamedSkill(t, tree, "source-skill")

	out, err := run(t, "a2check", "--source-skill", source, merged)
	if err != nil {
		t.Fatalf("a2check: %v\n%s", err, out)
	}
	if !strings.Contains(out, "states no A2 language signals at all") {
		t.Errorf("expected the missing-signals report, got:\n%s", out)
	}
}
