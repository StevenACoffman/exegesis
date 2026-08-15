package mergeindexgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/mergeindexgen"
	"github.com/StevenACoffman/exegesis/internal/mergemigrate"
)

// writeMerged writes a merged skill with the given sources, using mergemigrate.Render for
// the `## Provenance` block so the test exercises the exact shape the migration writes.
func writeMerged(t *testing.T, tree, slug string, sources ...string) {
	t.Helper()
	dir := filepath.Join(tree, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	srcs := make([]mergemigrate.Source, len(sources))
	for i, s := range sources {
		srcs[i] = mergemigrate.Source{Slug: s}
	}
	body := "# " + slug + "\n\ntext\n\n" +
		mergemigrate.Render(mergemigrate.Provenance{Type: "merged-skill", Sources: srcs})
	md := "---\nname: " + slug + "\ndescription: d\n---\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestGenerate(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	// shared/y feeds both alpha and beta -> it is the one fan-in source.
	writeMerged(t, tree, "alpha", "book1/x", "shared/y")
	writeMerged(t, tree, "beta", "book2/z", "shared/y")
	rej := filepath.Join(tree, "rejected")
	if err := os.MkdirAll(rej, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rej, "pair-001.md"),
		[]byte("# pair-001: a × b — REJECTED\n\nreason\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, err := mergeindexgen.Generate(tree)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	wants := []string{
		"**Total merged skills:** 2",
		"**Rejected pairs:** 1",
		"**Fan-in sources** (feed ≥ 2 merged skills): 1",
		"[alpha](alpha/SKILL.md)",
		"[beta](beta/SKILL.md)",
		"`shared/y` ★",           // the fan-in source is marked in both rows
		"`book1/x`",              // a non-fan-in source is not marked
		"No source-verification", // empty section states so, not a blank table
		"[pair-001](rejected/pair-001.md) — pair-001: a × b — REJECTED",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("generated INDEX.md missing %q:\n%s", w, out)
		}
	}
	if strings.Contains(out, "`book1/x` ★") {
		t.Errorf("a source feeding one skill must not be marked fan-in:\n%s", out)
	}
	// Deterministic: a second run is byte-identical (so --check is meaningful).
	again, err := mergeindexgen.Generate(tree)
	if err != nil {
		t.Fatalf("Generate (2nd): %v", err)
	}
	if again != out {
		t.Error("Generate is not deterministic")
	}
}
