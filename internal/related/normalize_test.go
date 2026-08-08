package related_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/related"
)

func TestNormalize(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in          string
		want        string
		wantChanged bool
	}{
		"a lowercase heading on disk is canonicalised": {
			in:          "## Related skills\n\n- depends-on: `alpha` — because\n",
			want:        "## Related Skills\n\n- depends-on: `alpha` — because\n",
			wantChanged: true,
		},
		"canonical section is already normal": {
			in:   "## Related Skills\n\n- depends-on: `alpha` — because\n",
			want: "## Related Skills\n\n- depends-on: `alpha` — because\n",
		},
		"bold kind with linked target becomes canonical": {
			in: "## Related Skills\n\n" +
				"- **composes-with** → [`alpha`](../alpha/SKILL.md): because\n",
			want:        "## Related Skills\n\n- composes-with: `alpha` — because\n",
			wantChanged: true,
		},
		"reversed form becomes canonical": {
			in:          "## Related Skills\n\n- **alpha** (contrasts-with): because\n",
			want:        "## Related Skills\n\n- contrasts-with: `alpha` — because\n",
			wantChanged: true,
		},
		"bare token becomes canonical": {
			in:          "## Related Skills\n\n- depends-on: alpha (because things)\n",
			want:        "## Related Skills\n\n- depends-on: `alpha` — (because things)\n",
			wantChanged: true,
		},
		// The 9-continuation-line risk: a wrapped rationale must survive whole.
		"wrapped rationale is folded, not truncated": {
			in: "## Related Skills\n\n" +
				"- **composes-with** [`alpha`](../alpha/SKILL.md): first part\n" +
				"  second part continues here\n",
			want:        "## Related Skills\n\n- composes-with: `alpha` — first part second part continues here\n",
			wantChanged: true,
		},
		// The 5-prose-bullet risk: a bullet naming no skill must not be deleted.
		"prose bullet is preserved verbatim": {
			in: "## Related Skills\n\n" +
				"- contrasts-with: (traditional headcount-scaling model)\n" +
				"- depends-on: `alpha` — because\n",
			want: "## Related Skills\n\n" +
				"- contrasts-with: (traditional headcount-scaling model)\n" +
				"- depends-on: `alpha` — because\n",
			// Nothing changes: the prose bullet is copied through and the other
			// bullet is already canonical.
			wantChanged: false,
		},
		"multi-target becomes one bullet per target": {
			in: "## Related Skills\n\n- composes-with: `alpha`, `beta`\n",
			want: "## Related Skills\n\n" +
				"- composes-with: `alpha` — \n" +
				"- composes-with: `beta` — \n",
			wantChanged: true,
		},
		"duplicate relationship collapses": {
			in: "## Related Skills\n\n" +
				"- **composes-with** [`alpha`](../alpha/SKILL.md): legacy wording\n" +
				"- composes-with: `alpha` — canonical wording\n",
			want:        "## Related Skills\n\n- composes-with: `alpha` — legacy wording\n",
			wantChanged: true,
		},
		"suffixed heading becomes canonical": {
			in:          "## Related skills (Stage 3 Filling)\n\n- depends-on: `alpha` — because\n",
			want:        "## Related Skills\n\n- depends-on: `alpha` — because\n",
			wantChanged: true,
		},
		"content outside the section is untouched": {
			in: "---\nname: s\n---\n\n# Body\n\nProse here.\n\n" +
				"## Related Skills\n\n- **alpha** (depends-on): why\n\n---\n\nTail prose.\n",
			want: "---\nname: s\n---\n\n# Body\n\nProse here.\n\n" +
				"## Related Skills\n\n- depends-on: `alpha` — why\n\n---\n\nTail prose.\n",
			wantChanged: true,
		},
		"intro sentence inside the section is kept": {
			in: "## Related Skills\n\nThese are the related skills:\n\n" +
				"- **alpha** (depends-on): why\n",
			want: "## Related Skills\n\nThese are the related skills:\n\n" +
				"- depends-on: `alpha` — why\n",
			wantChanged: true,
		},
		"no section is a no-op": {
			in:   "# Body\n\nNothing here.\n",
			want: "# Body\n\nNothing here.\n",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, changed := related.Normalize(tc.in)
			if got != tc.want {
				t.Errorf("Normalize mismatch\ngot:\n%s\nwant:\n%s", got, tc.want)
			}
			if changed != tc.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tc.wantChanged)
			}
		})
	}
}

func TestNormalizeIsIdempotent(t *testing.T) {
	t.Parallel()
	// Every dialect at once, so a second pass has plenty to be unstable about.
	in := "# Body\n\n## Related skills (Stage 3 Filling)\n\n" +
		"- **composes-with** → [`alpha`](../alpha/SKILL.md): first\n" +
		"  wrapped tail\n" +
		"- **beta** (depends-on): second\n" +
		"- contrasts-with: (prose, not a skill)\n" +
		"- depends-on: gamma (third)\n" +
		"- composes-with: `delta`, `epsilon`\n"

	once, changed := related.Normalize(in)
	if !changed {
		t.Fatal("first pass must change a legacy section")
	}
	twice, changedAgain := related.Normalize(once)
	if changedAgain {
		t.Errorf("second pass must be a no-op, got:\n%s", twice)
	}
	if twice != once {
		t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", once, twice)
	}
}

