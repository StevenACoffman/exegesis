package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readSkill(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	return string(b)
}

func TestLinkAddsEdge(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "skilla")
	writeSkill(t, dir)

	out, err := run(
		t,
		"link",
		"--kind",
		"depends-on",
		"--to",
		"other",
		"--rationale",
		"needs it",
		dir,
	)
	if err != nil {
		t.Fatalf("link: %v\n%s", err, out)
	}
	if !strings.Contains(out, "linked") {
		t.Errorf("expected 'linked' in output, got:\n%s", out)
	}
	body := readSkill(t, dir)
	if !strings.Contains(body, "## Related skills") {
		t.Errorf("section not added:\n%s", body)
	}
	if !strings.Contains(body, "- depends-on: `other` — needs it") {
		t.Errorf("edge bullet not written:\n%s", body)
	}
}

func TestLinkIsIdempotent(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "skilla")
	writeSkill(t, dir)
	args := []string{"link", "--kind", "composes-with", "--to", "b", "--rationale", "together", dir}

	if _, err := run(t, args...); err != nil {
		t.Fatalf("first link: %v", err)
	}
	out, err := run(t, args...)
	if err != nil {
		t.Fatalf("second link: %v", err)
	}
	if !strings.Contains(out, "unchanged") {
		t.Errorf("second identical link should report 'unchanged', got:\n%s", out)
	}
	if n := strings.Count(readSkill(t, dir), "`b`"); n != 1 {
		t.Errorf("edge should appear exactly once, found %d", n)
	}
}

func TestLinkRejectsUnknownKind(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "skilla")
	writeSkill(t, dir)

	_, err := run(t, "link", "--kind", "relates-to", "--to", "x", "--rationale", "y", dir)
	if err == nil {
		t.Fatal("expected an error for an unknown --kind")
	}
	if !strings.Contains(err.Error(), "--kind must be") {
		t.Errorf("error should explain the valid kinds, got: %v", err)
	}
}
