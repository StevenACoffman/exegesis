package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// declareMergedFrom rewrites dir's SKILL.md to declare metadata.merged-from sources.
func declareMergedFrom(t *testing.T, dir, sources string) {
	t.Helper()
	path := filepath.Join(dir, "SKILL.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	out := strings.Replace(string(b), "\n---\n",
		"\nmetadata:\n  merged-from: "+sources+"\n---\n", 1)
	writeAt(t, path, out)
}

// mixedTree writes a merged skill declaring its sources with a merged test set, and an
// ordinary skill with a standard one: the shape every market plugin holding a merged
// skill has.
func mixedTree(t *testing.T) (tree, plain string) {
	t.Helper()
	tree = t.TempDir()
	merged := writeNamedSkill(t, tree, "merged")
	declareMergedFrom(t, merged, "source-a source-b")
	writeAt(t, filepath.Join(merged, "test-prompts.json"), mergedTP)
	plain = writeNamedSkill(t, tree, "plain")
	writeAt(t, filepath.Join(plain, "test-prompts.json"), validTP)
	return tree, plain
}

// TestVerifyGatesEachSkillByItsDeclaration is the case --merge could not serve: one tree
// holding a merged skill and an ordinary one. Each is gated under its own composition.
func TestVerifyGatesEachSkillByItsDeclaration(t *testing.T) {
	t.Parallel()
	tree, _ := mixedTree(t)
	if out, err := run(t, "verify", "--gates", "skills", tree); err != nil {
		t.Fatalf("a mixed tree must verify without --merge: %v\n%s", err, out)
	}
	if out, err := run(
		t,
		"tests",
		filepath.Join(tree, "merged"),
		filepath.Join(tree, "plain"),
	); err != nil {
		t.Fatalf("tests must gate each set by its declaration: %v\n%s", err, out)
	}
}

// TestDeclaredMergedSkillNeedsTheMergedMix checks the declaration raises the bar rather
// than only lowering it: a declared merged skill with a standard set fails.
func TestDeclaredMergedSkillNeedsTheMergedMix(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	dir := writeNamedSkill(t, tree, "merged")
	declareMergedFrom(t, dir, "source-a source-b")
	writeAt(t, filepath.Join(dir, "test-prompts.json"), validTP)
	out, err := run(t, "verify", "--gates", "skills", tree)
	if err == nil || !strings.Contains(out, "need >=2 prefer_merged_over_source") {
		t.Fatalf("a declared merged skill must meet the merged composition; err=%v\n%s", err, out)
	}
}

// TestUndeclaredPreferCasesSayWhatToDeclare checks the failure names the repair.
func TestUndeclaredPreferCasesSayWhatToDeclare(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	dir := writeNamedSkill(t, tree, "merged")
	writeAt(t, filepath.Join(dir, "test-prompts.json"), mergedTP)
	for _, args := range [][]string{{"verify", "--gates", "skills", tree}, {"tests", dir}} {
		out, err := run(t, args...)
		if err == nil || !strings.Contains(out, "declare metadata.merged-from") {
			t.Errorf(
				"%s: want a failure naming metadata.merged-from; err=%v\n%s",
				args[0],
				err,
				out,
			)
		}
	}
}

func TestLintRejectsMalformedMergedFrom(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	dir := writeNamedSkill(t, tree, "merged")
	declareMergedFrom(t, dir, "merged Not_A_Name")
	out, err := run(t, "lint", dir)
	if err == nil {
		t.Fatalf("lint must fail on a malformed metadata.merged-from:\n%s", out)
	}
	for _, want := range []string{"names the skill itself", `"Not_A_Name" is not a valid skill name`} {
		if !strings.Contains(out, want) {
			t.Errorf("lint output missing %q:\n%s", want, out)
		}
	}
}

// TestEdgeToSubsumedSkillIsRedirected checks the second job of the declaration: an edge
// naming a source no longer in the tree is reported as merged, and INDEX.md draws it to
// the skill that subsumed the source rather than dropping it.
func TestEdgeToSubsumedSkillIsRedirected(t *testing.T) {
	t.Parallel()
	tree, plain := mixedTree(t)
	appendRelated(t, plain, "- depends-on: `source-a` — learn the source first")

	out, err := run(t, "verify", "--gates", "skills", tree)
	if err != nil {
		t.Fatalf("verify: %v\n%s", err, out)
	}
	if !strings.Contains(
		out,
		"graph: plain: depends-on `source-a` — not in this tree, but merged into `merged`",
	) {
		t.Errorf("want the edge reported as merged into the merged skill:\n%s", out)
	}

	if _, err := run(t, "index", tree); err != nil {
		t.Fatalf("index: %v", err)
	}
	index := readFileString(t, filepath.Join(tree, "INDEX.md"))
	if !strings.Contains(index, "plain -->|depends-on| merged") {
		t.Errorf("INDEX.md must draw the edge to the merged skill:\n%s", index)
	}
}
