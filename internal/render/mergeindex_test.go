package render_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/render"
)

func TestMergeIndexRenders(t *testing.T) {
	t.Parallel()
	mi := &book2skill.MergeIndex{
		RunSlug: "decisions",
		Sources: []book2skill.MergeSourceBook{
			{
				Slug: "munger", Title: "Poor Charlie's Almanack", Author: "Munger",
				Skills:     []string{"inversion", "circle-of-competence"},
				Superseded: map[string]bool{"inversion": true},
				Edges: []book2skill.Relationship{
					{From: "circle-of-competence", To: "inversion", Kind: book2skill.DependsOn},
				},
			},
			{
				Slug:   "aurelius",
				Title:  "Meditations",
				Author: "Aurelius",
				Skills: []string{
					"control-dichotomy",
				},
				Superseded: map[string]bool{"control-dichotomy": true},
			},
		},
		Verification: []book2skill.VerificationRow{
			{
				Pair: "inversion-vs-control",
				R: []book2skill.VerificationSource{
					{Book: "munger", Skill: "inversion", Status: book2skill.StatusAccurate},
					{
						Book: "aurelius", Skill: "control-dichotomy",
						Status: book2skill.StatusDriftedMinor, Corrected: true,
					},
				},
				A1: []book2skill.VerificationSource{
					{Book: "munger", Skill: "inversion", Status: book2skill.StatusVerified},
				},
				Validations: "all pass",
			},
		},
		Merges: []book2skill.MergeRecord{{
			Slug: "inversion-and-control", Title: "Inversion and Control",
			Parents: []book2skill.MergeParent{
				{BookSlug: "munger", SkillSlug: "inversion", State: book2skill.StateMerged},
				{
					BookSlug:  "aurelius",
					SkillSlug: "control-dichotomy",
					State:     book2skill.StateMerged,
				},
			},
			Edges: []book2skill.Relationship{{
				From: "inversion-and-control", To: "circle-of-competence",
				Kind: book2skill.ComposesWith,
			}},
		}},
	}
	out := render.MergeIndex(mi)

	for _, want := range []string{
		"# Merged Skills Index — decisions",
		"## Source Books",
		"## Provenance",
		"## Cross-Book Skill Graph",
		"## Superseded Source Skills",
		"[`inversion-and-control`](./inversion-and-control/SKILL.md)",
		"-->|superseded-by|",
		"-->|depends-on| s_munger_inversion",
		"-->|composes-with| s_munger_circle_of_competence",
		"classDef merged",
		"`munger/inversion`",
		"## Source Verification Summary",
		"aurelius/control-dichotomy: drifted-minor (corrected)",
		"| `inversion-vs-control` |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("MergeIndex output missing %q\n---\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n") {
		t.Error("output should end in exactly one newline")
	}
}
