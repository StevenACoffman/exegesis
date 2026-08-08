package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLinkSupersededByAcrossTrees drives the shape merge-skills prescribes: a source
// skill in one book pointing at the merged skill that replaced it, in another tree.
func TestLinkSupersededByAcrossTrees(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	book := filepath.Join(root, "some-book")
	merged := filepath.Join(root, "merged", "all-books-v1")
	if err := os.MkdirAll(merged, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := writeNamedSkill(t, book, "source-skill")
	writeNamedSkill(t, merged, "merged-skill")

	out, err := run(t, "link", "--kind", "superseded-by",
		"--to", "merged/all-books-v1/merged-skill",
		"--rationale", "consolidated with another book", source)
	if err != nil {
		t.Fatalf("link: %v\n%s", err, out)
	}
	if strings.Contains(out, "warning") {
		t.Errorf("a target that exists in the sibling tree must not warn:\n%s", out)
	}
	got := readFileString(t, filepath.Join(source, "SKILL.md"))
	want := "- superseded-by: `merged/all-books-v1/merged-skill` — consolidated with another book"
	if !strings.Contains(got, want) {
		t.Errorf("expected %q, got:\n%s", want, got)
	}

	// The qualifier must survive: slugging the whole target would write one mangled
	// slug naming a skill that cannot exist.
	if strings.Contains(got, "merged-all-books-v1-merged-skill") {
		t.Errorf("target was slugged as a whole, destroying the tree qualifier:\n%s", got)
	}

	// verify's graph gate must not call the cross-tree edge dangling. The fixture
	// fails other gates (it has no test-prompts.json), so the assertion is on the
	// graph report itself rather than on the exit code.
	out, _ = run(t, "verify", book)
	if strings.Contains(out, "graph:") {
		t.Errorf("a qualified target was reported as dangling:\n%s", out)
	}
}

func TestLinkWarnsOnAQualifiedTargetThatDoesNotExist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	book := filepath.Join(root, "some-book")
	source := writeNamedSkill(t, book, "source-skill")

	out, err := run(t, "link", "--kind", "superseded-by",
		"--to", "merged/all-books-v1/absent-skill", "--rationale", "typo", source)
	if err != nil {
		t.Fatalf("link: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no skill \"merged/all-books-v1/absent-skill\"") {
		t.Errorf("expected a warning naming the unresolved target, got:\n%s", out)
	}
}

func TestRelateFailsOnAQualifiedTargetThatDoesNotExist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	book := filepath.Join(root, "some-book")
	writeNamedSkill(t, book, "source-skill")
	edges := filepath.Join(root, "edges.json")
	table := `{"edges":[{"from":"source-skill","kind":"superseded-by",` +
		`"to":"merged/all-books-v1/absent-skill","rationale":"typo"}]}`
	if err := os.WriteFile(edges, []byte(table), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// relate is handed the tree, so it errors where link warns.
	_, err := run(t, "relate", "--edges", edges, book)
	if err == nil || !strings.Contains(err.Error(), "no such skill") {
		t.Errorf("expected relate to reject an unresolvable cross-tree target, got %v", err)
	}
}
