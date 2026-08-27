package related_test

import (
	"testing"

	"github.com/StevenACoffman/exegesis/internal/related"
)

func tree() []related.Node {
	return []related.Node{
		{Slug: "alpha", Heading: "Alpha Skill"},
		{Slug: "beta", Heading: "Beta Skill"},
		// Two skills sharing a heading. Measured on a real corpus: 232 distinct
		// headings across 233 skills, so this is not hypothetical.
		{Slug: "gamma-one", Heading: "Shared Heading"},
		{Slug: "gamma-two", Heading: "Shared Heading"},
		{Slug: "no-heading"},
	}
}

// wantResolve asserts a title's resolution and slug together, because the pair is the
// contract: a slug without TitleResolved would be written into an edge by a caller that
// only checked one of them.
func wantResolve(t *testing.T, title, wantSlug string, wantRes related.Resolution) {
	t.Helper()
	slug, res := related.NewTitles(tree()).Resolve(title)
	if res != wantRes {
		t.Errorf("Resolve(%q) resolution = %v, want %v", title, res, wantRes)
	}
	if slug != wantSlug {
		t.Errorf("Resolve(%q) slug = %q, want %q", title, slug, wantSlug)
	}
}

func TestResolveTitle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		title string
		slug  string
		res   related.Resolution
	}{
		{"an exact heading resolves", "Alpha Skill", "alpha", related.TitleResolved},
		{"surrounding space is ignored", "  Beta Skill ", "beta", related.TitleResolved},
		// Refusing is the point: picking either would be a coin flip recorded as fact.
		{
			"a shared heading is ambiguous and yields no slug",
			"Shared Heading",
			"",
			related.TitleAmbiguous,
		},
		{"an unknown title yields no slug", "Nothing Like This", "", related.TitleUnknown},
		// Exact only. Slugifying would return "alpha" here and be wrong as often as right.
		{"a near miss does not resolve", "Alpha Skil", "", related.TitleUnknown},
		{"case is not folded", "alpha skill", "", related.TitleUnknown},
		{"a skill with no heading is not indexed", "", "", related.TitleUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantResolve(t, tt.title, tt.slug, tt.res)
		})
	}
}

// TestTitleRefsSkipsWhatIsNotATitle. A bold token that is already a slug, or a kind in
// the bold position, is a dialect problem rather than a lookup one and must not be
// reported here — reporting it would inflate the count the dialect work is measured by.
func TestTitleRefsSkipsWhatIsNotATitle(t *testing.T) {
	t.Parallel()
	nodes := append(tree(), related.Node{
		Slug:    "caller",
		Heading: "Caller",
		Body: "## Related Skills\n\n" +
			"- **Alpha Skill** — *depends-on* → a title\n" +
			"- **alpha** — *depends-on* → already a slug\n" +
			"- **composes_with**: beta — a kind in the bold slot\n" +
			"- composes-with: `beta` — canonical, not bold\n",
	})
	refs := related.TitleRefs(nodes)
	if len(refs) != 1 {
		t.Fatalf("refs = %+v, want only the display-title bullet", refs)
	}
	if refs[0].Slug != "alpha" || refs[0].Resolution != related.TitleResolved {
		t.Errorf("ref = %+v, want alpha resolved", refs[0])
	}
}

// TestInlineSlugNeedsTheTargetInTree. A title carrying its own slug resolves only when
// that slug is a skill here; a cross-tree reference has no answer, and inventing one
// would be the wrong-match failure by another route.
func TestInlineSlugNeedsTheTargetInTree(t *testing.T) {
	t.Parallel()
	body := func(title string) string {
		return "## Related Skills\n\n- **" + title + "** — *depends-on* → why\n"
	}
	nodes := append(tree(), related.Node{
		Slug: "caller", Heading: "Caller",
		Body: body("Some Title (`alpha`)") + body("Other Title (`elsewhere`)"),
	})
	refs := related.TitleRefs(nodes)
	got := map[string]related.Resolution{}
	for _, r := range refs {
		got[r.Slug] = r.Resolution
	}
	if got["alpha"] != related.TitleResolved {
		t.Errorf("an in-tree inline slug did not resolve: %+v", refs)
	}
	for _, r := range refs {
		if r.Title == "Other Title (`elsewhere`)" && r.Resolution != related.TitleUnknown {
			t.Errorf("a cross-tree inline slug resolved to %q; it has no answer here", r.Slug)
		}
	}
}
