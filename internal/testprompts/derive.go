package testprompts

import (
	"regexp"
	"strings"
)

// Derivation patterns. Each is intentionally conservative: a check is emitted
// only when the cue is unambiguous, so a derived set never fails judge on a
// guess. Ambiguous Expected text yields no checks (the caller must then hand-write
// them) rather than a wrong one.
var (
	// `"Result" section` / `"Boundary" section` -> section_present(Result).
	reSectionQuoted = regexp.MustCompile(`"([^"]+)"\s+section`)
	// A literal markdown heading inside Expected -> section_present(<heading>).
	reHeading = regexp.MustCompile(`(?m)^#{1,6}\s+(.+?)\s*$`)
	// `"foo" tool` / `` `foo` tool `` -> tool_called(foo).
	reToolQuoted = regexp.MustCompile("[\"`]([^\"`]+)[\"`]\\s+tool")
	// `contains/includes/mentions "phrase"` -> contains(phrase).
	reContains = regexp.MustCompile(`(?i)(?:contains|includes|mentions|outputs)\s+"([^"]+)"`)
	// `<= N chars`, `under N characters`, `at most N characters` -> max_chars(N).
	reMaxChars = regexp.MustCompile(`(?i)(?:<=|≤|under|at most|no more than)\s+(\d+)\s+char`)
	// `>= N chars`, `at least N characters` -> min_chars(N).
	reMinChars = regexp.MustCompile(`(?i)(?:>=|≥|at least|no fewer than)\s+(\d+)\s+char`)
)

// DeriveChecks converts an Expected description into deterministic judge checks.
//
// Requires: expected is the human description of a good output.
// Ensures:  every returned Check is backed by an unambiguous cue in expected;
//
//	returns nil (not a wrong guess) when nothing is inferable; the result
//	is de-duplicated and its ordering is stable for identical input.
func DeriveChecks(expected string) []Check {
	var checks []Check
	seen := map[Check]bool{}
	add := func(op, arg string) {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			return
		}
		c := Check{Op: op, Arg: arg}
		if !seen[c] {
			seen[c] = true
			checks = append(checks, c)
		}
	}
	// Fixed operator order (not text order) so output is stable.
	for _, m := range reSectionQuoted.FindAllStringSubmatch(expected, -1) {
		add("section_present", m[1])
	}
	for _, m := range reHeading.FindAllStringSubmatch(expected, -1) {
		add("section_present", m[1])
	}
	for _, m := range reToolQuoted.FindAllStringSubmatch(expected, -1) {
		add("tool_called", m[1])
	}
	for _, m := range reContains.FindAllStringSubmatch(expected, -1) {
		add("contains", m[1])
	}
	for _, m := range reMaxChars.FindAllStringSubmatch(expected, -1) {
		add("max_chars", m[1])
	}
	for _, m := range reMinChars.FindAllStringSubmatch(expected, -1) {
		add("min_chars", m[1])
	}
	return checks
}
