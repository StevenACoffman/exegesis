package related

import "strings"

// Normalize rewrites md's `## Related skills` section into the canonical form: the
// heading exactly `## Related skills`, and every bullet that names a skill replaced
// by one canonical Bullet per target it named. It reports whether md changed.
//
// It substitutes only the lines it understands. A bullet whose "target" is prose, an
// introductory sentence, a blank line, a thematic break, fenced code, and everything
// outside the section are all left exactly as they were. That is deliberate: a
// rewrite that only ever replaces matched lines cannot lose unmatched content, so
// there is no separate preservation rule that could be got wrong. Regenerating the
// section from its parsed edges would have deleted the five prose bullets in the real
// books.
//
// Edges are deduplicated by (Kind, Target), first occurrence winning, matching
// ParseSection — so a legacy bullet and a canonical bullet naming the same
// relationship collapse to one line rather than producing two identical bullets.
//
// Ensures: Normalize(Normalize(md)) == Normalize(md); ParseSection(out) contains the
//
//	same edges as ParseSection(md); every line outside the section is
//	byte-identical; it is pure.
func Normalize(md string) (string, bool) {
	lines := strings.Split(md, "\n")
	head, end, found := findSection(lines)
	if !found {
		return md, false
	}
	out := make([]string, 0, len(lines))
	out = append(out, lines[:head]...)
	out = append(out, sectionHeading)

	rewritten, next := normalizeBullets(lines, head, end)
	out = append(out, rewritten...)
	out = append(out, lines[next:]...)

	result := strings.Join(out, "\n")
	return result, result != md
}

// normalizeBullets returns the section body with each parseable bullet replaced by
// its canonical form, and the index one past the body it consumed. Lines that are not
// a parseable bullet are copied through untouched.
func normalizeBullets(lines []string, head, end int) (body []string, next int) {
	bullets := sectionBullets(lines, head, end)
	seen := make(map[edgeKey]bool)
	at := head + 1
	for i := range bullets {
		b := bullets[i]
		body = append(body, lines[at:b.start]...) // untouched lines before this bullet
		body = append(body, canonicalize(b, lines, seen)...)
		at = b.end
	}
	body = append(body, lines[at:end]...)
	return body, end
}

// canonicalize returns the replacement lines for one logical bullet: a canonical
// bullet per new target, or the bullet's original lines when it names no skill.
func canonicalize(b logicalBullet, lines []string, seen map[edgeKey]bool) []string {
	edges, ok := readBullet(b.text)
	if !ok {
		return lines[b.start:b.end]
	}
	out := make([]string, 0, len(edges))
	for _, e := range edges {
		key := edgeKey{kind: e.Kind, target: e.Target}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Bullet(e))
	}
	if len(out) == 0 {
		// Every target was a duplicate; drop the now-empty bullet rather than
		// leaving a legacy line that says the same thing as an earlier one.
		return nil
	}
	return out
}
