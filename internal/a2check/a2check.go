// Package a2check is the A2-sharpness gate: it reports which of a merged skill's
// language signals are new — stated by the merged skill and by none of the sources it
// was merged from.
//
// A merged skill earns its place by being *more* specific than either source, and A2 is
// where that specificity is written down: the phrases a user says that should invoke
// this skill rather than one of its parents. A merged A2 that only repeats its sources'
// signals describes no situation they did not already cover, and the corpus will hold
// three skills competing for the same triggers where it held two.
//
// The counting is structural and nothing here judges meaning. Two signals can be
// worded differently and mean the same thing, and this will call them both new;
// deciding that is the reader's, which is why the command is advisory unless asked to
// gate.
//
// Everything here is pure. The command reads the files.
package a2check

import (
	"regexp"
	"strings"

	"github.com/StevenACoffman/exegesis/internal/quotecheck"
	"github.com/StevenACoffman/exegesis/internal/textnorm"
)

// Segment is the RIA-TV++ segment holding the trigger scenarios and their signals.
const Segment = "A2"

// MinNew is how many new signals a merged skill needs to be sharper than its sources.
//
// Two, because one can be an accident of wording: a source's "we need five nines" and a
// merged "we need five-nines" are one signal that folding did not quite join. Requiring
// two makes a passing result mean the merged skill reaches situations its sources did
// not, rather than that it paraphrased one of them.
const MinNew = 2

// minSignalRunes is the shortest quoted run treated as a signal. Below it a quotation
// is a word being named — "the `merged` state" — not a phrase a user would say.
const minSignalRunes = 12

// reQuoted matches a double-quoted run, which is how every skill in this corpus writes
// a language signal, under a "### Language Signals" heading in some and a bold
// "**Language signals:**" label in others.
//
// Matching the quotation rather than the subsection is deliberate and measured: only 10
// of the 27 merged skills have either form of that heading, so a reader that required
// it would score two-thirds of the tree as having no signals at all.
var reQuoted = regexp.MustCompile(`"([^"\n]+)"`)

// Signals returns the language signals stated in body's A2 segment: the quoted phrases
// in it, folded for comparison and deduplicated, in document order.
//
// Ensures: no duplicates; every result is at least minSignalRunes long; it is pure.
func Signals(body string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, m := range reQuoted.FindAllStringSubmatch(quotecheck.Segment(body, Segment), -1) {
		signal := key(m[1])
		if len([]rune(signal)) < minSignalRunes || seen[signal] {
			continue
		}
		seen[signal] = true
		out = append(out, textnorm.Fold(m[1]))
	}
	return out
}

// New returns the signals in merged that no source states, in merged's order.
//
// Comparison is on the folded, lower-cased, punctuation-trimmed form, so a signal that
// two skills wrote with different quote characters or a trailing full stop is one
// signal rather than a spurious novelty. Anything subtler than that — a paraphrase, a
// synonym — reads as new here and is the reader's to judge.
//
// Ensures: the result is a subset of merged; it is pure.
func New(merged []string, sources [][]string) []string {
	known := make(map[string]bool)
	for _, source := range sources {
		for _, signal := range source {
			known[key(signal)] = true
		}
	}
	var out []string
	for _, signal := range merged {
		if !known[key(signal)] {
			out = append(out, signal)
		}
	}
	return out
}

// key reduces a signal to the form two skills' phrasings are compared in: folded,
// lower-cased, and stripped of the punctuation a sentence ends with.
func key(signal string) string {
	return strings.Trim(strings.ToLower(textnorm.Fold(signal)), " .!?,;:'\"")
}
