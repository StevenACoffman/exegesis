package related_test

import (
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
