package related_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/related"
)

// twoSections is the shape 13 real skills have: a suffixed heading and a plain one,
// each with its own bullets, separated by a thematic break.
const twoSections = `# Body

## Related skills (Stage 3 Filling)

- depends-on: ` + "`a`" + ` — one
- contrasts-with: (an idea that is not a skill)

---

## Related Skills

- composes-with: ` + "`b`" + ` — two
- depends-on: ` + "`a`" + ` — restated in different words

---

## Audit Information
`

func TestBulletExactFormat(t *testing.T) {
	t.Parallel()
	got := related.Bullet(
		related.Edge{Kind: related.DependsOn, Target: "other-skill", Rationale: "needs it first"},
	)
	want := "- depends-on: `other-skill` — needs it first"
	if got != want {
		t.Fatalf("Bullet = %q, want %q", got, want)
	}
}

func TestKindValid(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		kind related.Kind
		want bool
	}{
		"depends-on": {related.DependsOn, true},
		"contrasts":  {related.ContrastsWith, true},
		"composes":   {related.ComposesWith, true},
		"unknown":    {related.Kind("relates-to"), false},
		"empty":      {related.Kind(""), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := tc.kind.Valid(); got != tc.want {
				t.Errorf("Kind(%q).Valid() = %v, want %v", tc.kind, got, tc.want)
			}
		})
	}
}

