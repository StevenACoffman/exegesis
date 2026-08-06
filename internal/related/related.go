// Package related is the pure model behind `exegesis link` and `exegesis index`:
// the related-skill edges recorded in a skill's `## Related skills` section, plus
// the parse/serialize of that section (related.go), the older bullet dialects still
// tolerated on read (dialects.go), the graph those edges form (graph.go), and the
// INDEX.md rendered from them (index.go). Every function is
// pure — text in, text out, no I/O and no globals — so the commands own the file
// reads and writes and decide exit codes.
package related

import (
	"fmt"
	"strings"
)

// sectionHeading is the exact H2 that holds a skill's related-skill edges.
const sectionHeading = "## Related skills"

// The known related-skill edge kinds. Any other kind is invalid: `link` rejects
// it, `index` skips it on read.
const (
	// DependsOn means the source skill needs the target as a prerequisite;
	// `index` topologically sorts the learning path on these edges.
	DependsOn Kind = "depends-on"
	// ContrastsWith means the two skills are alternatives worth comparing.
	ContrastsWith Kind = "contrasts-with"
	// ComposesWith means the two skills are used together.
	ComposesWith Kind = "composes-with"
)

// Kind is the relationship a related-skill edge expresses. Only the three known
// kinds are valid: `link` rejects an unknown kind, `index` skips one on read.
type Kind string

// Edge is one related-skill relationship: its kind, the target skill slug, and a
// short rationale. It is parsed from, and rendered to, a `## Related skills`
// bullet.
type Edge struct {
	Kind      Kind
	Target    string
	Rationale string
}

// edgeKey identifies a relationship for deduplication. The rationale is
// deliberately excluded: two bullets naming the same kind and target are one edge,
// whichever words each chose to explain it.
type edgeKey struct {
	kind   Kind
	target string
}

// logicalBullet is one bullet of a `## Related skills` section together with the
// line span it occupies. The span lets Normalize replace exactly the lines a bullet
// came from, so a rewrite never has to reconstruct the lines it did not understand.
type logicalBullet struct {
	text  string // the bullet with any continuation lines folded into one line
	start int    // index of the bullet's first line
	end   int    // one past the index of its last line
}

// Valid reports whether k is one of the three known kinds.
func (k Kind) Valid() bool {
	switch k {
	case DependsOn, ContrastsWith, ComposesWith:
		return true
	default:
		return false
	}
}

// Bullet renders e in the exact canonical form the section uses:
// "- <kind>: `<target>` — <rationale>". The " — " separator (space, em dash,
// space) is part of the wire format and round-trips with ParseSection.
func Bullet(e Edge) string {
	return fmt.Sprintf("- %s: `%s` — %s", e.Kind, e.Target, e.Rationale)
}

// ParseSection returns the edges in md's `## Related skills` section, in file
// order, skipping any bullet whose kind is not Valid or that names no skill. md may
// be a full SKILL.md or just its body. Bullets inside code fences are ignored.
//
// Reading is tolerant of every bullet dialect found in real trees (see
// dialects.go), so a section written before the canonical format settled still
// yields its edges instead of being silently ignored. A bullet naming several
// targets yields one edge per target.
//
// Edges are deduplicated by (Kind, Target), first occurrence winning: the section
// expresses a set of relationships, and once legacy and canonical bullets coexist in
// one section — which happens the first time `relate` runs over legacy content — the
// same relationship would otherwise be reported twice.
func ParseSection(md string) []Edge {
	lines := strings.Split(md, "\n")
	head, end, found := findSection(lines)
	if !found {
		return nil
	}
	var edges []Edge
	seen := make(map[edgeKey]bool)
	for _, b := range sectionBullets(lines, head, end) {
		parsed, ok := readBullet(b.text)
		if !ok {
			continue
		}
		for _, e := range parsed {
			key := edgeKey{kind: e.Kind, target: e.Target}
			if seen[key] {
				continue
			}
			seen[key] = true
			edges = append(edges, e)
		}
	}
	return edges
}

