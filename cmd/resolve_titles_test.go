package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd/root"
)

// titleSkill writes a skill whose H1 is a display title and whose related section names
// another skill the same way -- the shape 97 bullets in the real market corpus use:
// a bold display name, an italic kind, and an arrow separator.
func titleSkill(t *testing.T, tree, slug, heading, body string) string {
	t.Helper()
	dir := filepath.Join(tree, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "---\nname: " + slug + "\n" +
		"description: Invoke when the user needs a demo thing done in a particular way.\n" +
		"---\n# " + heading + "\n\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", dir, err)
	}
	return dir
}

func TestResolveTitlesReportsThenRewrites(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	titleSkill(t, tree, "architect-elevator", "Architect Elevator", "Body.\n")
	source := titleSkill(t, tree, "skilla", "Skill A", `Body.

## Related Skills

- **Architect Elevator** — *depends-on* → it frames the same trade-off
- **Nobody Is Called This** — *depends-on* → names no skill in this tree
`)

	// Report mode names both outcomes and writes nothing.
	out, err := run(t, "normalize", "--resolve-titles", tree)
	if err != nil {
		t.Fatalf("--resolve-titles: %v\n%s", err, out)
	}
	if !strings.Contains(out, "resolved\tArchitect Elevator\tarchitect-elevator") {
		t.Errorf("expected the resolved title to be named, got:\n%s", out)
	}
	if !strings.Contains(out, "unknown\tNobody Is Called This") {
		t.Errorf("expected the unresolvable title to be named, got:\n%s", out)
	}
	if !strings.Contains(out, "2 title-named bullet(s): 1 resolved, 1 unknown, 0 ambiguous") {
		t.Errorf("expected the tally, got:\n%s", out)
	}
	if got := readFileString(t, filepath.Join(source, "SKILL.md")); !strings.Contains(
		got, "**Architect Elevator**",
	) {
		t.Errorf("report mode must not write, got:\n%s", got)
	}

	if out, err = run(t, "normalize", "--resolve-titles", "--write", tree); err != nil {
		t.Fatalf("--write: %v\n%s", err, out)
	}
	got := readFileString(t, filepath.Join(source, "SKILL.md"))
	if !strings.Contains(got, "- depends-on: `architect-elevator` — it frames the same trade-off") {
		t.Errorf("expected the resolved bullet in canonical form, got:\n%s", got)
	}
	// An unresolved title is left exactly as its author wrote it: visibly unreadable
	// beats resolved-to-something-plausible-and-wrong.
	if !strings.Contains(got, "**Nobody Is Called This** — *depends-on* → names no skill") {
		t.Errorf("an unresolved bullet must survive byte-identical, got:\n%s", got)
	}
}

// TestResolveTitlesMakesTheEdgeVisibleToIndex is the point of rewriting rather than
// reporting: an unresolved bullet yields no edge, so INDEX.md's graph and learning path
// are built as though the relationship had never been declared.
func TestResolveTitlesMakesTheEdgeVisibleToIndex(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	titleSkill(t, tree, "architect-elevator", "Architect Elevator", "Body.\n")
	titleSkill(t, tree, "skilla", "Skill A", `Body.

## Related Skills

- **Architect Elevator** — *depends-on* → it frames the same trade-off
`)

	if out, err := run(t, "index", tree); err != nil {
		t.Fatalf("index: %v\n%s", err, out)
	}
	before := readFileString(t, filepath.Join(tree, "INDEX.md"))
	if strings.Contains(before, "skilla -->|depends-on| architect-elevator") {
		t.Fatalf("the title bullet should be unreadable before rewriting:\n%s", before)
	}

	if out, err := run(t, "normalize", "--resolve-titles", "--write", tree); err != nil {
		t.Fatalf("--write: %v\n%s", err, out)
	}
	if out, err := run(t, "index", tree); err != nil {
		t.Fatalf("index: %v\n%s", err, out)
	}
	after := readFileString(t, filepath.Join(tree, "INDEX.md"))
	if !strings.Contains(after, "skilla -->|depends-on| architect-elevator") {
		t.Errorf("expected the rewritten bullet to render as an edge, got:\n%s", after)
	}
}

func TestWriteWithoutResolveTitlesIsAUsageError(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeNamedSkill(t, tree, "skilla")

	_, err := run(t, "normalize", "--write", tree)
	var usage root.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("expected a usage error for --write alone, got %v", err)
	}
}
