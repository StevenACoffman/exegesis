package skilllint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint"
)

func TestCheckDisclosure(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "disc")
	writeSkill(
		t,
		dir,
		"---\nname: disc\ndescription: Use when you need to test disclosure checks here.\n---\n"+
			"# H\n\nSee [guide](references/guide.md).\n",
	)

	// A large reference file (> ~10000 tokens via the ~4 chars/token estimate).
	mustWrite(t, filepath.Join(dir, "references", "big.md"), strings.Repeat("word ", 12000))
	// A reference that links onward to another local file (nesting).
	mustWrite(
		t,
		filepath.Join(dir, "references", "guide.md"),
		"# Guide\n\nSee [deep](../assets/x.md).\n",
	)
	mustWrite(t, filepath.Join(dir, "assets", "x.md"), "# X\n")

	ids := checkIDs(skilllint.CheckDisclosure(skilllint.Parse(dir), nil))
	if !ids["4b.reference.large"] {
		t.Errorf("expected 4b.reference.large; got %v", keysOf(ids))
	}
	if !ids["4c.nesting"] {
		t.Errorf("expected 4c.nesting; got %v", keysOf(ids))
	}
}

func TestReferenceSizingSkipsBinary(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "bin")
	writeSkill(
		t,
		dir,
		"---\nname: bin\ndescription: Use when you need to test binary skipping here.\n---\n# H\ntext\n",
	)
	// A large binary blob (NUL byte + well over the ~10000-token estimate).
	blob := append([]byte{0}, []byte(strings.Repeat("x", 60000))...)
	mustWriteBytes(t, filepath.Join(dir, "references", "big.bin"), blob)

	ids := checkIDs(skilllint.CheckDisclosure(skilllint.Parse(dir), nil))
	if ids["4b.reference.large"] {
		t.Error("binary reference should be skipped by the token-size check")
	}
}

func mustWriteBytes(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}
