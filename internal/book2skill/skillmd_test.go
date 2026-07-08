package book2skill_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

func TestSegmentTagFromHeading(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		line    string
		wantTag string
		wantOK  bool
	}{
		"plain tag":         {"## R", book2skill.SegR, true},
		"decorated tag":     {"## A1 — Past Application (from the book)", book2skill.SegA1, true},
		"leading space":     {"  ## E — Execution", book2skill.SegE, true},
		"not a heading":     {"some body text", "", false},
		"level-3 heading":   {"### R", "", false},
		"unknown tag":       {"## X — Something", "", false},
		"prefix-only match": {"## Rationale", "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gotTag, gotOK := book2skill.SegmentTagFromHeading(tc.line)
			if gotTag != tc.wantTag || gotOK != tc.wantOK {
				t.Errorf("SegmentTagFromHeading(%q) = (%q, %v), want (%q, %v)",
					tc.line, gotTag, gotOK, tc.wantTag, tc.wantOK)
			}
		})
	}
}

func TestParseSegments(t *testing.T) {
	t.Parallel()
	md := "# Title\n\npreamble ignored\n\n" +
		"## R — Original text\n> a quote\n\n" +
		"## I — Interpretation\nthe framework\n\n" +
		"## E — Execution\n1. do the thing\n"

	got := book2skill.ParseSegments(md)

	want := map[string]string{
		book2skill.SegR: "> a quote",
		book2skill.SegI: "the framework",
		book2skill.SegE: "1. do the thing",
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %d segments, want %d: %v", len(got), len(want), got)
	}
	for tag, wantBody := range want {
		if got[tag] != wantBody {
			t.Errorf("segment %q = %q, want %q", tag, got[tag], wantBody)
		}
	}
	// Preamble before the first heading must not appear under any segment.
	for tag, body := range got {
		if body == "preamble ignored" {
			t.Errorf("segment %q captured preamble text", tag)
		}
	}
}

func TestParseTitle(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		md   string
		want string
	}{
		"after frontmatter": {
			"---\nname: x\n---\n\n# Inversion Thinking\n\nbody",
			"Inversion Thinking",
		},
		"first of many": {"# First\n\n# Second", "First"},
		"trims space":   {"#   Padded  \n", "Padded"},
		"no heading":    {"just body text", ""},
		"not level-1":   {"## Section", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := book2skill.ParseTitle(tc.md); got != tc.want {
				t.Errorf("ParseTitle(%q) = %q, want %q", tc.md, got, tc.want)
			}
		})
	}
}

func TestParseRelated(t *testing.T) {
	t.Parallel()
	md := "# Skill\n\n## Related skills\n\n" +
		"- depends-on: `first-principles` — grounds this skill\n" +
		"- contrasts-with: `analogy` — opposite mode\n" +
		"- bogus-kind: `ignored` — unknown kind is skipped\n" +
		"- composes-with: `checklists`\n\n" +
		"## Provenance\n\n- depends-on: `not-in-section` — outside the section\n"

	got := book2skill.ParseRelated("inversion", md)
	if len(got) != 3 {
		t.Fatalf("parsed %d relationships, want 3: %+v", len(got), got)
	}
	assertRel(
		t,
		got[0],
		"inversion",
		"first-principles",
		book2skill.DependsOn,
		"grounds this skill",
	)
	assertRel(t, got[1], "inversion", "analogy", book2skill.ContrastsWith, "opposite mode")
	assertRel(t, got[2], "inversion", "checklists", book2skill.ComposesWith, "")
}

func TestParseRelatedNoSection(t *testing.T) {
	t.Parallel()
	if got := book2skill.ParseRelated("x", "# Skill\n\nno related section here\n"); got != nil {
		t.Errorf("ParseRelated with no section = %+v, want nil", got)
	}
}

// TestParseRelatedDecoratedHeading ensures a decorative suffix on the heading
// (as the SKILL.md template writes) is still recognized, and a bullet without a
// rationale is parsed with an empty Rationale.
func TestParseRelatedDecoratedHeading(t *testing.T) {
	t.Parallel()
	md := "# Skill\n\n## Related skills (Stage 3 Filling)\n\n" +
		"- depends-on: `first-principles`\n"
	got := book2skill.ParseRelated("second-order", md)
	if len(got) != 1 {
		t.Fatalf("parsed %d relationships, want 1: %+v", len(got), got)
	}
	assertRel(t, got[0], "second-order", "first-principles", book2skill.DependsOn, "")
}

// TestParseRelatedTitleCasedHeading proves the parser tolerates a markdown
// formatter's title-cased heading ("## Related Skills" from "## Related skills").
func TestParseRelatedTitleCasedHeading(t *testing.T) {
	t.Parallel()
	md := "# Skill\n\n## Related Skills\n\n- depends-on: `first-principles` — grounds it\n"
	got := book2skill.ParseRelated("inversion", md)
	if len(got) != 1 {
		t.Fatalf("parsed %d relationships, want 1: %+v", len(got), got)
	}
	assertRel(t, got[0], "inversion", "first-principles", book2skill.DependsOn, "grounds it")
}

