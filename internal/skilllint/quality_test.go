package skilllint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint"
)

func TestCheckQualityCleanSkill(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "clean")
	writeSkill(t, dir, "---\nname: clean\ndescription: Use when you need a clean, "+
		"sufficiently long description here.\n---\n# Heading\n\nSolid instructions.\n")
	ids := checkIDs(skilllint.CheckQuality(skilllint.Parse(dir), nil))
	for _, unwanted := range []string{
		"2a.description.short", "2a.description.no-when", "2c.broken-link", "2d.unclosed-fence",
	} {
		if ids[unwanted] {
			t.Errorf("clean skill unexpectedly fired %q", unwanted)
		}
	}
}

func TestCheckQualityViolations(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "messy")
	body := "# H\n\nSee [gone](missing.md) and jump to [x](#no-such-heading).\n\n" +
		"```go\nunclosed fence\n"
	writeSkill(t, dir, "---\nname: messy\ndescription: short\n---\n"+body)

	mustWrite(t, filepath.Join(dir, ".env"), "SECRET=1")
	mustWrite(t, filepath.Join(dir, "tool.exe"), "MZ")
	mustWrite(t, filepath.Join(dir, "README.md"), "# readme")
	mustWrite(t, filepath.Join(dir, "references", "unused.md"), "# unused\n")

	ids := checkIDs(skilllint.CheckQuality(skilllint.Parse(dir), nil))
	want := []string{
		"2a.description.short",
		"2a.description.no-when",
		"2c.broken-link",
		"2c.broken-link.fragment",
		"2d.unclosed-fence",
		"2b.secrets.filename",
		"2b.binary",
		"2b.extraneous-file",
		"2b.orphan",
	}
	for _, id := range want {
		if !ids[id] {
			t.Errorf("expected %q to fire; got %v", id, keysOf(ids))
		}
	}
}

func TestCheckQualitySecretContent(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "leaky")
	writeSkill(
		t,
		dir,
		"---\nname: leaky\ndescription: Use when you need to test secret scanning here.\n---\n# H\ntext\n",
	)
	mustWrite(t, filepath.Join(dir, "notes.md"), "token AKIAABCDEFGHIJKLMNOP in text")
	ids := checkIDs(skilllint.CheckQuality(skilllint.Parse(dir), nil))
	if !ids["2b.secrets.content"] {
		t.Errorf("expected 2b.secrets.content; got %v", keysOf(ids))
	}
}

func TestCheckQualityExcludePrunes(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "vendored")
	writeSkill(
		t,
		dir,
		"---\nname: vendored\ndescription: Use when you need to test exclude glob pruning here.\n---\n# H\ntext\n",
	)
	// A secret-bearing file that would fire 2b.secrets.content unless pruned.
	mustWrite(t, filepath.Join(dir, "node_modules", "leak.md"), "token AKIAABCDEFGHIJKLMNOP here")

	if ids := checkIDs(
		skilllint.CheckQuality(skilllint.Parse(dir), nil),
	); !ids["2b.secrets.content"] {
		t.Fatalf("expected 2b.secrets.content without exclude; got %v", keysOf(ids))
	}
	if ids := checkIDs(
		skilllint.CheckQuality(skilllint.Parse(dir), []string{"node_modules"}),
	); ids["2b.secrets.content"] {
		t.Errorf("exclude=node_modules should prune the walk; still got %v", keysOf(ids))
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}
