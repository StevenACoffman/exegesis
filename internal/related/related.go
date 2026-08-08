// Package related is the pure model behind `exegesis link` and `exegesis index`:
// the related-skill edges recorded in a skill's `## Related skills` sections — a file
// may hold more than one, and they are read as a single set — plus the
// parse/serialize of those sections (related.go), the older bullet dialects still
// tolerated on read (dialects.go), the graph those edges form (graph.go), and the
// INDEX.md rendered from them (index.go). Every function is
// pure — text in, text out, no I/O and no globals — so the commands own the file
// reads and writes and decide exit codes.
package related

import (
	"fmt"
	"sort"
	"strings"
)

// sectionHeading is the exact H2 that holds a skill's related-skill edges, in the form
// this package writes.
//
// Title case because `rumdl` MD063 enforces it: with the lowercase form, `normalize`
// rewrote the heading one way and `rumdl fmt` rewrote it straight back, each undoing the
// other on the same line. The corpus had already settled it -- 179 skills title case,
// none lowercase -- so this makes normalize a no-op on the heading instead of a churn
// across every file. Matching stays case-insensitive (see isSectionHeading), so a skill
// written either way is still found.
const sectionHeading = "## Related Skills"

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
	// SupersededBy means a merge run replaced the source skill with the target,
	// which usually lives in another tree — see Qualified.
	//
	// It is terminal where the other three relate skills that both live, but it
	// changes nothing about how a skill is presented: a superseded skill keeps its
	// place in the skill list and the learning path, because merge-skills retains
	// source skills as the audit trail of what was merged, and the path is ordered
	// on DependsOn alone.
	SupersededBy Kind = "superseded-by"
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

// section is one related-skills section's line span: head is its heading line, end
// is one past its last line — the next heading, or the end of the file.
//
// A file may hold more than one: 13 skills in the real books carry a second section,
// typically a suffixed heading followed by a plain one, and a reader that stopped at
// the first silently dropped everything the second declared.
type section struct {
	head int
	end  int
}

// Kinds returns the known edge kinds, in the order a caller should offer them.
// It is the vocabulary itself, so Valid and every usage message read from one
// definition and cannot list different kinds.
func Kinds() []Kind {
	return []Kind{DependsOn, ContrastsWith, ComposesWith, SupersededBy}
}

// Valid reports whether k is one of the known kinds.
func (k Kind) Valid() bool {
	for _, known := range Kinds() {
		if k == known {
			return true
		}
	}
	return false
}

// Qualified reports whether target names a skill in another tree, written as a
// path — "merged/all-books-v1/some-skill" rather than a bare slug.
//
// The form is not new and is not specific to SupersededBy: 26 superseded-by bullets
// and 9 on the other three kinds already write it in the real books, and
// merge-skills' Phase 3 prescribes it for cross-book links of any kind. It is a
// property of a target, so nothing here branches on the edge's kind.
//
// A qualified target resolves against the parent of the tree being read, which is a
// directory a pure function cannot see. That is why the graph gate skips these and
// the commands check them instead — see DanglingEdges.
func Qualified(target string) bool {
	return strings.Contains(target, "/")
}

// Bullet renders e in the exact canonical form the section uses:
// "- <kind>: `<target>` — <rationale>". The " — " separator (space, em dash,
// space) is part of the wire format and round-trips with ParseSection.
func Bullet(e Edge) string {
	return fmt.Sprintf("- %s: `%s` — %s", e.Kind, e.Target, e.Rationale)
}

// ParseSection returns the edges declared in every `## Related skills` section of md,
// skipping any bullet whose kind is not Valid or that names no skill. md may be a full
// SKILL.md or just its body. Bullets inside code fences are ignored.
//
// Reading is tolerant of every bullet dialect found in real trees (see
// dialects.go), so a section written before the canonical format settled still
// yields its edges instead of being silently ignored. A bullet naming several
// targets yields one edge per target.
//
// Edges are deduplicated by (Kind, Target), first occurrence winning: a section
// expresses a set of relationships, and once legacy and canonical bullets coexist —
// which happens the first time `relate` runs over legacy content, and in every file
// carrying two sections — the same relationship would otherwise be reported twice.
//
// The result is sorted by kind, then target, rather than left in file order. A
// relationship means the same thing whichever of a file's sections happens to state
// it, so the reader's answer should not depend on that; sorting also makes merging two
// sections produce one order rather than an order that records which section won.
// Nothing downstream reads file order: Mermaid sorts its edge lines, DanglingEdges
// sorts, LearningPath only counts indegrees, and INDEX.md never lists a skill's edges.
//
// Ensures: the result is sorted and holds no two edges with the same (Kind, Target);
// it is pure.
func ParseSection(md string) []Edge {
	lines := strings.Split(md, "\n")
	var edges []Edge
	seen := make(map[edgeKey]bool)
	for _, sec := range findSections(lines) {
		for _, b := range sectionBullets(lines, sec.head, sec.end) {
			fresh, _ := newEdges(b.text, seen)
			edges = append(edges, fresh...)
		}
	}
	sortEdges(edges)
	return edges
}

