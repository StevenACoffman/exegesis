package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTreeSkill creates a skill in tree whose SKILL.md carries body.
func writeTreeSkill(t *testing.T, tree, slug, body string) {
	t.Helper()
	dir := filepath.Join(tree, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	writeAt(t, filepath.Join(dir, "SKILL.md"),
		"---\nname: "+slug+"\ndescription: Invoke when the situation applies.\n---\n\n"+body)
}

// TestDuplicatesReportsIdenticalCopiesWithoutFailing is the split the command exists to
// make: a skill installed into several trees is normal, so it is reported and exits 0.
func TestDuplicatesReportsIdenticalCopiesWithoutFailing(t *testing.T) {
	t.Parallel()
	treeA, treeB := t.TempDir(), t.TempDir()
	writeTreeSkill(t, treeA, "shared", "# Same\n")
	writeTreeSkill(t, treeB, "shared", "# Same\n")

	out, err := run(t, "duplicates", treeA, treeB)
	if err != nil {
		t.Fatalf("identical copies must not fail: %v\n%s", err, out)
	}
	if !strings.Contains(out, "shared: in 2 trees, same in every tree") {
		t.Errorf("want the duplicate reported as agreeing, got:\n%s", out)
	}
}

// TestDuplicatesFailsOnDivergence is the finding: two copies of one skill whose content
// differs mean at least one is stale.
func TestDuplicatesFailsOnDivergence(t *testing.T) {
	t.Parallel()
	treeA, treeB := t.TempDir(), t.TempDir()
	writeTreeSkill(t, treeA, "shared", "# One version\n")
	writeTreeSkill(t, treeB, "shared", "# A different version\n")

	out, err := run(t, "duplicates", treeA, treeB)
	if err == nil {
		t.Fatalf("divergence must fail, got:\n%s", out)
	}
	if !strings.Contains(out, "shared: in 2 trees, DIVERGED") {
		t.Errorf("want the divergence named, got:\n%s", out)
	}
	if !strings.Contains(out, "1 skill(s) in more than one tree, 1 diverged") {
		t.Errorf("want the summary line, got:\n%s", out)
	}
}

// TestDuplicatesReportsNothingWhenTreesShareNoSkills keeps the clean answer distinct
// from the "nothing was compared" answer below it.
func TestDuplicatesReportsNothingWhenTreesShareNoSkills(t *testing.T) {
	t.Parallel()
	treeA, treeB := t.TempDir(), t.TempDir()
	writeTreeSkill(t, treeA, "only-a", "# A\n")
	writeTreeSkill(t, treeB, "only-b", "# B\n")

	out, err := run(t, "duplicates", treeA, treeB)
	if err != nil {
		t.Fatalf("disjoint trees must not fail: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no skill appears in more than one of the 2 trees") {
		t.Errorf("want the empty result stated, got:\n%s", out)
	}
}

// TestDuplicatesReportsATreeWithNoSkills is the wrong-path guard: a tree contributing
// nothing cannot duplicate anything, so a silent run would answer "no duplicates" when
// the truth is "I did not look".
func TestDuplicatesReportsATreeWithNoSkills(t *testing.T) {
	t.Parallel()
	populated, bare := t.TempDir(), t.TempDir()
	writeTreeSkill(t, populated, "x", "# X\n")

	out, err := run(t, "duplicates", populated, bare)
	if err != nil {
		t.Fatalf("an empty tree must not fail the run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no skills found") {
		t.Errorf("want the empty tree reported, got:\n%s", out)
	}
}

// TestDuplicatesRequiresTwoTrees refuses the invocation that could only ever report
// nothing -- the vacuous-check trap this item's original in-tree half died of.
func TestDuplicatesRequiresTwoTrees(t *testing.T) {
	t.Parallel()
	out, err := run(t, "duplicates", t.TempDir())
	if err == nil {
		t.Fatalf("one tree must be a usage error, got:\n%s", out)
	}
	if !strings.Contains(err.Error(), "at least two trees") {
		t.Errorf("error = %v, want it to say two trees are needed", err)
	}
}

// TestDuplicatesAcrossThreeTrees covers the real shape on this machine: a market
// repository plus several installed copies, one of which drifted.
func TestDuplicatesAcrossThreeTrees(t *testing.T) {
	t.Parallel()
	market, claude, agents := t.TempDir(), t.TempDir(), t.TempDir()
	writeTreeSkill(t, market, "shared", "# Current\n")
	writeTreeSkill(t, claude, "shared", "# Current\n")
	writeTreeSkill(t, agents, "shared", "# Stale\n")

	out, err := run(t, "duplicates", market, claude, agents)
	if err == nil {
		t.Fatalf("one stale copy of three must fail, got:\n%s", out)
	}
	if !strings.Contains(out, "shared: in 3 trees, DIVERGED") {
		t.Errorf("want all three copies counted, got:\n%s", out)
	}
}
