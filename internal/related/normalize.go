package related

import "strings"

// Normalize rewrites md's related-skills content into the canonical form: one
// `## Related Skills` heading, and every bullet that names a skill replaced by one
// canonical Bullet per target it named. It reports whether md changed.
//
// A file holding several related-skills sections is merged into the first, which is
// why this can move lines at all: 13 skills in the real books carry a second section,
// and leaving them apart means one skill declares its relationships in two places that
// disagree. A later section is merged only when every line of it is a bullet, a blank,
// or a thematic break — the shapes that can be moved or dropped without losing
// anything. A later section holding prose is left exactly where it is and normalized
// in place, because deleting a paragraph to tidy a heading is not a trade worth making.
//
// Apart from that merge it substitutes only the lines it understands. A bullet whose
// "target" is prose, an introductory sentence, a blank line, a thematic break, fenced
// code, and everything outside the sections are all left exactly as they were. That is
// deliberate: a rewrite that only ever replaces matched lines cannot lose unmatched
// content, so there is no separate preservation rule that could be got wrong.
// Regenerating a section from its parsed edges would have deleted the five prose
// bullets in the real books.
//
// Edges are deduplicated by (Kind, Target) across every section, first occurrence
// winning, matching ParseSection — so a relationship stated in both sections is
// written once rather than twice. Merged bullets keep document order; the canonical
// order is the reader's job (see ParseSection), and imposing it here would reorder
// bullets relative to the prose lines this refuses to move.
//
// Ensures: Normalize(Normalize(md)) == Normalize(md); ParseSection(out) == the same
//
//	edges as ParseSection(md); every line outside the sections is byte-identical;
//	it is pure.
func Normalize(md string) (string, bool) {
	lines := strings.Split(md, "\n")
	secs := findSections(lines)
	if len(secs) == 0 {
		return md, false
	}
	// The first section is rewritten first so that it, not a later one, decides the
	// rationale a relationship stated twice keeps.
	seen := make(map[edgeKey]bool)
	first := normalizeBullets(lines, secs[0], seen)
	var hoisted []string
	merged := make(map[int]bool, len(secs))
	for i := 1; i < len(secs); i++ {
		if !mergeable(lines, secs[i]) {
			continue
		}
		merged[i] = true
		for _, b := range sectionBullets(lines, secs[i].head, secs[i].end) {
			hoisted = append(hoisted, canonicalize(b, lines, seen)...)
		}
	}
	result := strings.Join(assemble(lines, secs, withBullets(first, hoisted), merged, seen), "\n")
	return result, result != md
}

// assemble rebuilds the file: every line outside a section copied through, each
// surviving section under the canonical heading, and each merged section's span
// dropped entirely — its bullets are already in first.
func assemble(
	lines []string,
	secs []section,
	first []string,
	merged map[int]bool,
	seen map[edgeKey]bool,
) []string {
	out := make([]string, 0, len(lines))
	at := 0
	for i, sec := range secs {
		out = append(out, lines[at:sec.head]...)
		at = sec.end
		if merged[i] {
			continue
		}
		out = append(out, sectionHeading)
		if i == 0 {
			out = append(out, first...)
			continue
		}
		out = append(out, normalizeBullets(lines, sec, seen)...)
	}
	return append(out, lines[at:]...)
}

// normalizeBullets returns the section body with each parseable bullet replaced by
// its canonical form. Lines that are not a parseable bullet are copied through
// untouched.
func normalizeBullets(lines []string, sec section, seen map[edgeKey]bool) []string {
	var body []string
	at := sec.head + 1
	for _, b := range sectionBullets(lines, sec.head, sec.end) {
		body = append(body, lines[at:b.start]...) // untouched lines before this bullet
		body = append(body, canonicalize(b, lines, seen)...)
		at = b.end
	}
	return append(body, lines[at:sec.end]...)
}

// canonicalize returns the replacement lines for one logical bullet: a canonical
// bullet per new target, or the bullet's original lines when it names no skill.
func canonicalize(b logicalBullet, lines []string, seen map[edgeKey]bool) []string {
	edges, understood := newEdges(b.text, seen)
	if !understood {
		return lines[b.start:b.end]
	}
	out := make([]string, 0, len(edges))
	for _, e := range edges {
		out = append(out, Bullet(e))
	}
	if len(out) == 0 {
		// Every target was a duplicate; drop the now-empty bullet rather than
		// leaving a legacy line that says the same thing as an earlier one.
		return nil
	}
	return out
}

// withBullets returns body with extra inserted after its last bullet, so hoisted
// bullets join the list rather than landing after the blank line and thematic break
// that close the section. A section with no bullets yet takes them after the blank
// line that follows its heading, which keeps the list separated from the heading.
func withBullets(body, extra []string) []string {
	if len(extra) == 0 {
		return body
	}
	at := 0
	for at < len(body) && strings.TrimSpace(body[at]) == "" {
		at++
	}
	for i, line := range body {
		if isBulletLine(strings.TrimSpace(line)) {
			at = i + 1
		}
	}
	out := make([]string, 0, len(body)+len(extra))
	out = append(out, body[:at]...)
	out = append(out, extra...)
	return append(out, body[at:]...)
}

// mergeable reports whether sec can be folded into an earlier section without losing
// content: every line of it other than the heading must be part of a bullet, blank, or
// a thematic break. The bullets move; the blanks and the break are dropped along with
// the heading, the break having separated a section that no longer exists.
//
// All 13 two-section skills in the real books qualify — heading, blank, bullets, blank,
// break — so the guard costs nothing there and refuses the case it exists for: a later
// section holding prose, a sub-heading, or fenced code.
func mergeable(lines []string, sec section) bool {
	inBullet := make(map[int]bool, sec.end-sec.head)
	for _, b := range sectionBullets(lines, sec.head, sec.end) {
		for i := b.start; i < b.end; i++ {
			inBullet[i] = true
		}
	}
	for i := sec.head + 1; i < sec.end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if inBullet[i] || trimmed == "" || isThematicBreak(trimmed) {
			continue
		}
		return false
	}
	return true
}

// isThematicBreak reports whether a trimmed line is a horizontal rule. Only the dash
// form is recognized, matching the one isBulletLine has to exclude.
func isThematicBreak(trimmed string) bool {
	return strings.HasPrefix(trimmed, "---")
}
