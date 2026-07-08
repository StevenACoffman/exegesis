package book2skill

// Phase-1.5 source-verification checks.
const (
	// CheckRQuoteAccuracy verifies each R-segment quote against the source text.
	CheckRQuoteAccuracy VerificationCheck = "r-quote-accuracy"
	// CheckA1Attribution verifies each A1 case against the source text.
	CheckA1Attribution VerificationCheck = "a1-attribution"
)

// Source-verification verdicts. The r-quote-accuracy check uses accurate/drifted/
// not-found; the a1-attribution check uses verified/mismatch/not-found. The union
// is one vocabulary so a single Valid covers both.
const (
	// StatusAccurate: the quote is verbatim or within paraphrase distance.
	StatusAccurate VerificationStatus = "accurate"
	// StatusDriftedMinor: minor drift, corrected in place.
	StatusDriftedMinor VerificationStatus = "drifted-minor"
	// StatusDriftedMajor: material drift.
	StatusDriftedMajor VerificationStatus = "drifted-major"
	// StatusNotFound: the quote or case could not be located in the source.
	StatusNotFound VerificationStatus = "not-found"
	// StatusVerified: the A1 case's chain matches the source.
	StatusVerified VerificationStatus = "verified"
	// StatusMismatch: the A1 case's chain does not match the source.
	StatusMismatch VerificationStatus = "mismatch"
)

// VerificationCheck names which Phase-1.5 check an artifact records.
type VerificationCheck string

// VerificationStatus is one source's verdict in a verification artifact.
type VerificationStatus string

// VerificationSource is one source skill's verdict within a verification artifact.
type VerificationSource struct {
	Book      string             `yaml:"book"`
	Skill     string             `yaml:"skill"`
	Status    VerificationStatus `yaml:"status"`
	Corrected bool               `yaml:"corrected,omitempty"`
}

// SourceVerification is the structured header of a source-verification artifact
// (`<pair-id>-r.md` or `<pair-id>-a1.md`). The struct tags are inert here;
// marshaling lives in the internal/mergedoc adapter.
type SourceVerification struct {
	Pair    string               `yaml:"pair"`
	Check   VerificationCheck    `yaml:"check"`
	Sources []VerificationSource `yaml:"sources"`
}

// VerificationRow is the render-ready summary for one pair: its R and A1 source
// verdicts and the V1–V4 outcome (assembled by internal/mergetree).
type VerificationRow struct {
	Pair        string
	R           []VerificationSource
	A1          []VerificationSource
	Validations string
}

// Valid reports whether s is a known verification status.
func (s VerificationStatus) Valid() bool {
	switch s {
	case StatusAccurate, StatusDriftedMinor, StatusDriftedMajor,
		StatusNotFound, StatusVerified, StatusMismatch:
		return true
	default:
		return false
	}
}

// Valid reports whether c is a known verification check.
func (c VerificationCheck) Valid() bool {
	return c == CheckRQuoteAccuracy || c == CheckA1Attribution
}

// Validate returns the reasons v is not a well-formed verification header; an
// empty slice means it is valid.
func (v *SourceVerification) Validate() []string {
	var problems []string
	if v.Pair == "" {
		problems = append(problems, "pair is required")
	}
	if !v.Check.Valid() {
		problems = append(problems, "unknown check "+string(v.Check))
	}
	if len(v.Sources) == 0 {
		problems = append(problems, "at least one source is required")
	}
	for i := range v.Sources {
		problems = append(problems, validateSource(&v.Sources[i])...)
	}
	return problems
}

func validateSource(s *VerificationSource) []string {
	var problems []string
	if s.Book == "" || s.Skill == "" {
		problems = append(problems, "each source needs book and skill")
	}
	if !s.Status.Valid() {
		problems = append(problems, "unknown status "+string(s.Status))
	}
	return problems
}
