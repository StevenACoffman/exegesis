package indexgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/indexgen"
)

func writeSkill(t *testing.T, tree, slug, description, related string) {
	t.Helper()
	dir := filepath.Join(tree, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "---\nname: " + slug + "\ndescription: " + description + "\n---\n\n# Body\n" + related
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func TestGenerate(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeSkill(t, tree, "alpha", "the base idea", "")
	writeSkill(t, tree, "beta", "builds on alpha",
		"\n## Related skills\n\n- depends-on: `alpha` — needs the base\n")

	out, err := indexgen.Generate(tree, "Demo Book", "")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		"# Demo Book", "**alpha**", "**beta**",
		"beta -->|depends-on| alpha", // Mermaid edge
		"1. alpha", "2. beta",        // learning path: prerequisite first
	} {
		if !strings.Contains(out, want) {
			t.Errorf("INDEX content missing %q:\n%s", want, out)
		}
	}
}

func TestPath(t *testing.T) {
	t.Parallel()
	if got := indexgen.Path("books/demo"); got != filepath.Join("books", "demo", "INDEX.md") {
		t.Errorf("Path = %q", got)
	}
}
