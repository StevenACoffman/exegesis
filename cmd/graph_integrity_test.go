package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd/root"
)

// writeNamedSkill writes a minimal skill whose frontmatter name matches its directory,
// so it passes lint's name/folder check, and returns the directory.
func writeNamedSkill(t *testing.T, tree, name string) string {
	t.Helper()
	dir := filepath.Join(tree, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "---\nname: " + name + "\n" +
		"description: Invoke when the user needs a demo thing done in a particular way.\n" +
		"---\n# Body\nNothing runtime-bound here.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", dir, err)
	}
	return dir
}

// appendRelated adds a `## Related skills` section holding bullet, writing the section
// directly so the test does not depend on `link`'s own behaviour.
func appendRelated(t *testing.T, dir, bullet string) {
	t.Helper()
	path := filepath.Join(dir, "SKILL.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	out := string(b) + "\n## Related skills\n\n" + bullet + "\n"
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestVerifyFailsOnDanglingEdgeTarget(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	dirA := writeNamedSkill(t, tree, "skilla")
	writeNamedSkill(t, tree, "skillb")
	appendRelated(t, dirA, "- depends-on: `ghost` — the target does not exist")
	// Scaffold both so the graph problem is the only failure in play.
	for _, name := range []string{"skilla", "skillb"} {
		if _, err := run(t, "tests", "--scaffold", filepath.Join(tree, name)); err != nil {
			t.Fatalf("scaffold %s: %v", name, err)
		}
	}

	out, err := run(t, "verify", tree)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected root.ExitError, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "graph: skilla: depends-on `ghost`") {
		t.Errorf("expected a graph problem naming the dangling edge, got:\n%s", out)
	}
	manifest := readFileString(t, filepath.Join(tree, "skills-manifest.json"))
	if !strings.Contains(manifest, `"structure_verified": false`) {
		t.Errorf("expected structure_verified=false in the manifest:\n%s", manifest)
	}
}

func TestVerifyPassesWhenEdgeTargetExists(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	dirA := writeNamedSkill(t, tree, "skilla")
	writeNamedSkill(t, tree, "skillb")
	appendRelated(t, dirA, "- depends-on: `skillb` — the target is a real skill")
	for _, name := range []string{"skilla", "skillb"} {
		if _, err := run(t, "tests", "--scaffold", filepath.Join(tree, name)); err != nil {
			t.Fatalf("scaffold %s: %v", name, err)
		}
	}

	out, err := run(t, "verify", tree)
	if err != nil {
		t.Fatalf("verify returned error: %v\n%s", err, out)
	}
	if strings.Contains(out, "graph:") {
		t.Errorf("resolvable edge must not be reported as a graph problem:\n%s", out)
	}
	if !strings.Contains(out, "skilla: ok") {
		t.Errorf("expected 'skilla: ok', got:\n%s", out)
	}
}

func TestRelateRefusesUnknownTargetWithoutWriting(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeNamedSkill(t, tree, "skilla")
	writeNamedSkill(t, tree, "skillb")
	// The first edge is valid and the second is not: the check must run before any
	// write, so a table that fails late leaves even its good edges unapplied.
	edges := `{"edges":[
	  {"from":"skilla","kind":"depends-on","to":"skillb","rationale":"valid"},
	  {"from":"skillb","kind":"depends-on","to":"ghost","rationale":"typo"}
	]}`
	edgesPath := filepath.Join(tree, "edges.json")
	if err := os.WriteFile(edgesPath, []byte(edges), 0o644); err != nil {
		t.Fatalf("write edges: %v", err)
	}

	out, err := run(t, "relate", "--edges", edgesPath, tree)
	if err == nil {
		t.Fatalf("expected an error for the unknown target, got none\n%s", out)
	}
	if !strings.Contains(err.Error(), "no such skill") ||
		!strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected the error to name the unknown skill, got: %v", err)
	}
	for _, name := range []string{"skilla", "skillb"} {
		got := readFileString(t, filepath.Join(tree, name, "SKILL.md"))
		if strings.Contains(got, "Related skills") {
			t.Errorf("%s was written despite the bad table:\n%s", name, got)
		}
	}
	if _, statErr := os.Stat(filepath.Join(tree, "INDEX.md")); statErr == nil {
		t.Error("INDEX.md was regenerated despite the bad table")
	}
}

func TestLinkWarnsOnUnknownTargetButStillWrites(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	dirA := writeNamedSkill(t, tree, "skilla")

	out, err := run(t, "link", "--kind", "depends-on", "--to", "ghost",
		"--rationale", "target does not exist", dirA)
	if err != nil {
		t.Fatalf("link returned error: %v\n%s", err, out)
	}
	if !strings.Contains(out, `no skill "ghost"`) {
		t.Errorf("expected a warning naming the unknown target, got:\n%s", out)
	}
	got := readFileString(t, filepath.Join(dirA, "SKILL.md"))
	if !strings.Contains(got, "- depends-on: `ghost`") {
		t.Errorf("link must still record the edge, got:\n%s", got)
	}
}
