package book2skill

import "strings"

// MinSharpSignals is the fewest language signals a merged skill's A2 must have
// that neither source skill has, to satisfy the structural half of merge-skills
// Red Line #3 (the A2 sharpness gate).
const MinSharpSignals = 2

// LanguageSignals returns the signals listed under the "### Language Signals"
// subsection of a SKILL.md body's A2 segment, with any surrounding quotes
// stripped. The subsection heading is matched case-insensitively (so a markdown
// formatter's title-casing does not break it). Returns nil when absent.
func LanguageSignals(body string) []string {
	sub, ok := subsection(ParseSegments(body)[SegA2], "Language Signals")
	if !ok {
		return nil
	}
	var signals []string
	for _, item := range listItems(sub) {
		if s := strings.TrimSpace(strings.Trim(item, "\"'`")); s != "" {
			signals = append(signals, s)
		}
	}
	return signals
}

// A2Sharpness returns the merged skill's language signals that appear in none of
// the source skills' signals (whitespace- and case-normalized). A result of at
// least MinSharpSignals satisfies the structural sharpness gate; whether those
// signals are genuinely, semantically distinct remains the agent's judgment.
func A2Sharpness(mergedBody string, sourceBodies []string) []string {
	inSource := make(map[string]bool)
	for _, sb := range sourceBodies {
		for _, s := range LanguageSignals(sb) {
			inSource[normalizeSignal(s)] = true
		}
	}
	var unique []string
	seen := make(map[string]bool)
	for _, s := range LanguageSignals(mergedBody) {
		key := normalizeSignal(s)
		if !inSource[key] && !seen[key] {
			unique = append(unique, s)
			seen[key] = true
		}
	}
	return unique
}

// subsection returns the body beneath a "### <heading>" line within a segment,
// up to the next "###"/"##" heading or end. Matching is case-insensitive.
func subsection(body, heading string) (string, bool) {
	want := strings.ToLower("### " + heading)
	var out []string
	in := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "### "):
			if in {
				return strings.Join(out, "\n"), true
			}
			lower := strings.ToLower(trimmed)
			in = lower == want || strings.HasPrefix(lower, want+" ")
		case strings.HasPrefix(trimmed, headingPrefix):
			if in {
				return strings.Join(out, "\n"), true
			}
		case in:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n"), in
}

func normalizeSignal(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
