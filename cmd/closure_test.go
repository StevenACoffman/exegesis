package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifyReportsDirectoryWithoutSkillFile pins the state the closure check exists
// for: a directory that lost its SKILL.md is skipped by Discover and, without a
// registry, reported by nothing else, so it verifies clean while holding no skill.
func TestVerifyReportsDirectoryWithoutSkillFile(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeNamedSkill(t, tree, "skilla")
	if _, err := run(t, "tests", "--scaffold", filepath.Join(tree, "skilla")); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	// A directory holding files but no SKILL.md: the "half-migrated" shape.
	lost := filepath.Join(tree, "workspace")
	if err := os.MkdirAll(lost, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lost, "notes.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, err := run(t, "verify", tree)
	if err != nil {
		t.Fatalf("closure is reported, not failed; got %v\n%s", err, out)
	}
	if !strings.Contains(out, "closure: workspace: a directory with no SKILL.md") {
		t.Errorf("expected the directory named, got:\n%s", out)
	}
	manifest := readFileString(t, filepath.Join(tree, "skills-manifest.json"))
	if !strings.Contains(manifest, `"structure_verified": true`) {
		t.Errorf("expected structure_verified=true:\n%s", manifest)
	}
}

// TestVerifyClosureIgnoresVerifiedSkills guards the direction the set difference can
// fail in: subtracting the wrong side would report every skill in the tree, which
// would read as total breakage rather than as one lost directory.
func TestVerifyClosureIgnoresVerifiedSkills(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	for _, name := range []string{"skilla", "skillb"} {
		writeNamedSkill(t, tree, name)
		if _, err := run(t, "tests", "--scaffold", filepath.Join(tree, name)); err != nil {
			t.Fatalf("scaffold %s: %v", name, err)
		}
	}
	out, err := run(t, "verify", tree)
	if err != nil {
		t.Fatalf("verify: %v\n%s", err, out)
	}
	if strings.Contains(out, "closure:") {
		t.Errorf("a tree of nothing but skills has nothing unlisted, got:\n%s", out)
	}
}

// TestVerifyFailsOnAnEntryThatEscapesTheTree pins K6's consumer half: a symbolic link out
// of a packaged tree would not survive an installer's copy, so verify reports it as a
// tree defect and fails, rather than following it or reading it as absent.
func TestVerifyFailsOnAnEntryThatEscapesTheTree(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	writeNamedSkill(t, outside, "elsewhere")

	tree := t.TempDir()
	writeNamedSkill(t, tree, "skilla")
	if _, err := run(t, "tests", "--scaffold", filepath.Join(tree, "skilla")); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if err := os.Symlink(
		filepath.Join(outside, "elsewhere"),
		filepath.Join(tree, "escaping"),
	); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	out, err := run(t, "verify", "--manifest", filepath.Join(t.TempDir(), "m.json"), tree)
	if err == nil {
		t.Fatalf("an escaping entry must fail verify, got:\n%s", out)
	}
	if !strings.Contains(out, "skipped: "+filepath.Join(tree, "escaping")+": ") {
		t.Errorf("expected the escaping entry named with its reason, got:\n%s", out)
	}
}

// TestVerifyReportsDevArtifactsInsideSkills pins the closure check's second half: the
// family's development files ship with a skill when they sit inside it, so verify names
// them, without failing a tree that is still being built.
func TestVerifyReportsDevArtifactsInsideSkills(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeNamedSkill(t, tree, "skilla")
	dir := filepath.Join(tree, "skilla")
	if _, err := run(t, "tests", "--scaffold", dir); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	for _, p := range []string{"test-results.md", "TODO.md", "eval-workspace/iteration-1/out.txt"} {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out, err := run(t, "verify", "--manifest", filepath.Join(t.TempDir(), "m.json"), tree)
	if err != nil {
		t.Fatalf("dev artifacts are reported, not failed; got %v\n%s", err, out)
	}
	for _, want := range []string{
		"closure: skilla/TODO.md: a development file",
		"closure: skilla/eval-workspace: a development file",
		"closure: skilla/test-results.md: a development file",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "eval-workspace/iteration-1") {
		t.Errorf("a workspace must be reported once, not per file:\n%s", out)
	}
}
