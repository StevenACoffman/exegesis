package skill_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skill"
)

func writeSkill(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLoadParsesFrontmatter(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "my-skill")
	writeSkill(
		t,
		dir,
		"---\nname: my-skill\ndescription: >-\n  A folded description\n  across two lines.\ntags: [a, b]\n---\n\n# Body\ntext\n",
	)

	s, err := skill.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Name != "my-skill" {
		t.Errorf("Name = %q, want my-skill", s.Name)
	}
	if want := "A folded description across two lines."; s.Description != want {
		t.Errorf("Description = %q, want %q", s.Description, want)
	}
	if got, want := s.FrontmatterKeys, []string{
		"description",
		"name",
		"tags",
	}; !equalStrings(
		got,
		want,
	) {
		t.Errorf("FrontmatterKeys = %v, want %v", got, want)
	}
	if wantBody := "\n# Body\ntext\n"; s.Body != wantBody {
		t.Errorf("Body = %q, want %q", s.Body, wantBody)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	if _, err := skill.Load(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing SKILL.md")
	}
}

func TestLoadMalformedFrontmatterIsEmptyNotError(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "bad")
	writeSkill(t, dir, "---\nname: [unterminated\n---\nbody\n")
	s, err := skill.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Name != "" {
		t.Errorf("Name = %q, want empty for malformed frontmatter", s.Name)
	}
}

func TestDiscover(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeSkill(t, filepath.Join(tree, "alpha"), "---\nname: alpha\n---\n")
	writeSkill(t, filepath.Join(tree, "beta"), "---\nname: beta\n---\n")
	if err := os.MkdirAll(filepath.Join(tree, "not-a-skill"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	dirs, err := skill.Discover(tree)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got, want := len(dirs), 2; got != want {
		t.Fatalf("Discover found %d dirs, want %d (%v)", got, want, dirs)
	}
	if filepath.Base(dirs[0]) != "alpha" || filepath.Base(dirs[1]) != "beta" {
		t.Errorf("Discover = %v, want [alpha beta] sorted", dirs)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