func assertRel(
	t *testing.T,
	got book2skill.Relationship,
	from, to string,
	kind book2skill.RelationshipKind,
	rationale string,
) {
	t.Helper()
	if got.From != from || got.To != to || got.Kind != kind || got.Rationale != rationale {
		t.Errorf("relationship = %+v, want {From:%s To:%s Kind:%s Rationale:%q}",
			got, from, to, kind, rationale)
	}
}

func TestRSegmentQuotesAndQuoteFound(t *testing.T) {
	t.Parallel()
	// Dual-citation R segment, as a merged skill uses: two blockquote runs, each
	// with an em-dash attribution line that must be dropped.
	md := "# Merged\n\n## R — Dual-Citation Reading\n\n" +
		"> Invert, always\n> invert.\n>\n> — Jacobi\n\n" +
		"> Know the edge of\n> your competence.\n>\n> — Munger\n\n" +
		"## I — Interpretation\n\nbody\n"
	quotes := book2skill.RSegmentQuotes(md)
	if len(quotes) != 2 {
		t.Fatalf("expected 2 quotes, got %d: %q", len(quotes), quotes)
	}
	if quotes[0] != "Invert, always invert." {
		t.Errorf("quote 0 = %q", quotes[0])
	}
	if strings.Contains(quotes[0], "Jacobi") {
		t.Errorf("attribution leaked into quote: %q", quotes[0])
	}

	// QuoteFound tolerates whitespace/wrapping differences but not absence.
	source := "Blah blah.  Invert,\n   always invert.  More text about competence."
	if !book2skill.QuoteFound(quotes[0], source) {
		t.Error("quote 0 should be found despite different wrapping")
	}
	if book2skill.QuoteFound(quotes[1], source) {
		t.Error("quote 1 is not in the source and must not be found")
	}
}

func TestAppendRelated(t *testing.T) {
	t.Parallel()
	// Creates the section when absent.
	md := "# Skill\n\n## Provenance\n\n- **Source:** book\n"
	out, changed := book2skill.AppendRelated(md, book2skill.Relationship{
		Kind: book2skill.SupersededBy, To: "merged-x", Rationale: "replaced",
	})
	if !changed {
		t.Fatal("expected change when creating the section")
	}
	rels := book2skill.ParseRelated("src", out)
	if len(rels) != 1 || rels[0].To != "merged-x" || rels[0].Kind != book2skill.SupersededBy {
		t.Fatalf("relationship not recovered: %+v", rels)
	}
	if !strings.Contains(out, "## Provenance") {
		t.Errorf("original content lost:\n%s", out)
	}

	// Idempotent: re-appending the same kind+target is a no-op.
	again, changed := book2skill.AppendRelated(out, book2skill.Relationship{
		Kind: book2skill.SupersededBy, To: "merged-x", Rationale: "different rationale",
	})
	if changed || again != out {
		t.Errorf("expected idempotent no-op for a duplicate (kind,to)")
	}

	// A different target adds a second bullet into the existing section.
	two, changed := book2skill.AppendRelated(out, book2skill.Relationship{
		Kind: book2skill.DependsOn, To: "first-principles",
	})
	if !changed || len(book2skill.ParseRelated("src", two)) != 2 {
		t.Errorf("expected a second relationship, got:\n%s", two)
	}
}

func TestLanguageSignalsAndA2Sharpness(t *testing.T) {
	t.Parallel()
	merged := "# M\n\n## A2 — Trigger\n\n### Language Signals\n\n" +
		"- \"invert the goal\"\n- \"what guarantees failure\"\n- \"pre-mortem\"\n\n" +
		"### Distinct from Adjacent Skills\n\n- other\n\n## E — Execution\n\n1. x\n"
	sig := book2skill.LanguageSignals(merged)
	if len(sig) != 3 || sig[0] != "invert the goal" {
		t.Fatalf("LanguageSignals = %q", sig)
	}

	sourceA := "# A\n\n## A2 — Trigger\n\n### Language Signals\n\n- \"invert the goal\"\n"
	sourceB := "# B\n\n## A2 — Trigger\n\n### Language Signals\n\n- \"what guarantees failure\"\n"
	unique := book2skill.A2Sharpness(merged, []string{sourceA, sourceB})
	// Only "pre-mortem" is absent from both sources.
	if len(unique) != 1 || unique[0] != "pre-mortem" {
		t.Errorf("A2Sharpness = %q, want [pre-mortem]", unique)
	}
}

func TestSegmentTagsOrderAndIsolation(t *testing.T) {
	t.Parallel()
	want := []string{
		book2skill.SegR, book2skill.SegI, book2skill.SegA1,
		book2skill.SegA2, book2skill.SegE, book2skill.SegB,
	}
	got := book2skill.SegmentTags()
	if len(got) != len(want) {
		t.Fatalf("SegmentTags length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SegmentTags()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Mutating the returned slice must not affect a subsequent call.
	got[0] = "mutated"
	if book2skill.SegmentTags()[0] != book2skill.SegR {
		t.Error("SegmentTags returned shared mutable state")
	}
}
