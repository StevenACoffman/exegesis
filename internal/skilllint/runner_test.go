package skilllint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint"
)

func TestRunEndToEnd(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// A clean RIA++ skill with test-prompts.json.
	good := filepath.Join(root, "skills", "inversion-thinking")
	writeSkill(t, good, validRIASkill)
	if err := os.WriteFile(
		filepath.Join(good, "test-prompts.json"),
		[]byte("[]"),
		0o600,
	); err != nil {
		t.Fatalf("write test-prompts: %v", err)
	}
	// A broken skill: name mismatch + missing description.
	writeSkill(t, filepath.Join(root, "skills", "broken"), "---\nname: mismatch\n---\n# H\ntext\n")

	all := map[skilllint.Category]bool{
		skilllint.CategoryRedlines: true,
		skilllint.CategorySpec:     true,
	}
	res, err := skilllint.Run(root, skilllint.Options{Categories: all})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	c := res.Counts()
	if c.Skills != 2 {
		t.Errorf("Skills = %d, want 2", c.Skills)
	}
	if c.Errors == 0 {
		t.Error("expected the broken skill to produce errors")
	}
	if res.ExitCode(false) != 1 {
		t.Error("ExitCode should be 1 with errors present")
	}
}
