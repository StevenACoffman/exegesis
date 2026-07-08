package book2skill_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

func TestParseBookOverviewGate(t *testing.T) {
	t.Parallel()
	good := "# Meditations — Book Overview\n\n" +
		"- **Author:** Marcus Aurelius\n\n" +
		"## One-sentence summary\n\nHow to live with reason and virtue.\n\n" +
		"## Skeleton\n\n- control vs not\n- memento mori\n- duty\n\n" +
		"**Argument relationship:** progressive\n\n" +
		"## Key terms\n\n- **logos:** cosmic reason\n- **prohairesis:** choice\n" +
		"- **apatheia:** equanimity\n- **kathekon:** proper action\n- **oikeiosis:** affinity\n\n" +
		"## Era limitations\n\n- slavery unquestioned\n\n" +
		"## Author blind spots\n\n- elite vantage\n\n" +
		"## Unproven assumptions\n\n- cosmic order exists\n"

	o := book2skill.ParseBookOverview(good)
	if o.Title != "Meditations" || o.Author != "Marcus Aurelius" {
		t.Errorf("header = (%q, %q)", o.Title, o.Author)
	}
	if len(o.Structure.Skeleton) != 3 || len(o.Interpretation.KeyTerms) != 5 {
		t.Errorf(
			"skeleton=%d keyterms=%d",
			len(o.Structure.Skeleton),
			len(o.Interpretation.KeyTerms),
		)
	}
	if o.Interpretation.KeyTerms[0].Term != "logos" {
		t.Errorf("key term 0 = %q, want logos", o.Interpretation.KeyTerms[0].Term)
	}
	if problems := o.QualityGate(); len(problems) != 0 {
		t.Fatalf("well-formed overview should pass the gate, got %v", problems)
	}

	// Title-cased headings (as a formatter's heading-case rule produces) must
	// still parse and pass the gate.
	titleCased := strings.NewReplacer(
		"## One-sentence summary", "## One-Sentence Summary",
		"## Key terms", "## Key Terms",
		"## Era limitations", "## Era Limitations",
		"## Author blind spots", "## Author Blind Spots",
		"## Unproven assumptions", "## Unproven Assumptions",
	).Replace(good)
	tc := book2skill.ParseBookOverview(titleCased)
	if problems := tc.QualityGate(); len(problems) != 0 {
		t.Fatalf("title-cased overview should still pass the gate, got %v", problems)
	}

	// Drop the key-terms section: the gate must fail (needs ≥5).
	deficient := strings.ReplaceAll(good, "## Key terms", "## Other")
	parsed := book2skill.ParseBookOverview(deficient)
	if problems := parsed.QualityGate(); len(problems) == 0 {
		t.Error("overview missing key terms should fail the gate")
	}
}

func TestRelationshipCountAdvice(t *testing.T) {
	t.Parallel()
	skillsWith := func(n, relsEach int) []book2skill.Skill {
		skills := make([]book2skill.Skill, n)
		for i := range skills {
			skills[i].Related = make([]book2skill.Relationship, relsEach)
		}
		return skills
	}
	cases := map[string]struct {
		skills   []book2skill.Skill
		wantWarn bool
	}{
		"too few skills to judge": {skillsWith(3, 0), false},
		"in band (n=10, r=10)":    {skillsWith(10, 1), false},
		"too independent":         {skillsWith(10, 0), true},
		"too dense":               {skillsWith(10, 3), true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := book2skill.RelationshipCountAdvice(tc.skills)
			if (got != "") != tc.wantWarn {
				t.Errorf("RelationshipCountAdvice = %q, wantWarn=%v", got, tc.wantWarn)
			}
		})
	}
}

func TestParseOverviewHeader(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		md         string
		wantTitle  string
		wantAuthor string
	}{
		"full header": {
			"# Poor Charlie's Almanack — Book Overview\n\n" +
				"- **Author:** Charlie Munger\n- **Year:** 2005\n",
			"Poor Charlie's Almanack", "Charlie Munger",
		},
		"title only": {"# Meditations — Book Overview\n\nbody", "Meditations", ""},
		"empty":      {"", "", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gotTitle, gotAuthor := book2skill.ParseOverviewHeader(tc.md)
			if gotTitle != tc.wantTitle || gotAuthor != tc.wantAuthor {
				t.Errorf("ParseOverviewHeader = (%q, %q), want (%q, %q)",
					gotTitle, gotAuthor, tc.wantTitle, tc.wantAuthor)
			}
		})
	}
}
