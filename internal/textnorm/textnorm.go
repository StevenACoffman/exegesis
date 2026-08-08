// Package textnorm folds the differences between two copies of the same words that
// nobody means as differences: a run of whitespace where a line happened to wrap, and
// the typographic characters a book, a plain-text extraction and a Markdown file each
// spell their own way.
//
// It exists because two guards need the same folding and must not disagree about it.
// `quotecheck` compares a quotation against the source it claims to come from, and
// `a2check` compares a merged skill's language signals against its sources'; a
// quotation that matched in one and not the other would be a defect nobody could
// explain from the output.
//
// Everything here is pure.
package textnorm

import (
	"regexp"
	"strings"
)

// reSpace matches any run of whitespace, including the newlines a quotation is wrapped
// across in Markdown but not in an extracted source text.
var reSpace = regexp.MustCompile(`\s+`)

// Fold reduces text to the form two copies of it are compared in: whitespace runs
// collapsed to one space, typographic characters folded to ASCII, ends trimmed.
//
// It deliberately does not change case. Case can carry meaning in a quotation, and the
// caller that wants case-insensitivity can say so; a folder that quietly lower-cased
// would leave no way back.
//
// Ensures: Fold(Fold(s)) == Fold(s); it is pure.
func Fold(s string) string {
	return strings.TrimSpace(reSpace.ReplaceAllString(typographic().Replace(s), " "))
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