// Upsert returns md with e recorded in a `## Related skills` section and whether md
// changed. It is idempotent by (Kind, Target): an identical bullet is a no-op; a
// bullet with the same kind and target but a different rationale is rewritten in
// place; otherwise the bullet is appended (creating the section at end of file when
// absent). md is never mutated.
//
// An existing bullet is looked for in every section but a new one is written to the
// first, so a file carrying two sections does not gain a duplicate bullet the first
// time `relate` or `link` records a relationship its second section already states.
//
// Requires: e.Kind.Valid() and e.Target != "".
// Ensures:  ParseSection(out) contains e; Upsert(out, e) == (out, false).
func Upsert(md string, e Edge) (string, bool) {
	lines := strings.Split(md, "\n")
	secs := findSections(lines)
	if len(secs) == 0 {
		return appendSection(md, e), true
	}
	bullet := Bullet(e)
	if at := findEdge(lines, secs, e.Kind, e.Target); at >= 0 {
		if lines[at] == bullet {
			return md, false
		}
		lines[at] = bullet
		return strings.Join(lines, "\n"), true
	}
	at := insertAt(lines, secs[0])
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

// findSections returns every related-skills section in lines, in file order. A
// section runs from its heading to the next heading of any level, or to the end of
// the file; headings inside a code fence are text, not boundaries.
//
// Returning all of them rather than the first is what makes a second section
// visible: 13 skills in the real books carry one, and everything it declared used to
// be dropped without a word.
//
// Ensures: the spans are ascending and non-overlapping, and each head is a heading
// line satisfying isSectionHeading; it is pure.
func findSections(lines []string) []section {
	var out []section
	open := -1
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isFence(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence || !isHeading(trimmed) {
			continue
		}
		if open >= 0 {
			out = append(out, section{head: open, end: i})
			open = -1
		}
		if isSectionHeading(trimmed) {
			open = i
		}
	}
	if open >= 0 {
		out = append(out, section{head: open, end: len(lines)})
	}
	return out
}

// newEdges returns the edges of one bullet that are not already in seen, recording
// them there, and whether the bullet was understood at all. It is the shared body of
// the read and normalize paths, so the two cannot disagree about which repeat of a
// relationship is the one that counts.
//
// The two answers are distinct: a bullet naming only relationships already seen is
// understood and yields nothing, while a bullet naming no skill is not understood and
// must be preserved verbatim by the caller that rewrites files.
func newEdges(bullet string, seen map[edgeKey]bool) ([]Edge, bool) {
	parsed, understood := readBullet(bullet)
	if !understood {
		return nil, false
	}
	out := make([]Edge, 0, len(parsed))
	for _, e := range parsed {
		key := edgeKey{kind: e.Kind, target: e.Target}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	return out, true
}

// sortEdges orders edges by kind, then target: the canonical order, independent of
// which section or dialect each edge was written in.
func sortEdges(edges []Edge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Kind != edges[j].Kind {
			return edges[i].Kind < edges[j].Kind
		}
		return edges[i].Target < edges[j].Target
	})
}

// findEdge returns the line index of the first bullet in any of secs with the given
// kind and target, or -1.
func findEdge(lines []string, secs []section, k Kind, target string) int {
	for _, sec := range secs {
		for i := sec.head + 1; i < sec.end; i++ {
			if e, ok := parseBullet(lines[i]); ok && e.Kind == k && e.Target == target {
				return i
			}
		}
	}
	return -1
}

// insertAt returns the index just after the last bullet of sec, or the line after
// its heading when the section has no bullets yet.
func insertAt(lines []string, sec section) int {
	at := sec.head + 1
	for i := at; i < sec.end; i++ {
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
