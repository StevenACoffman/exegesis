// Package quotecheck is the fabrication guard: it reports which of a skill's
// quotations cannot be found in any of the source texts they claim to come from.
//
// It answers only the mechanical question — does this run of words appear in the source
// at all. Judging whether a paraphrase is faithful, or whether a quotation is used in
// context, stays with the reader; a tool that guessed at those would produce verdicts
// nobody could check. What it does catch is the failure that matters most and is hardest
// to spot by eye: a quotation that was never in the book.
//
// Everything here is pure. The command reads the files.
package quotecheck

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/StevenACoffman/skillet/redlines"
)

// MinPassageWords is the shortest passage worth a verdict.
//
// A three-word fragment is weak evidence in both directions: it appears in almost any
// book by chance, so finding it proves nothing, and failing to find it is as likely to
// be a split artifact as a fabrication. Reporting those would bury the findings that
// mean something.
const MinPassageWords = 6

// reSpace matches any run of whitespace, including the newlines a quotation is wrapped
// across in Markdown but not in an extracted source text.
var reSpace = regexp.MustCompile(`\s+`)

// reSentence matches the sentence terminators a quotation is split on.
var reSentence = regexp.MustCompile(`[.!?]+`)

// Source is one plain-text source a quotation may have come from.
type Source struct {
	Name string // how to name it in a report; usually the file path
	Text string
}

// Finding is one checked passage and where it was found.
type Finding struct {
	Passage string
	FoundIn string // name of the first source containing it; empty means found in none
}

// Missing reports whether the quotation was found in no source at all.
func (f Finding) Missing() bool { return f.FoundIn == "" }

// Check reports, for each passage of each quotation in the named segment of a skill
// body, whether any source contains it.
//
// Quotations are the blockquote runs skillet's redlines.Quotes finds, so this guard and
// the quotation-length red line agree on what a quotation is. Only the named segment is
// examined: a skill quotes its sources in R, and an illustrative blockquote elsewhere is
// not a claim about a book.
//
// Matching is per passage rather than per quotation because in practice a skill's whole
// R segment is one blockquote — 95% of this corpus, median 860 characters. Requiring all
// of that to appear verbatim and contiguously means a single editorial difference
// anywhere condemns the entire segment and says nothing about where the problem is.
// Per passage, a fabricated sentence inserted into an otherwise faithful quotation is
// caught and named, which is the subtle case that matters most.
//
// Both sides are normalized before matching — whitespace runs collapsed, typographic
// characters folded to ASCII — because a quotation is line-wrapped in Markdown and its
// source is not, so a literal comparison would report everything missing. That
// normalization is the whole of the mechanical latitude given.
//
// Ensures: one Finding per checked passage, in document order; it is pure.
func Check(body, segment string, sources []Source) []Finding {
	quotes := redlines.Quotes(Segment(body, segment))
	if len(quotes) == 0 {
		return nil
	}
	// Normalize each source once. A source is a whole book; doing it per passage would
	// multiply that cost by the number of passages for no benefit.
	haystacks := make([]string, len(sources))
	for i, s := range sources {
		haystacks[i] = normalize(s.Text)
	}
	var findings []Finding
	for _, q := range quotes {
		for _, p := range Passages(q) {
			f := Finding{Passage: p}
			needle := normalize(p)
			for i, h := range haystacks {
				if strings.Contains(h, needle) {
					f.FoundIn = sources[i].Name
					break
				}
			}
			findings = append(findings, f)
		}
	}
	return findings
}

// Passages splits a quotation into the chunks matched individually: sentences, dropped
// to those of at least MinPassageWords.
//
// Terminators are discarded along with the split, and abbreviations and decimals
// therefore over-split ("e.g." becomes two fragments). Both are deliberate and safe:
// over-splitting leaves the substantive text still being checked, and the resulting
// fragments are too short to survive the MinPassageWords filter.
//
// Ensures: every returned passage has at least MinPassageWords words; it is pure.
func Passages(quote string) []string {
	var out []string
	for _, chunk := range reSentence.Split(quote, -1) {
		if chunk = strings.TrimSpace(chunk); len(strings.Fields(chunk)) >= MinPassageWords {
			out = append(out, chunk)
		}
	}
	return out
}

// Segment returns the body text under the "## " heading whose label is want, up to the
// next such heading, or "" when the segment is absent.
//
// A heading's label is its leading run of letters and digits, upper-cased — the same
// rule skillet's redlines uses to decide which RIA segments a body declares. It is
// restated here rather than guessed at, because a guard that disagreed with the red
// lines about where R begins would check the wrong text. Headings that yield no label,
// which is every "###" and deeper, are content rather than boundaries, so a subsection
// does not end the segment.
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

// typographic folds the characters a book and its plain-text extraction are most likely
// to disagree about. A guard that fired on every curly apostrophe would not get run.
//
// The two space-like entries are written as escapes on purpose: a literal non-breaking
// or zero-width space in source is invisible, so a later reader could neither tell what
// the entry does nor notice if it were silently edited away.
//
// Built per call rather than kept in a package variable: it is microseconds against
// reading whole books off disk, and a package-level Replacer is shared state.
func typographic() *strings.Replacer {
	return strings.NewReplacer(
		"\u2018", "'", "\u2019", "'", // curly single quotes
		"\u201c", `"`, "\u201d", `"`, // curly double quotes
		"\u2013", "-", "\u2014", "-", // en and em dash
		"\u2026", "...", // ellipsis
		"\u00a0", " ", // non-breaking space
		"\u200b", "", // zero-width space
	)
}

// normalize reduces text to the form both sides are compared in.
func normalize(s string) string {
	return strings.TrimSpace(reSpace.ReplaceAllString(typographic().Replace(s), " "))
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
