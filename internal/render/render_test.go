package render_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/render"
)

func sampleSkill() *book2skill.Skill {
	return &book2skill.Skill{
		Slug:           "inversion-thinking",
		Title:          "Inversion Thinking",
		Description:    "Invoke when a user is stuck on a decision and keeps listing reasons for.",
		Tags:           []string{"decision", "mental-model"},
		Reading:        book2skill.Reading{Quote: "Invert, always invert.", Attribution: "Jacobi"},
		Interpretation: "Ask what would guarantee failure, then avoid it.",
		Application: []book2skill.ApplicationCase{{
			Name: "Avoiding ruin", Problem: "risk", MethodologyUse: "listed failures",
			Conclusion: "avoid them", Result: "survived",
		}},
		Trigger: book2skill.Trigger{
			Scenarios:       []string{"stuck on a decision"},
			LanguageSignals: []string{"how do I succeed at X"},
		},
		Execution: []book2skill.Step{{
			Text: "List failure modes", CompletionCriterion: "at least three listed",
		}},
		Boundary:   book2skill.Boundary{AntiScenarios: []string{"pure information lookup"}},
		Provenance: "Poor Charlie's Almanack by Charlie Munger",
	}
}

// TestSkillRoundTrip renders a skill and confirms every RIA++ segment is
// recoverable via the ParseSegments contract (D3).
func TestSkillRoundTrip(t *testing.T) {
	t.Parallel()
	md := render.Skill(sampleSkill())

	body := md
	if _, after, ok := strings.Cut(md, "\n---\n"); ok {
		body = after // drop YAML frontmatter before parsing segments
	}
	segments := book2skill.ParseSegments(body)
	for _, tag := range book2skill.SegmentTags() {
		if _, ok := segments[tag]; !ok {
			t.Errorf("rendered skill missing segment %q\n---\n%s", tag, md)
		}
	}
	if !strings.Contains(md, "name: inversion-thinking") {
		t.Error("frontmatter missing name")
	}
	if !strings.Contains(md, "> Invert, always invert.") {
		t.Error("R segment missing quote")
	}
}

// TestTitleAndRelatedRoundTrip locks render.Skill and the book2skill parsers to
// the same SKILL.md format: what render writes, ParseTitle and ParseRelated must
// read back. It is the anti-drift guard for the index command.
func TestTitleAndRelatedRoundTrip(t *testing.T) {
	t.Parallel()
	s := sampleSkill()
	s.Related = []book2skill.Relationship{
		{From: s.Slug, To: "first-principles", Kind: book2skill.DependsOn, Rationale: "grounds it"},
		{From: s.Slug, To: "analogy", Kind: book2skill.ContrastsWith, Rationale: "opposite mode"},
	}
	md := render.Skill(s)

	if got := book2skill.ParseTitle(md); got != s.Title {
		t.Errorf("ParseTitle = %q, want %q", got, s.Title)
	}
	got := book2skill.ParseRelated(s.Slug, md)
	if len(got) != len(s.Related) {
		t.Fatalf(
			"ParseRelated recovered %d relationships, want %d: %+v",
			len(got),
			len(s.Related),
			got,
		)
	}
	for i := range s.Related {
		if got[i] != s.Related[i] {
			t.Errorf("relationship %d = %+v, want %+v", i, got[i], s.Related[i])
		}
	}
}

// TestBookOverviewRoundTrip locks render.BookOverview and
// book2skill.ParseBookOverview to one schema: a rendered overview that passes
// the Stage-0 gate must still pass after being parsed back. This is what makes
// gating a hand-authored BOOK_OVERVIEW.md deterministic.
func TestBookOverviewRoundTrip(t *testing.T) {
	t.Parallel()
	o := &book2skill.BookOverview{
		Title: "Poor Charlie's Almanack", Author: "Charlie Munger", Year: "2005",
		Structure: book2skill.Structure{
			Genre:              "essays",
			OneSentenceSummary: "Worldly wisdom via a latticework of mental models.",
			Skeleton:           []string{"multidisciplinary models", "inversion", "incentives"},
		},
		Interpretation: book2skill.Interpretation{KeyTerms: []book2skill.KeyTerm{
			{Term: "latticework"},
			{Term: "circle of competence"},
			{Term: "lollapalooza"},
			{Term: "inversion"},
			{Term: "incentive-caused bias"},
		}},
		Critique: book2skill.Critique{
			EraLimitations:      []string{"pre-2008 finance"},
			AuthorBlindSpots:    []string{"survivorship"},
			UnprovenAssumptions: []string{"rationality is teachable"},
		},
	}
	if problems := o.QualityGate(); len(problems) != 0 {
		t.Fatalf("fixture should pass the gate first: %v", problems)
	}

	parsed := book2skill.ParseBookOverview(render.BookOverview(o))
	if parsed.Title != o.Title || parsed.Author != o.Author {
		t.Errorf("header lost: (%q, %q)", parsed.Title, parsed.Author)
	}
	if problems := parsed.QualityGate(); len(problems) != 0 {
		t.Errorf("round-tripped overview should still pass the gate, got %v", problems)
	}
}

func TestLearningOrderRespectsDependencies(t *testing.T) {
	t.Parallel()
	skills := []book2skill.Skill{
		{Slug: "advanced", Related: []book2skill.Relationship{
			{From: "advanced", To: "basic", Kind: book2skill.DependsOn},
		}},
		{Slug: "basic"},
	}
	order := render.LearningOrder(skills)
	if len(order) != 2 {
		t.Fatalf("order = %v, want 2 entries", order)
	}
	if order[0] != "basic" || order[1] != "advanced" {
		t.Errorf("order = %v, want [basic advanced]", order)
	}
}

func TestWrapTextInDescriptionStaysIndented(t *testing.T) {
	t.Parallel()
	s := sampleSkill()
	s.Description = strings.Repeat("word ", 40)
	md := render.Skill(s)
	// Every folded description line must be indented by two spaces under
	// "description: >-".
	inDesc := false
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "description:") {
			inDesc = true
			continue
		}
		if inDesc {
			if strings.HasPrefix(line, "tags:") || line == "---" {
				break
			}
			if !strings.HasPrefix(line, "  ") {
				t.Fatalf("folded description line not indented: %q", line)
			}
		}
	}
}
