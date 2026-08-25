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
