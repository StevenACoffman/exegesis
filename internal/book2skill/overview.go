package book2skill

import "strings"

// Stage-0 quality-gate thresholds (spec §7.1).
const (
	minSkeleton = 3
	maxSkeleton = 7
	minKeyTerms = 5
	minCritique = 3
)

// BookOverview is the Stage-0 analytical-reading result for one book. It is the
// global context every later stage reads, and its Critique feeds each skill's
// Boundary section.
type BookOverview struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Year   string `json:"year"`

	Structure      Structure      `json:"structure"`
	Interpretation Interpretation `json:"interpretation"`
	Critique       Critique       `json:"critique"`
	Applicability  Applicability  `json:"applicability"`
}

// Structure captures the book's skeleton (Adler step 1).
type Structure struct {
	Genre                string   `json:"genre"`
	OneSentenceSummary   string   `json:"one_sentence_summary"`
	Skeleton             []string `json:"skeleton"`
	ArgumentRelationship string   `json:"argument_relationship"`
	CoreProblem          string   `json:"core_problem"`
}

// KeyTerm records one term in the author's own usage (Adler step 2).
type KeyTerm struct {
	Term              string `json:"term"`
	AuthorDefinition  string `json:"author_definition"`
	DiffersFromCommon string `json:"differs_from_common"`
}

// Interpretation captures the book's claims and key terms (Adler step 2).
type Interpretation struct {
	KeyTerms         []KeyTerm `json:"key_terms"`
	CorePropositions []string  `json:"core_propositions"`
	ArgumentChain    string    `json:"argument_chain"`
}

// Critique captures the book's limitations (Adler step 3); it is the sole source
// for each skill's Boundary section.
type Critique struct {
	EraLimitations      []string `json:"era_limitations"`
	AuthorBlindSpots    []string `json:"author_blind_spots"`
	UnprovenAssumptions []string `json:"unproven_assumptions"`
	StrongestObjection  string   `json:"strongest_objection"`
}

// Applicability captures what in the book is worth turning into skills (Adler
// step 4, added by this pipeline).
type Applicability struct {
	SkillableTopics         []string `json:"skillable_topics"`
	NonSkillableContent     []string `json:"non_skillable_content"`
	EstimatedSkillCountLow  int      `json:"estimated_skill_count_low"`
	EstimatedSkillCountHigh int      `json:"estimated_skill_count_high"`
	PriorityRanking         []string `json:"priority_ranking"`
}

// ParseOverviewHeader extracts the title and author from a rendered
// BOOK_OVERVIEW.md. It is the inverse of render.BookOverview's header: the title
// is the "# <Title> — Book Overview" heading with the suffix removed, and the
// author is the "- **Author:** <author>" bullet. Missing fields return "".
// Matching is case-insensitive so a markdown formatter's heading title-casing
// (e.g. "Book overview" → "Book Overview") does not break it.
func ParseOverviewHeader(md string) (title, author string) {
	const (
		titleSuffix  = " — Book Overview"
		authorPrefix = "- **Author:** "
	)
	fenced := fencedLines(md)
	for i, line := range strings.Split(md, "\n") {
		if fenced[i] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case title == "" && strings.HasPrefix(trimmed, "# "):
			title = trimSuffixFold(strings.TrimSpace(trimmed[len("# "):]), titleSuffix)
		case author == "" && hasPrefixFold(trimmed, authorPrefix):
			author = strings.TrimSpace(trimmed[len(authorPrefix):])
		}
	}
	return title, author
}

// trimSuffixFold removes suffix from s if present, comparing case-insensitively.
func trimSuffixFold(s, suffix string) string {
	if len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix) {
		return s[:len(s)-len(suffix)]
	}
	return s
}

// hasPrefixFold reports whether s begins with prefix, comparing case-insensitively.
func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// ParseBookOverview recovers the Stage-0 quality-gate fields from a rendered
// BOOK_OVERVIEW.md: the one-sentence summary, the skeleton, the key terms, and
// the three critique lists (plus title/author). It is the inverse of
// render.BookOverview for those sections — the schema that makes gating a
// hand-authored overview deterministic. Sections the gate does not read
// (genre, propositions, skillable topics) are left zero.
func ParseBookOverview(md string) BookOverview {
	title, author := ParseOverviewHeader(md)
	var o BookOverview
	o.Title, o.Author = title, author
	if body, ok := sectionBody(md, "One-sentence summary"); ok {
		o.Structure.OneSentenceSummary = strings.TrimSpace(body)
	}
	o.Structure.Skeleton = sectionList(md, "Skeleton")
	o.Interpretation.KeyTerms = parseKeyTerms(md)
	o.Critique.EraLimitations = sectionList(md, "Era limitations")
	o.Critique.AuthorBlindSpots = sectionList(md, "Author blind spots")
	o.Critique.UnprovenAssumptions = sectionList(md, "Unproven assumptions")
	return o
}

// sectionList returns the bullet items under the "## <heading>" section, or nil.
func sectionList(md, heading string) []string {
	if body, ok := sectionBody(md, heading); ok {
		return listItems(body)
	}
	return nil
}

// parseKeyTerms recovers the key terms from the "## Key terms" section. Each
// rendered bullet is "- **<Term>:** <definition> (differs from common usage:
// <diff>)"; only Term is recovered (the gate counts entries).
func parseKeyTerms(md string) []KeyTerm {
	var terms []KeyTerm
	for _, item := range sectionList(md, "Key terms") {
		term := item
		if rest, ok := strings.CutPrefix(item, "**"); ok {
			if name, _, found := strings.Cut(rest, ":**"); found {
				term = strings.TrimSpace(name)
			}
		}
		terms = append(terms, KeyTerm{Term: term})
	}
	return terms
}

// QualityGate returns the reasons o fails the Stage-0 quality gate; an empty
// slice means the overview passes and Stage 1 may begin.
func (o *BookOverview) QualityGate() []string {
	var problems []string
	if o.Structure.OneSentenceSummary == "" {
		problems = append(problems, "one-sentence summary is empty")
	}
	if n := len(o.Structure.Skeleton); n < minSkeleton || n > maxSkeleton {
		problems = append(problems, "skeleton must list 3 to 7 primary arguments")
	}
	if len(o.Interpretation.KeyTerms) < minKeyTerms {
		problems = append(problems, "key-term glossary needs at least 5 entries")
	}
	critique := len(o.Critique.EraLimitations) +
		len(o.Critique.AuthorBlindSpots) +
		len(o.Critique.UnprovenAssumptions)
	if critique < minCritique {
		problems = append(problems, "critique must list at least 3 author limitations")
	}
	return problems
}