func TestParseSection(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		md   string
		want []related.Edge
	}{
		"no section": {
			md:   "# Title\n\nbody only\n",
			want: nil,
		},
		"one edge": {
			md:   "# T\n\n## Related skills\n\n- depends-on: `a` — because\n",
			want: []related.Edge{{Kind: related.DependsOn, Target: "a", Rationale: "because"}},
		},
		// The result is ordered by kind then target, not by where each bullet sits:
		// a relationship means the same thing wherever in the file it was written.
		"canonical order, all kinds": {
			md: "## Related skills\n\n" +
				"- depends-on: `a` — one\n" +
				"- contrasts-with: `b` — two\n" +
				"- composes-with: `c` — three\n",
			want: []related.Edge{
				{Kind: related.ComposesWith, Target: "c", Rationale: "three"},
				{Kind: related.ContrastsWith, Target: "b", Rationale: "two"},
				{Kind: related.DependsOn, Target: "a", Rationale: "one"},
			},
		},
		"skips unknown kind": {
			md:   "## Related skills\n\n- relates-to: `a` — nope\n- depends-on: `b` — yes\n",
			want: []related.Edge{{Kind: related.DependsOn, Target: "b", Rationale: "yes"}},
		},
		"stops at next heading": {
			md:   "## Related skills\n\n- depends-on: `a` — one\n\n## Notes\n\n- depends-on: `z` — ignored\n",
			want: []related.Edge{{Kind: related.DependsOn, Target: "a", Rationale: "one"}},
		},
		"ignores bullets in a fence": {
			md:   "## Related skills\n\n```\n- depends-on: `x` — fenced\n```\n- depends-on: `a` — real\n",
			want: []related.Edge{{Kind: related.DependsOn, Target: "a", Rationale: "real"}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := related.ParseSection(tc.md)
			if !slices.Equal(got, tc.want) {
				t.Errorf("ParseSection = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestUpsertCreatesSection(t *testing.T) {
	t.Parallel()
	md := "---\nname: s\n---\n# Body\n\ntext\n"
	out, changed := related.Upsert(
		md,
		related.Edge{Kind: related.ComposesWith, Target: "t", Rationale: "why"},
	)
	if !changed {
		t.Fatal("expected changed=true when creating the section")
	}
	if !strings.Contains(out, "## Related Skills") {
		t.Errorf("section heading missing:\n%s", out)
	}
	if !strings.HasPrefix(out, md[:len(md)-1]) { // original preserved (minus its trailing newline)
		t.Errorf("original content not preserved:\n%s", out)
	}
	got := related.ParseSection(out)
	if len(got) != 1 || got[0].Target != "t" {
		t.Errorf("round-trip failed: %#v", got)
	}
}

func TestUpsertIdempotent(t *testing.T) {
	t.Parallel()
	md := "## Related skills\n\n- depends-on: `a` — first\n"
	e := related.Edge{Kind: related.DependsOn, Target: "a", Rationale: "first"}
	out1, changed1 := related.Upsert(md, e)
	if changed1 {
		t.Fatalf("identical edge should be a no-op, got changed=true:\n%s", out1)
	}
	out2, changed2 := related.Upsert(out1, e)
	if changed2 || out2 != out1 {
		t.Errorf("Upsert is not idempotent: changed=%v", changed2)
	}
}

func TestUpsertUpdatesRationaleInPlace(t *testing.T) {
	t.Parallel()
	md := "## Related skills\n\n- depends-on: `a` — old reason\n"
	out, changed := related.Upsert(
		md,
		related.Edge{Kind: related.DependsOn, Target: "a", Rationale: "new reason"},
	)
	if !changed {
		t.Fatal("expected changed=true when the rationale differs")
	}
	if strings.Contains(out, "old reason") {
		t.Errorf("old rationale should be replaced:\n%s", out)
	}
	if strings.Count(out, "`a`") != 1 {
		t.Errorf("edge should be updated in place, not duplicated:\n%s", out)
	}
}

func TestUpsertAppendsToExistingSection(t *testing.T) {
	t.Parallel()
	md := "## Related skills\n\n- depends-on: `a` — one\n"
	out, changed := related.Upsert(
		md,
		related.Edge{Kind: related.ComposesWith, Target: "b", Rationale: "two"},
	)
	if !changed {
		t.Fatal("expected changed=true when appending a new edge")
	}
	// The new bullet is written after the existing one; the parsed order is canonical
	// rather than positional, so the placement is asserted on the text.
	if !strings.Contains(out, "- depends-on: `a` — one\n- composes-with: `b` — two") {
		t.Errorf("append did not follow the existing bullet:\n%s", out)
	}
	if got := related.ParseSection(out); len(got) != 2 {
		t.Errorf("append failed, edges = %#v", got)
	}
}

func TestUpsertAllAppliesEveryEdgeAndIsIdempotent(t *testing.T) {
	t.Parallel()
	md := "---\nname: s\n---\n# Body\n\ntext\n"
	edges := []related.Edge{
		{Kind: related.DependsOn, Target: "a", Rationale: "needs a"},
		{Kind: related.ComposesWith, Target: "b", Rationale: "with b"},
	}
	want := []related.Edge{ // canonical order: by kind, then target
		{Kind: related.ComposesWith, Target: "b", Rationale: "with b"},
		{Kind: related.DependsOn, Target: "a", Rationale: "needs a"},
	}
	out, changed := related.UpsertAll(md, edges)
	if !changed {
		t.Fatal("expected changed=true when adding edges")
	}
	if got := related.ParseSection(out); !slices.Equal(got, want) {
		t.Errorf("UpsertAll did not record every edge: %+v", got)
	}
	if again, changedAgain := related.UpsertAll(out, edges); changedAgain || again != out {
		t.Error("UpsertAll should be idempotent on a second identical apply")
	}
}

func TestParseSectionReadsEverySection(t *testing.T) {
	t.Parallel()
	// Before this, everything the second section declared was dropped without a word.
	got := related.ParseSection(twoSections)
	want := []related.Edge{
		{Kind: related.ComposesWith, Target: "b", Rationale: "two"},
		{Kind: related.DependsOn, Target: "a", Rationale: "one"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("ParseSection = %#v, want %#v", got, want)
	}
}

func TestParseSectionIgnoresASectionHeadingInsideAFence(t *testing.T) {
	t.Parallel()
	// A skill documenting the format in a code block does not thereby declare edges.
	md := "## Related Skills\n\n- depends-on: `a` — real\n\n" +
		"## Notes\n\n```\n## Related Skills\n\n- depends-on: `fenced` — not an edge\n```\n"
	got := related.ParseSection(md)
	want := []related.Edge{{Kind: related.DependsOn, Target: "a", Rationale: "real"}}
	if !slices.Equal(got, want) {
		t.Errorf("ParseSection = %#v, want %#v", got, want)
	}
}

func TestUpsertRewritesAnEdgeLivingInALaterSection(t *testing.T) {
	t.Parallel()
	// Appending to the first section instead would leave the file stating the same
	// relationship twice, with two different rationales.
	md := "## Related Skills\n\n- depends-on: `a` — one\n\n" +
		"## Related Skills\n\n- composes-with: `b` — stale\n"
	out, changed := related.Upsert(
		md,
		related.Edge{Kind: related.ComposesWith, Target: "b", Rationale: "fresh"},
	)
	if !changed {
		t.Fatal("expected changed=true when the rationale differs")
	}
	if strings.Count(out, "`b`") != 1 {
		t.Errorf("edge was duplicated instead of rewritten in place:\n%s", out)
	}
	if !strings.Contains(out, "- composes-with: `b` — fresh") {
		t.Errorf("rationale was not updated:\n%s", out)
	}
	if again, changedAgain := related.Upsert(out, related.Edge{
		Kind: related.ComposesWith, Target: "b", Rationale: "fresh",
	}); changedAgain || again != out {
		t.Error("Upsert should be idempotent across sections")
	}
}
