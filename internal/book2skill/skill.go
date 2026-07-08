package book2skill

import "strconv"

// RelationshipKind values for the Phase-3 Zettelkasten graph.
const (
	// DependsOn means using the source skill requires understanding the target.
	DependsOn RelationshipKind = "depends-on"
	// ContrastsWith means the two skills are alternatives chosen by context.
	ContrastsWith RelationshipKind = "contrasts-with"
	// ComposesWith means the two skills are commonly used together.
	ComposesWith RelationshipKind = "composes-with"
	// SupersededBy means a merged skill (merge-skills) replaces this one; the
	// target is the merged skill. It is a navigation pointer, not a learning
	// dependency, so LearningOrder does not follow it.
	SupersededBy RelationshipKind = "superseded-by"
)

// RelationshipKind classifies an edge between two skills.
type RelationshipKind string

// Reading is the R segment: a source quote and its attribution.
type Reading struct {
	Quote       string `json:"quote"`
	Attribution string `json:"attribution"`
}

// ApplicationCase is one A1 case: the author's own use of the methodology.
type ApplicationCase struct {
	Name           string `json:"name"`
	Problem        string `json:"problem"`
	MethodologyUse string `json:"methodology_use"`
	Conclusion     string `json:"conclusion"`
	Result         string `json:"result"`
}

// AdjacentDistinction names a neighbouring skill and how this skill differs.
type AdjacentDistinction struct {
	Skill      string `json:"skill"`
	Difference string `json:"difference"`
}

// Trigger is the A2 segment: when the skill should fire.
type Trigger struct {
	Scenarios            []string              `json:"scenarios"`
	LanguageSignals      []string              `json:"language_signals"`
	AdjacentDistinctions []AdjacentDistinction `json:"adjacent_distinctions"`
}

// Step is one E-segment execution step with its completion and stop conditions.
type Step struct {
	Text                string `json:"text"`
	CompletionCriterion string `json:"completion_criterion"`
	StopCondition       string `json:"stop_condition,omitempty"`
}

// Boundary is the B segment: when not to use the skill.
type Boundary struct {
	AntiScenarios        []string `json:"anti_scenarios"`
	AuthorWarnedFailures []string `json:"author_warned_failures"`
	AuthorBlindSpots     []string `json:"author_blind_spots"`
	ConfusableNeighbors  []string `json:"confusable_neighbors"`
}

// Relationship is one edge in the Phase-3 skill graph.
type Relationship struct {
	From      string           `json:"from"`
	To        string           `json:"to"`
	Kind      RelationshipKind `json:"kind"`
	Rationale string           `json:"rationale,omitempty"`
}

// Skill is a fully constructed RIA++ skill, ready to render as SKILL.md.
type Skill struct {
	Slug           string            `json:"slug"`
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	Tags           []string          `json:"tags"`
	Reading        Reading           `json:"reading"`
	Interpretation string            `json:"interpretation"`
	Application    []ApplicationCase `json:"application"`
	Trigger        Trigger           `json:"trigger"`
	Execution      []Step            `json:"execution"`
	Boundary       Boundary          `json:"boundary"`
	Provenance     string            `json:"provenance"`
	Related        []Relationship    `json:"related"`
}

// RelationshipCountAdvice returns a heuristic warning when the total number of
// declared relationships is out of band for the number of skills, or "" when
// the count is reasonable or there are too few skills to judge. The band is
// [ceil(0.8n), floor(1.5n)] relationships for n skills, generalizing
// methodology/05's "~8–15 for 10 skills" rule of thumb. It never fails a build;
// it only advises.
func RelationshipCountAdvice(skills []Skill) string {
	const minSkills = 4 // too few skills to judge the relationship count
	n := len(skills)
	if n < minSkills {
		return ""
	}
	total := 0
	for i := range skills {
		total += len(skills[i].Related)
	}
	low := (n*8 + 9) / 10 // ceil(0.8n)
	high := n * 3 / 2     // floor(1.5n)
	switch {
	case total < low:
		return "only " + strconv.Itoa(total) + " relationships across " + strconv.Itoa(n) +
			" skills — the breakdown may be too independent; recheck unit selection"
	case total > high:
		return strconv.Itoa(total) + " relationships across " + strconv.Itoa(n) +
			" skills — some may be artificial; keep only genuine dependencies"
	default:
		return ""
	}
}

// Valid reports whether k is one of the known relationship kinds.
func (k RelationshipKind) Valid() bool {
	switch k {
	case DependsOn, ContrastsWith, ComposesWith, SupersededBy:
		return true
	default:
		return false
	}
}
