package related_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/related"
)

// TestParseSectionDialects covers every bullet dialect found in real skill trees,
// plus the three ways a first draft of the tolerant reader got it wrong.
func TestParseSectionDialects(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		bullet string
		want   []related.Edge
	}{
		"canonical": {
			bullet: "- depends-on: `alpha` — because",
			want: []related.Edge{
				{Kind: related.DependsOn, Target: "alpha", Rationale: "because"},
			},
		},
		"bold kind with arrow and linked backticked target": {
			bullet: "- **composes-with** → [`alpha`](../alpha/SKILL.md): because",
			want: []related.Edge{
				{Kind: related.ComposesWith, Target: "alpha", Rationale: "because"},
			},
		},
		"bold kind with linked backticked target": {
			bullet: "- **composes-with** [`alpha`](../alpha/SKILL.md): because",
			want: []related.Edge{
				{Kind: related.ComposesWith, Target: "alpha", Rationale: "because"},
			},
		},
		"plain kind with linked bare target": {
			bullet: "- depends-on: [alpha](../alpha/SKILL.md) — because",
			want: []related.Edge{
				{Kind: related.DependsOn, Target: "alpha", Rationale: "because"},
			},
		},
		"reversed: bold slug with kind in parens": {
			bullet: "- **alpha** (contrasts-with): because",
			want: []related.Edge{
				{Kind: related.ContrastsWith, Target: "alpha", Rationale: "because"},
			},
		},
		"bare token followed by prose": {
			bullet: "- depends-on: alpha (because of things)",
			want: []related.Edge{
				{Kind: related.DependsOn, Target: "alpha", Rationale: "(because of things)"},
			},
		},
		"multi-target expands to one edge per target": {
			bullet: "- composes-with: `alpha`, `beta`",
			want: []related.Edge{
				{Kind: related.ComposesWith, Target: "alpha"},
				{Kind: related.ComposesWith, Target: "beta"},
			},
		},
		"no rationale": {
			bullet: "- depends-on: `alpha`",
			want:   []related.Edge{{Kind: related.DependsOn, Target: "alpha"}},
		},
		"asterisk list marker": {
			bullet: "* depends-on: `alpha` — because",
			want: []related.Edge{
				{Kind: related.DependsOn, Target: "alpha", Rationale: "because"},
			},
		},

		// Regression: a first draft scanned the whole line for backticks and
		// manufactured edges to skills named "--force" and "--yes" out of a
		// rationale, which the verify graph gate then reported as dangling.
		"backticked code in the rationale is not a target": {
			bullet: "- composes-with: `alpha` — tier flags (`--force`, `--yes`) are stable",
			want: []related.Edge{
				{
					Kind:      related.ComposesWith,
					Target:    "alpha",
					Rationale: "tier flags (`--force`, `--yes`) are stable",
				},
			},
		},
		// Regression: unknown kinds must stay skipped. Both of these appear in real
		// trees and a first draft accepted the reversed one.
		"unknown kind in the reversed form yields nothing": {
			bullet: "- **alpha** (broader): because",
		},
		"unknown kind in the plain form yields nothing": {
			bullet: "- precedes: `alpha` — because",
		},
		// A bullet whose target is prose names no skill; inventing one would create
		// an edge the graph gate would report as dangling.
		"parenthesised prose yields nothing": {
			bullet: "- contrasts-with: (traditional headcount-scaling model)",
		},
		"capitalised words are not a slug": {
			bullet: "- depends-on: Four Golden Signals",
		},
		"not a bullet at all": {
			bullet: "some prose about depends-on: `alpha`",
		},
		"thematic break is not a bullet": {
			bullet: "---",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			md := "## Related skills\n\n" + tc.bullet + "\n"
			got := related.ParseSection(md)
			if len(got) != len(tc.want) {
				t.Fatalf("ParseSection(%q) = %+v, want %+v", tc.bullet, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("edge %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseSectionHeadingVariants(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		heading string
		want    int
	}{
		"canonical heading": {heading: "## Related skills", want: 1},
		"suffixed heading is the section": {
			heading: "## Related skills (Stage 3 Filling)",
			want:    1,
		},
		"capitalised heading is the section": {heading: "## Related Skills", want: 1},
		"capitalised and suffixed heading is the section": {
			heading: "## Related Skills (Stage 3 Filling)",
			want:    1,
		},
		"deeper level is not the section": {heading: "### Related skills", want: 0},
		"different word is not the section": {
			heading: "## Related skillset",
			want:    0,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			md := tc.heading + "\n\n- depends-on: `alpha` — because\n"
			if got := len(related.ParseSection(md)); got != tc.want {
				t.Errorf("%s: parsed %d edges, want %d", tc.heading, got, tc.want)
			}
		})
	}
}

func TestParseSectionDedupesRepeatedEdges(t *testing.T) {
	t.Parallel()
	// The state after `relate` runs over a legacy section: a legacy bullet and the
	// canonical bullet it was rewritten as both name the same relationship.
	md := "## Related skills\n\n" +
		"- **composes-with** [`alpha`](../alpha/SKILL.md): legacy wording\n" +
		"- composes-with: `alpha` — canonical wording\n"
	got := related.ParseSection(md)
	if len(got) != 1 {
		t.Fatalf("expected the relationship once, got %+v", got)
	}
	if got[0].Rationale != "legacy wording" {
		t.Errorf("first occurrence should win, got %q", got[0].Rationale)
	}
}

func TestUpsertOverLegacySectionStaysIdempotent(t *testing.T) {
	t.Parallel()
	// The write path must not have regressed: Upsert matches only canonical bullets,
	// so it appends beside a legacy one, and a second identical Upsert is a no-op.
	md := "# Skill\n\n## Related skills (Stage 3 Filling)\n\n" +
		"- **composes-with** [`alpha`](../alpha/SKILL.md): legacy wording\n"
	edge := related.Edge{Kind: related.DependsOn, Target: "beta", Rationale: "why"}

	first, changed := related.Upsert(md, edge)
	if !changed {
		t.Fatal("first Upsert must change the section")
	}
	second, changedAgain := related.Upsert(first, edge)
	if changedAgain {
		t.Errorf("second Upsert must be a no-op, got:\n%s", second)
	}
	// It must write into the suffixed section, not append a second one.
	if n := countHeadings(first); n != 1 {
		t.Errorf("expected exactly one related-skills heading, got %d:\n%s", n, first)
	}
	// The legacy edge must survive the write untouched.
	edges := related.ParseSection(first)
	if len(edges) != 2 {
		t.Fatalf("expected the legacy and new edge, got %+v", edges)
	}
}

// countHeadings counts the `## Related skills...` headings in md.
func countHeadings(md string) int {
	n := 0
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "## Related skills") {
			n++
		}
	}
	return n
}