// Upsert returns md with e recorded in its `## Related skills` section and
// whether md changed. It is idempotent by (Kind, Target): an identical bullet is
// a no-op; a bullet with the same kind and target but a different rationale is
// rewritten in place; otherwise the bullet is appended (creating the section at
// end of file when absent). md is never mutated.
//
// Requires: e.Kind.Valid() and e.Target != "".
// Ensures:  ParseSection(out) contains e; Upsert(out, e) == (out, false).
func Upsert(md string, e Edge) (string, bool) {
	lines := strings.Split(md, "\n")
	head, end, found := findSection(lines)
	if !found {
		return appendSection(md, e), true
	}
	bullet := Bullet(e)
	if at := findEdge(lines, head+1, end, e.Kind, e.Target); at >= 0 {
		if lines[at] == bullet {
			return md, false
		}
		lines[at] = bullet
		return strings.Join(lines, "\n"), true
	}
	at := insertAt(lines, head+1, end)
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:at]...)
	out = append(out, bullet)
	out = append(out, lines[at:]...)
	return strings.Join(out, "\n"), true
}

// UpsertAll applies every edge to md in order, returning the final content and whether
// any edge changed it. It is Upsert folded over a slice, so it is idempotent as a whole:
// a second call with the same edges returns (md, false).
func UpsertAll(md string, edges []Edge) (string, bool) {
	out, changed := md, false
	for i := range edges {
		next, did := Upsert(out, edges[i])
		out = next
		changed = changed || did
	}
	return out, changed
}

// findSection returns the [head, end) line range of the `## Related skills`
// section: head is the heading line index, end is the first line after the
// section (the next heading, or len(lines)). found is false when no such
// heading exists outside a code fence.
func findSection(lines []string) (head, end int, found bool) {
	head = -1
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isFence(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		switch {
		case head < 0 && isSectionHeading(trimmed):
			head = i
		case head >= 0 && isHeading(trimmed):
			return head, i, true
		}
	}
	if head < 0 {
		return -1, -1, false
	}
	return head, len(lines), true
}

// findEdge returns the line index in [start, end) of the first bullet with the
// given kind and target, or -1.
func findEdge(lines []string, start, end int, k Kind, target string) int {
	for i := start; i < end; i++ {
		if e, ok := parseBullet(lines[i]); ok && e.Kind == k && e.Target == target {
			return i
		}
	}
	return -1
}

// insertAt returns the index just after the last bullet in [start, end), or
// start when the section has no bullets yet.
func insertAt(lines []string, start, end int) int {
	at := start
	for i := start; i < end; i++ {
		if _, ok := parseBullet(lines[i]); ok {
			at = i + 1
		}
	}
	return at
}

// appendSection returns md with a fresh `## Related skills` section holding e,
// separated from the existing content by a blank line.
func appendSection(md string, e Edge) string {
	body := strings.TrimRight(md, "\n")
	return body + "\n\n" + sectionHeading + "\n\n" + Bullet(e) + "\n"
}

// parseBullet parses one "- <kind>: `<target>` — <rationale>" line. ok is false
// for any line that is not a bullet with a known kind and a backticked target.
func parseBullet(line string) (Edge, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(line), "- ")
	if !ok {
		return Edge{}, false
	}
	kindStr, tail, ok := strings.Cut(rest, ": ")
	if !ok {
		return Edge{}, false
	}
	kind := Kind(kindStr)
	if !kind.Valid() {
		return Edge{}, false
	}
	tail, ok = strings.CutPrefix(strings.TrimSpace(tail), "`")
	if !ok {
		return Edge{}, false
	}
	target, rationale, ok := strings.Cut(tail, "`")
	if !ok || target == "" {
		return Edge{}, false
	}
	rationale = strings.TrimPrefix(strings.TrimSpace(rationale), "—")
	return Edge{Kind: kind, Target: target, Rationale: strings.TrimSpace(rationale)}, true
}

// isSectionHeading reports whether a trimmed line opens the related-skills
// section. Matching is by prefix and case-insensitive, because a section exegesis
// cannot find is a section whose edges it silently drops, and real trees vary the
// heading in both ways: "## Related Skills" appears in 189 files and
// "## Related skills (Stage 3 Filling)" in 49.
//
// The level is still exact, so a deeper "### Related skills" is just a heading, and
// a suffix must start at a word boundary, so "## Related skillset" is not a match.
//
// Upsert therefore writes into a variant section when one exists, rather than
// appending a second canonical section below it.
func isSectionHeading(trimmed string) bool {
	if len(trimmed) < len(sectionHeading) {
		return false
	}
	if !strings.EqualFold(trimmed[:len(sectionHeading)], sectionHeading) {
		return false
	}
	rest := trimmed[len(sectionHeading):]
	return rest == "" || strings.HasPrefix(rest, " ")
}

// isHeading reports whether a trimmed line is an ATX heading.
func isHeading(trimmed string) bool {
	return strings.HasPrefix(trimmed, "#")
}

// isFence reports whether a trimmed line opens or closes a code fence.
func isFence(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}
