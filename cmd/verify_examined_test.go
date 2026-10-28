package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/skillet/manifest"
)

// manifestIn decodes the manifest a verify run already wrote into tree. Distinct from
// readManifest, which runs verify itself; these tests need to assert on the exit code first.
func manifestIn(t *testing.T, tree string) manifest.Manifest {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tree, "skills-manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	m, err := manifest.Parse(b)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

// TestVerifyEmptyTreeIsNotVerified is the measured defect: a tree holding no skills passed
// every gate vacuously, wrote structure_verified=true, and exited 0 -- byte-identical in
// verdict to a tree that genuinely passed.
func TestVerifyEmptyTreeIsNotVerified(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()

	out, err := run(t, "verify", tree)
	if err == nil {
		t.Fatalf("verify over a tree holding no skills should not pass, got:\n%s", out)
	}
	if !strings.Contains(out, "no skills found under "+tree) {
		t.Errorf("want the path named so a reader sees where it looked, got:\n%s", out)
	}
	// The discovery rule is the trap -- one level down, root excluded -- so naming it is
	// what turns the report into a diagnosis.
	if !strings.Contains(out, "immediate subdirectory") {
		t.Errorf("want the discovery rule stated, got:\n%s", out)
	}
}

// TestVerifyRecordsHowManySkillsItExamined is the artifact half: a consumer reading the
// manifest can now tell a verdict about nothing from a verdict about a tree.
func TestVerifyRecordsHowManySkillsItExamined(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	skillDir := filepath.Join(tree, "skilla")
	writeSkill(t, skillDir)
	if _, err := run(t, "tests", "--scaffold", skillDir); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if out, err := run(t, "verify", tree); err != nil {
		t.Fatalf("verify: %v\n%s", err, out)
	}

	m := manifestIn(t, tree)
	if m.Examined != 1 {
		t.Errorf("Examined = %d, want the one skill counted", m.Examined)
	}
	if !m.StructureVerified {
		t.Errorf("a clean tree should still be structure_verified: %+v", m)
	}
}

// TestVerifyEmptyTreeManifestSaysItExaminedNothing pins the pair a consumer reads together.
// StructureVerified is deliberately untouched -- forcing it false would overload one value
// with "gates failed" and "no gates ran", which is the conflation the count removes.
func TestVerifyEmptyTreeManifestSaysItExaminedNothing(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()

	if _, err := run(t, "verify", tree); err == nil {
		t.Fatal("an unexaminable tree should not pass")
	}
	m := manifestIn(t, tree)
	if m.Examined != 0 {
		t.Errorf("Examined = %d, want zero for a tree with no skills", m.Examined)
	}
	if len(m.Skills) != 0 {
		t.Errorf("Skills = %+v, want none", m.Skills)
	}
}