func TestNormalizePreservesTheEdgeSet(t *testing.T) {
	t.Parallel()
	// The safety net for the migration: normalizing must not change which edges the
	// section expresses, only how they are written.
	in := "## Related skills (Stage 3 Filling)\n\n" +
		"- **composes-with** → [`alpha`](../alpha/SKILL.md): first\n" +
		"  wrapped tail\n" +
		"- **beta** (depends-on): second\n" +
		"- contrasts-with: (prose, not a skill)\n" +
		"- depends-on: gamma (third)\n" +
		"- composes-with: `delta`, `epsilon`\n"

	before := related.ParseSection(in)
	out, _ := related.Normalize(in)
	after := related.ParseSection(out)

	if len(before) != len(after) {
		t.Fatalf("edge count changed: %d -> %d\nbefore: %+v\nafter: %+v",
			len(before), len(after), before, after)
	}
	for i := range before {
		if before[i].Kind != after[i].Kind || before[i].Target != after[i].Target {
			t.Errorf("edge %d changed: %+v -> %+v", i, before[i], after[i])
		}
	}
}

func TestNormalizeMergesASecondSection(t *testing.T) {
	t.Parallel()
	// The shape of all 13 two-section skills in the real books.
	in := "# Body\n\n## Related skills (Stage 3 Filling)\n\n" +
		"- depends-on: `alpha` — first\n" +
		"- contrasts-with: (prose, not a skill)\n\n" +
		"---\n\n" +
		"## Related Skills\n\n" +
		"- composes-with: `beta` — second\n" +
		"- depends-on: `alpha` — restated in other words\n\n" +
		"---\n\n" +
		"## Audit Information\n"
	want := "# Body\n\n## Related Skills\n\n" +
		"- depends-on: `alpha` — first\n" +
		"- contrasts-with: (prose, not a skill)\n" +
		"- composes-with: `beta` — second\n\n" +
		"---\n\n" +
		"## Audit Information\n"

	got, changed := related.Normalize(in)
	if !changed {
		t.Fatal("expected a two-section file to change")
	}
	if got != want {
		t.Errorf("merge mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
	if again, changedAgain := related.Normalize(got); changedAgain || again != got {
		t.Errorf("merging must be idempotent, second pass:\n%s", again)
	}
}

func TestNormalizeMergeKeepsEveryEdgeAndTheFirstRationale(t *testing.T) {
	t.Parallel()
	in := "## Related Skills\n\n- depends-on: `alpha` — the reason that was written first\n\n" +
		"## Related Skills\n\n- depends-on: `alpha` — a later, different reason\n" +
		"- composes-with: `beta` — only in the second section\n"

	out, _ := related.Normalize(in)
	if strings.Count(out, "## Related Skills") != 1 {
		t.Errorf("expected one section after merging, got:\n%s", out)
	}
	if strings.Count(out, "`alpha`") != 1 {
		t.Errorf("the repeated relationship should be written once, got:\n%s", out)
	}
	if !strings.Contains(out, "the reason that was written first") {
		t.Errorf("the first section's rationale should win, got:\n%s", out)
	}
	if !strings.Contains(out, "- composes-with: `beta` — only in the second section") {
		t.Errorf("an edge unique to the second section was lost:\n%s", out)
	}
}

func TestNormalizeLeavesASecondSectionHoldingProseWhereItIs(t *testing.T) {
	t.Parallel()
	// Merging would have to delete the paragraph or move it somewhere it does not
	// belong. Leaving the section in place is the smaller harm, and it is still
	// canonicalised where it can be.
	in := "## Related Skills\n\n- depends-on: `alpha` — first\n\n" +
		"## Related skills (Stage 3 Filling)\n\n" +
		"These edges were added by hand and still need review.\n\n" +
		"- **composes-with** → [`beta`](../beta/SKILL.md): second\n"

	out, _ := related.Normalize(in)
	if strings.Count(out, "## Related Skills") != 2 {
		t.Errorf("a section holding prose must not be merged away, got:\n%s", out)
	}
	if !strings.Contains(out, "These edges were added by hand and still need review.") {
		t.Errorf("prose was lost:\n%s", out)
	}
	if !strings.Contains(out, "- composes-with: `beta` — second") {
		t.Errorf("the unmerged section was not canonicalised:\n%s", out)
	}
	if again, changedAgain := related.Normalize(out); changedAgain || again != out {
		t.Errorf("the unmerged case must still be idempotent, second pass:\n%s", again)
	}
}

func TestNormalizeMergeMovesABulletItCannotParse(t *testing.T) {
	t.Parallel()
	// The real second sections write "**depends_on**" with an underscore, which is not
	// a known kind. Those lines carry the only rationale anyone wrote, so they move
	// verbatim rather than being dropped with the heading.
	in := "## Related Skills\n\n- depends-on: `alpha` — first\n\n" +
		"## Related Skills\n\n- **depends_on**: alpha — a kind spelt with an underscore\n"

	out, _ := related.Normalize(in)
	if strings.Count(out, "## Related Skills") != 1 {
		t.Errorf("expected one section after merging, got:\n%s", out)
	}
	if !strings.Contains(out, "- **depends_on**: alpha — a kind spelt with an underscore") {
		t.Errorf("an unparsed bullet was dropped instead of moved:\n%s", out)
	}
}
