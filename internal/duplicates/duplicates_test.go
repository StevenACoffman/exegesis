package duplicates_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/duplicates"
)

func TestFind(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		copies       []duplicates.Copy
		wantSlugs    []string
		wantDiverged map[string]bool
	}{
		"a slug in one tree only is not a duplicate": {
			copies:    []duplicates.Copy{{Tree: "a", Slug: "x", Hash: "1"}},
			wantSlugs: nil,
		},
		"identical copies are duplicated but not diverged": {
			copies: []duplicates.Copy{
				{Tree: "a", Slug: "x", Hash: "1"},
				{Tree: "b", Slug: "x", Hash: "1"},
			},
			wantSlugs:    []string{"x"},
			wantDiverged: map[string]bool{"x": false},
		},
		"differing copies are diverged": {
			copies: []duplicates.Copy{
				{Tree: "a", Slug: "x", Hash: "1"},
				{Tree: "b", Slug: "x", Hash: "2"},
			},
			wantSlugs:    []string{"x"},
			wantDiverged: map[string]bool{"x": true},
		},
		"three trees where only the third differs": {
			copies: []duplicates.Copy{
				{Tree: "a", Slug: "x", Hash: "1"},
				{Tree: "b", Slug: "x", Hash: "1"},
				{Tree: "c", Slug: "x", Hash: "9"},
			},
			wantSlugs:    []string{"x"},
			wantDiverged: map[string]bool{"x": true},
		},
		"findings are sorted by slug": {
			copies: []duplicates.Copy{
				{Tree: "a", Slug: "z", Hash: "1"},
				{Tree: "b", Slug: "z", Hash: "1"},
				{Tree: "a", Slug: "m", Hash: "1"},
				{Tree: "b", Slug: "m", Hash: "2"},
			},
			wantSlugs:    []string{"m", "z"},
			wantDiverged: map[string]bool{"m": true, "z": false},
		},
		"no copies at all": {
			copies:    nil,
			wantSlugs: nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertFindings(t, duplicates.Find(tc.copies), tc.wantSlugs, tc.wantDiverged)
		})
	}
}

// assertFindings checks that got names wantSlugs in order and that each one's Diverged
// verdict matches. It exists to keep TestFind's body flat: the two assertions inside a
// loop inside a subtest are what push the table over the complexity cap.
func assertFindings(
	t *testing.T,
	got []duplicates.Finding,
	wantSlugs []string,
	wantDiverged map[string]bool,
) {
	t.Helper()
	if len(got) != len(wantSlugs) {
		t.Fatalf("Find returned %d finding(s), want %d: %+v", len(got), len(wantSlugs), got)
	}
	for i, f := range got {
		if f.Slug != wantSlugs[i] {
			t.Errorf("finding %d slug = %q, want %q", i, f.Slug, wantSlugs[i])
		}
		if f.Diverged() != wantDiverged[f.Slug] {
			t.Errorf("%s Diverged() = %t, want %t",
				f.Slug, f.Diverged(), wantDiverged[f.Slug])
		}
	}
}

// TestFindSortsCopiesByTree keeps a report stable across runs: the copies come out of a
// map, so without the sort the order would vary between invocations.
func TestFindSortsCopiesByTree(t *testing.T) {
	t.Parallel()
	got := duplicates.Find([]duplicates.Copy{
		{Tree: "zebra", Slug: "x", Hash: "1"},
		{Tree: "alpha", Slug: "x", Hash: "1"},
	})
	if len(got) != 1 {
		t.Fatalf("Find = %+v, want one finding", got)
	}
	if got[0].Copies[0].Tree != "alpha" || got[0].Copies[1].Tree != "zebra" {
		t.Errorf("copies = %+v, want them sorted by tree", got[0].Copies)
	}
}

// writeSkill creates a skill directory whose SKILL.md carries body.
func writeSkill(t *testing.T, tree, slug, body string) {
	t.Helper()
	dir := filepath.Join(tree, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	doc := "---\nname: " + slug + "\ndescription: d\n---\n\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write %s: %v", dir, err)
	}
}

func TestScan(t *testing.T) {
	t.Parallel()
	treeA, treeB := t.TempDir(), t.TempDir()
	writeSkill(t, treeA, "shared", "# Same\n")
	writeSkill(t, treeB, "shared", "# Different\n")
	writeSkill(t, treeA, "only-in-a", "# Alone\n")

	copies, empty, err := duplicates.Scan([]string{treeA, treeB})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("empty = %q, want none", empty)
	}
	if len(copies) != 3 {
		t.Fatalf("copies = %+v, want three", copies)
	}
	found := duplicates.Find(copies)
	if len(found) != 1 || found[0].Slug != "shared" {
		t.Fatalf("Find = %+v, want just the shared slug", found)
	}
	if !found[0].Diverged() {
		t.Error("Diverged() = false, want true: the two copies hold different text")
	}
}

// TestScanHashesIdenticalContentIdentically is what makes divergence meaningful: two
// byte-identical copies must hash the same, or every duplicate would look diverged.
func TestScanHashesIdenticalContentIdentically(t *testing.T) {
	t.Parallel()
	treeA, treeB := t.TempDir(), t.TempDir()
	writeSkill(t, treeA, "shared", "# Same\n")
	writeSkill(t, treeB, "shared", "# Same\n")

	copies, _, err := duplicates.Scan([]string{treeA, treeB})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	found := duplicates.Find(copies)
	if len(found) != 1 {
		t.Fatalf("Find = %+v, want one finding", found)
	}
	if found[0].Diverged() {
		t.Errorf("Diverged() = true for identical copies: %+v", found[0].Copies)
	}
}

func TestScanReportsATreeWithNoSkills(t *testing.T) {
	t.Parallel()
	populated, bare := t.TempDir(), t.TempDir()
	writeSkill(t, populated, "x", "# X\n")

	copies, empty, err := duplicates.Scan([]string{populated, bare})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(empty) != 1 || empty[0] != bare {
		t.Errorf("empty = %q, want the tree holding no skills", empty)
	}
	if len(copies) != 1 {
		t.Errorf("copies = %+v, want the populated tree still read", copies)
	}
}

func TestScanReportsAnUnreadableTree(t *testing.T) {
	t.Parallel()
	_, _, err := duplicates.Scan([]string{filepath.Join(t.TempDir(), "absent")})
	if err == nil {
		t.Error("Scan over a missing tree = nil error, want the tree reported")
	}
}
