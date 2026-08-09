package lint

import (
	"fmt"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/markdown"
	"github.com/StevenACoffman/skillet/skill"
	"github.com/StevenACoffman/skillet/skilllens"
)

// softeningLimit is the number of hedging phrases at which a skill's instructions
// read as unfollowable. It matches skillsaw's rubric dim-5 threshold so the lint tier
// and the score cannot disagree about the same file.
const softeningLimit = 3

// boundarySeverity is the severity for a missing boundary / counter-example section.
// It is a warning, not an error. Measuring the three detectors across the 277-skill
// corpus first (TODO §971) settled this: only 6% lack a boundary section (5% lack
// failure encoding, 0% carry >=3 softening phrases), and every one of those coincided
// with an already-broken skill. A gate that fires on 6% could be an error, but this
// coarse "has any boundary section" test is not the finer "the B segment does its job"
// structural check the error tier would need, so it surfaces the gap as a warning and
// leaves the stricter check to future work.
const boundarySeverity = finding.SeverityWarning

// skillLens returns the SkillLens quality diagnostics for s: the three externally
// validated dimensions from microsoft/SkillLens (arXiv:2605.23899) — does the skill
// encode failure mechanisms, are its instructions specific rather than softened, and
// does it draw a boundary around what not to do — located by skillet/skilllens.
//
// It is a third lint tier beside speclint (the spec) and redlines (book2skill's red
// lines), on its own schedule, so it stays opt-in and separate rather than folded into
// either. The detectors are pure; this parses s.Body once and decides severities.
func skillLens(s *skill.Skill) []finding.Diagnostic {
	doc := markdown.Parse(s.Body)
	var ds []finding.Diagnostic
	if len(skilllens.FailureMechanisms(doc)) == 0 {
		ds = append(ds, lensDiag(finding.SeverityWarning, "skilllens-failure",
			"no failure-mechanism encoding: no inline failure branch or failure-mode section"))
	}
	if n := len(skilllens.SofteningPhrases(doc)); n >= softeningLimit {
		ds = append(ds, lensDiag(finding.SeverityWarning, "skilllens-softening",
			fmt.Sprintf("%d softening phrases hedge the instructions (>= %d)", n, softeningLimit)))
	}
	if len(skilllens.BlacklistSections(doc)) == 0 {
		ds = append(ds, lensDiag(boundarySeverity, "skilllens-boundary",
			"no boundary / counter-example section (what not to do)"))
	}
	return ds
}

// lensDiag builds a categorized SkillLens diagnostic. Unlike diag, it sets Category
// and a per-finding Severity: the three dimensions do not share one severity.
func lensDiag(sev finding.Severity, category, message string) finding.Diagnostic {
	return finding.Diagnostic{Severity: sev, Category: category, Message: message}
}
