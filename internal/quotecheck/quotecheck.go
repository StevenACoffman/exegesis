// Package quotecheck adapts skillet's fabrication guard to a skill document.
//
// The comparison itself — split a quotation into passages, fold both sides, ask whether the
// source contains them — moved to skillet/quotecheck when gnosis became its second consumer.
// What stays here is the part that is exegesis's alone: where a quotation *is*. A skill
// quotes its sources in its R segment, and quotations are the blockquote runs skillet's
// redlines.Quotes finds, so this guard and the quotation-length red line agree on what a
// quotation is. Neither of those conventions means anything to the other consumer, and a
// shared kernel carrying them would be a general mechanism with one caller's document
// format baked in.
//
// Everything here is pure. The command reads the files.
package quotecheck

import (
	"strings"
	"unicode"

	"github.com/StevenACoffman/skillet/quotecheck"
	"github.com/StevenACoffman/skillet/redlines"
)

// MinPassageWords is the shortest passage worth a verdict. Re-exported so callers and tests
// name one constant rather than two that could drift apart.
const MinPassageWords = quotecheck.MinPassageWords

// Status values, re-exported for the same reason.
const (
	Unchecked = quotecheck.Unchecked
	Found     = quotecheck.Found
	Missing   = quotecheck.Missing
)

// Source is one plain-text source a quotation may have come from.
//
// This and the two below are aliases rather than wrappers: a wrapper would be a second type
// meaning the same thing, and every call site would convert between them for no gain.
type Source = quotecheck.Source

// Finding is one checked passage and where it was found.
type Finding = quotecheck.Finding

// Status is whether a passage was located.
type Status = quotecheck.Status

// Passages and Support are deliberately not re-exported. Re-exporting a function needs
// either a package-level var, which is prohibited, or a wrapper whose whole body forwards
// the same signature, which is the pass-through the guidelines name as a red flag. A caller
// wanting them imports skillet/quotecheck, which is where they live.

// Check reports, for each passage of each quotation in the named segment of a skill body,
// whether any source contains it.
//
// Only the named segment is examined: a skill quotes its sources in R, and an illustrative
// blockquote elsewhere is not a claim about a book.
//
// Requires: nothing; an absent segment and a segment with no blockquotes are both valid.
// Ensures: one Finding per checked passage, in document order; a quotation too short to
// split is reported Unchecked rather than dropped, so a caller counting findings cannot
// mistake "not looked at" for "clean". It is pure.
func Check(body, segment string, sources []Source) []Finding {
	return quotecheck.Check(redlines.Quotes(Segment(body, segment)), sources)
}

// Segment returns the body text under the "## " heading whose label is want, up to the next
// such heading, or "" when the segment is absent.
//
// A heading's label is its leading run of letters and digits, upper-cased — the same rule
// skillet's redlines uses to decide which RIA segments a body declares. It is restated here
// rather than guessed at, because a guard that disagreed with the red lines about where R
// begins would check the wrong text. Headings that yield no label, which is every "###" and
// deeper, are content rather than boundaries, so a subsection does not end the segment.
//
// Ensures: it is pure.
func Segment(body, want string) string {
	var out []string
	inSegment := false
	for _, line := range strings.Split(body, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "## "); ok {
			if label := leadingAlnum(strings.TrimSpace(rest)); label != "" {
				inSegment = strings.EqualFold(label, want)
				continue
			}
		}
		if inSegment {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// leadingAlnum returns the leading run of letters and digits in s.
func leadingAlnum(s string) string {
	for i, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return s[:i]
		}
	}
	return s
}
