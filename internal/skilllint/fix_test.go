package skilllint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint"
)

func TestFixLowercaseAndDirMatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// dir "good-skill" but name "Good--Skill": needs lowercase + hyphen collapse;
	// after which name "good-skill" already matches the dir.
	dir := filepath.Join(root, "good-skill")
	writeSkill(
		t,
		dir,
		"---\nname: Good--Skill\ndescription: Use when you need something here now.\n---\n# H\ntext\n",
	)

	cats := map[skilllint.Category]bool{skilllint.CategorySpec: true}
	_, fixes, err := skilllint.Fix(root, cats)
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(fixes) == 0 {
		t.Fatal("expected fixes to be applied")
	}

	got, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(got), "name: good-skill") {
		t.Errorf("name not normalized; file:\n%s", got)
	}

	// Re-running the linter should now report no name errors.
	res, err := skilllint.Run(root, skilllint.Options{Categories: cats})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Counts().Errors != 0 {
		t.Errorf("expected 0 errors after fix, got %d", res.Counts().Errors)
	}
}

func TestFixRenamesDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// dir "wrong" but valid name "right" -> directory renamed.
	writeSkill(t, filepath.Join(root, "wrong"),
		"---\nname: right\ndescription: Use when you need something here now.\n---\n# H\ntext\n")

	_, fixes, err := skilllint.Fix(root, map[skilllint.Category]bool{skilllint.CategorySpec: true})
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if len(fixes) == 0 {
		t.Fatal("expected a rename fix")
	}
	if !pathIsDir(filepath.Join(root, "right")) {
		t.Error("expected directory renamed to 'right'")
	}
	if pathIsDir(filepath.Join(root, "wrong")) {
		t.Error("old directory 'wrong' should no longer exist")
	}
}

func pathIsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
