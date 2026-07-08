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
		"classDef merged",
		"`munger/inversion`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("MergeIndex output missing %q\n---\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n") {
		t.Error("output should end in exactly one newline")
	}
}
