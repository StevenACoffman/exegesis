package skilllint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint"
)

func writeSkill(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestParseValid(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "my-skill")
	writeSkill(
		t,
		dir,
		"---\nname: my-skill\ndescription: Does a thing.\ntags: [a, b]\n---\n# Body\ntext\n",
	)

	s := skilllint.Parse(dir)
	if s.ParseError != "" {
		t.Fatalf("unexpected parse error: %s", s.ParseError)
	}
	if s.Frontmatter["name"] != "my-skill" {
		t.Errorf("name = %v, want my-skill", s.Frontmatter["name"])
	}
	if got := s.FrontmatterKeys; len(got) != 3 || got[0] != "name" || got[2] != "tags" {
		t.Errorf("FrontmatterKeys = %v, want [name description tags]", got)
	}
	if _, ok := s.Frontmatter["tags"].([]any); !ok {
		t.Errorf("tags should decode to a list, got %T", s.Frontmatter["tags"])
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		content string
		want    string
	}{
		"no opening":  {"name: x\n", "missing opening frontmatter delimiter (---)"},
		"no closing":  {"---\nname: x\n", "missing closing frontmatter delimiter (---)"},
		"not mapping": {"---\n- a\n- b\n---\n", "frontmatter must be a YAML mapping"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), "s")
			writeSkill(t, dir, tc.content)
			if got := skilllint.Parse(dir).ParseError; got != tc.want {
				t.Errorf("ParseError = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseMissing(t *testing.T) {
	t.Parallel()
	s := skilllint.Parse(filepath.Join(t.TempDir(), "nope"))
	if s.ParseError != "SKILL.md not found" {
		t.Errorf("ParseError = %q, want 'SKILL.md not found'", s.ParseError)
	}
}

func TestDiscover(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "skills", "alpha"), "---\nname: alpha\ndescription: d\n---\n")
	writeSkill(t, filepath.Join(root, "skills", "bravo"), "---\nname: bravo\ndescription: d\n---\n")
	// A "skills" child without SKILL.md must still be discovered.
	if err := os.MkdirAll(filepath.Join(root, "skills", "charlie"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	dirs, err := skilllint.Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	var names []string
	for _, d := range dirs {
		names = append(names, filepath.Base(d))
	}
	want := []string{"alpha", "bravo", "charlie"}
	if len(names) != len(want) {
		t.Fatalf("discovered %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("sorted[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}
