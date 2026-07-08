package book2skill

import "encoding/json"

// Phase-4 stress-test case categories.
const (
	// ShouldTrigger marks a prompt the skill must fire on.
	ShouldTrigger TestType = "should_trigger"
	// ShouldNotTrigger marks a decoy the skill must stay silent on.
	ShouldNotTrigger TestType = "should_not_trigger"
	// EdgeCase marks a boundary prompt whose judgement may reasonably go
	// either way.
	EdgeCase TestType = "edge_case"
	// PreferMergedOverSource marks a merge-skills prompt where the merged skill
	// must outperform either source skill alone (the additive gate).
	PreferMergedOverSource TestType = "prefer_merged_over_source"
)

// Structural Phase-4 gate thresholds (decision D4): the minimum composition a
// darwin-bound test set must have. These are the single source of truth; the
// pipeline consumes them via ValidateTestSet.
const (
	// MinShouldTrigger is the fewest positive cases a test set must contain.
	MinShouldTrigger = 3
	// MinShouldNotTrigger is the fewest decoy cases a test set must contain.
	MinShouldNotTrigger = 2
	// MinEdgeCase is the fewest boundary cases a test set must contain.
	MinEdgeCase = 1

	// MinMergedEdgeCase is the fewest boundary cases a merge-skills test set must
	// contain (merged skills need boundaries against each source).
	MinMergedEdgeCase = 2
	// MinPreferMerged is the fewest prefer_merged_over_source cases a merge-skills
	// test set must contain: the "identify ≥2 scenarios or auto-dissolve" rule.
	MinPreferMerged = 2
)

// TestType classifies a Phase-4 stress-test case.
type TestType string

// TestCase is one Phase-4 stress-test case.
//
// Its JSON form is a superset of the darwin-skill test-prompts element
// {id, prompt, expected}: darwin reads those three keys and ignores type and
// notes. A slice of TestCase therefore marshals to the bare JSON array
// darwin-skill consumes as test-prompts.json, while retaining book2skill's own
// category metadata for the Phase-4 harness.
type TestCase struct {
	ID       int      `json:"id"`
	Type     TestType `json:"type"`
	Prompt   string   `json:"prompt"`
	Expected string   `json:"expected"`
	Notes    string   `json:"notes,omitempty"`
}

// EncodeTestPrompts renders cases as the indented, bare JSON array that
// darwin-skill consumes as test-prompts.json.
func EncodeTestPrompts(cases []TestCase) ([]byte, error) {
	b, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		return nil, &Error{Op: "book2skill.EncodeTestPrompts", Err: err}
	}
	return b, nil
}

// DecodeTestPrompts parses the darwin-shaped bare JSON array that
// EncodeTestPrompts produces. It is the inverse of EncodeTestPrompts.
func DecodeTestPrompts(data []byte) ([]TestCase, error) {
	var cases []TestCase
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, &Error{Code: EINVALID, Message: "test-prompts.json is not a JSON array"}
	}
	return cases, nil
}

// CountByType tallies cases by their TestType. It is pure and allocates a fresh
// map each call.
func CountByType(cases []TestCase) map[TestType]int {
	counts := make(map[TestType]int, 3)
	for i := range cases {
		counts[cases[i].Type]++
	}
	return counts
}

// ValidateTestSet returns the reasons cases fail the structural Phase-4 gate; an
// empty slice means the set passes and darwin scoring may proceed. It reports
// every failing threshold at once. Runtime trigger scoring is delegated to
// darwin-skill, which consumes the emitted test-prompts.json.
func ValidateTestSet(cases []TestCase) []string {
	counts := CountByType(cases)
	var problems []string
	if counts[ShouldTrigger] < MinShouldTrigger {
		problems = append(problems, "needs at least 3 should_trigger cases")
	}
	if counts[ShouldNotTrigger] < MinShouldNotTrigger {
		problems = append(problems, "needs at least 2 should_not_trigger decoys")
	}
	if counts[EdgeCase] < MinEdgeCase {
		problems = append(problems, "needs at least 1 edge_case")
	}
	return problems
}

// ValidateMergedTestSet returns the reasons cases fail the merge-skills Phase-4
// gate; an empty slice means the set passes. It requires the three book2skill
// categories plus prefer_merged_over_source, with merge-specific minimums.
// Runtime pass-rate (including "≥1 prefer_merged must pass") is delegated to
// darwin-skill.
func ValidateMergedTestSet(cases []TestCase) []string {
	counts := CountByType(cases)
	var problems []string
	if counts[ShouldTrigger] < MinShouldTrigger {
		problems = append(problems, "needs at least 3 should_trigger cases")
	}
	if counts[ShouldNotTrigger] < MinShouldNotTrigger {
		problems = append(problems, "needs at least 2 should_not_trigger decoys")
	}
	if counts[EdgeCase] < MinMergedEdgeCase {
		problems = append(problems, "needs at least 2 edge_case cases (one per source boundary)")
	}
	if counts[PreferMergedOverSource] < MinPreferMerged {
		problems = append(problems,
			"needs at least 2 prefer_merged_over_source cases, else auto-dissolve the merge")
	}
	return problems
}

// TemplateMergedTestCases returns a minimal, gate-passing merge scaffold covering
// all four categories at their merge minimums, with sequential ids.
func TemplateMergedTestCases() []TestCase {
	total := MinShouldTrigger + MinShouldNotTrigger + MinMergedEdgeCase + MinPreferMerged
	cases := make([]TestCase, 0, total)
	id := 1
	add := func(t TestType, n int, prompt, expected string) {
		for range n {
			cases = append(cases, TestCase{
				ID: id, Type: t, Prompt: prompt, Expected: expected,
				Notes: "TODO: replace with a real case",
			})
			id++
		}
	}
	add(ShouldTrigger, MinShouldTrigger, "TODO: a core scenario for the merged skill",
		"merged skill invoked")
	add(ShouldNotTrigger, MinShouldNotTrigger, "TODO: a decoy neither source applies to",
		"no methodology skill invoked")
	add(EdgeCase, MinMergedEdgeCase, "TODO: a boundary between merged and a source skill",
		"merged or that source, with a reason")
	add(PreferMergedOverSource, MinPreferMerged,
		"TODO: a scenario where the merged skill beats either source alone",
		"merged produces what neither source alone would")
	return cases
}

// TemplateTestCases returns a minimal, gate-passing scaffold: placeholder cases
// covering MinShouldTrigger positives, MinShouldNotTrigger decoys, and
// MinEdgeCase boundary case, with sequential ids. Authors edit the prompts.
func TemplateTestCases() []TestCase {
	cases := make([]TestCase, 0, MinShouldTrigger+MinShouldNotTrigger+MinEdgeCase)
	id := 1
	add := func(t TestType, n int, prompt, expected string) {
		for range n {
			cases = append(cases, TestCase{
				ID: id, Type: t, Prompt: prompt, Expected: expected,
				Notes: "TODO: replace with a real case",
			})
			id++
		}
	}
	add(ShouldTrigger, MinShouldTrigger, "TODO: a prompt the skill must fire on", "skill invoked")
	add(
		ShouldNotTrigger,
		MinShouldNotTrigger,
		"TODO: a decoy the skill must ignore",
		"skill not invoked",
	)
	add(EdgeCase, MinEdgeCase, "TODO: a boundary prompt", "either outcome is defensible")
	return cases
}

// Valid reports whether t is one of the known test types.
func (t TestType) Valid() bool {
	switch t {
	case ShouldTrigger, ShouldNotTrigger, EdgeCase, PreferMergedOverSource:
		return true
	default:
		return false
	}
}
