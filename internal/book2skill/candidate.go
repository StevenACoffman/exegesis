package book2skill

// CandidateType values, one per Phase-1 extractor.
const (
	// TypeFramework is a mental model, decision framework, or reasoning method.
	TypeFramework CandidateType = "framework"
	// TypePrinciple is a principle, checklist, rule, or maxim.
	TypePrinciple CandidateType = "principle"
	// TypeCase is an example the author personally applied.
	TypeCase CandidateType = "case"
	// TypeCounterExample is a failure mode, trap, or bias the author warns of.
	TypeCounterExample CandidateType = "counter-example"
	// TypeTerm is a key concept in the author's specific usage.
	TypeTerm CandidateType = "term"
)

// CandidateType classifies a candidate methodology unit by extractor origin.
type CandidateType string

// Evidence locates one piece of within-book support for a unit.
type Evidence struct {
	Location string `json:"location"`
	Summary  string `json:"summary"`
}

// CandidateUnit is a raw methodology fragment produced by a Phase-1 extractor,
// before any screening. Type-specific fields are populated only for the
// matching Type and are otherwise empty.
type CandidateUnit struct {
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Type          CandidateType `json:"type"`
	SourceChapter string        `json:"source_chapter"`
	SourceQuote   string        `json:"source_quote"`
	Summary       string        `json:"summary"`
	Tags          []string      `json:"tags"`

	// BoundTo lists the methodology topics (case units) or the positive skills a
	// counter-example unit constrains. Outcome records a case's result.
	BoundTo []string `json:"bound_to,omitempty"`
	Outcome string   `json:"outcome,omitempty"`

	// Counter-example units describe a failure, its mechanism, and its warning
	// signs.
	FailureMode  string   `json:"failure_mode,omitempty"`
	Mechanism    string   `json:"mechanism,omitempty"`
	WarningSigns []string `json:"warning_signs,omitempty"`

	// Term units define a word in the author's specific usage.
	AuthorDefinition string `json:"author_definition,omitempty"`
	KeyDistinction   string `json:"key_distinction,omitempty"`
	WhyItMatters     string `json:"why_it_matters,omitempty"`
}

// CrossDomain is the Phase-1.5 V1 verdict: evidence in at least two independent
// within-book contexts.
type CrossDomain struct {
	Passed   bool       `json:"passed"`
	Evidence []Evidence `json:"evidence"`
}

// Predictive is the Phase-1.5 V2 verdict: the unit answers a question the book
// does not explicitly address.
type Predictive struct {
	Passed        bool   `json:"passed"`
	NovelQuestion string `json:"novel_question"`
	DerivedAnswer string `json:"derived_answer"`
}

// Exclusivity is the Phase-1.5 V3 verdict: the unit is the author's distinctive
// insight, not common sense.
type Exclusivity struct {
	Passed       bool   `json:"passed"`
	WhyNotCommon string `json:"why_not_common"`
}

// TripleValidation records all three Phase-1.5 verdicts for a candidate.
type TripleValidation struct {
	V1 CrossDomain `json:"v1_cross_domain"`
	V2 Predictive  `json:"v2_predictive_power"`
	V3 Exclusivity `json:"v3_exclusivity"`
}

// VerifiedUnit is a candidate that has been through Phase-1.5 validation.
type VerifiedUnit struct {
	Candidate  CandidateUnit    `json:"candidate"`
	Validation TripleValidation `json:"validation"`
}

// ParseCandidateType returns the CandidateType for s, or EINVALID if s is not a
// recognized type.
func ParseCandidateType(s string) (CandidateType, error) {
	t := CandidateType(s)
	if !t.Valid() {
		return "", &Error{Code: EINVALID, Message: "unknown candidate type: " + s}
	}
	return t, nil
}

// Valid reports whether t is one of the known candidate types.
func (t CandidateType) Valid() bool {
	switch t {
	case TypeFramework, TypePrinciple, TypeCase, TypeCounterExample, TypeTerm:
		return true
	default:
		return false
	}
}

// Passed reports whether all three validations passed, the gate for a candidate
// to become a skill.
func (v *TripleValidation) Passed() bool {
	return v.V1.Passed && v.V2.Passed && v.V3.Passed
}
