package book2skill

import (
	"regexp"
	"strings"
)

// Canonical SKILL.md segment tags. Each of the six RIA++ segments begins with a
// level-2 heading whose first whitespace-delimited token after "## " is the tag;
// decorative text may follow (for example "## R — Original text (Reading)").
// merge-skills relies on this contract to extract segments deterministically,
// without an LLM.
const (
	// SegR is the Reading (original text) segment tag.
	SegR = "R"
	// SegI is the Interpretation segment tag.
	SegI = "I"
	// SegA1 is the Past Application segment tag.
	SegA1 = "A1"
	// SegA2 is the Future Trigger segment tag.
	SegA2 = "A2"
	// SegE is the Execution segment tag.
	SegE = "E"
	// SegB is the Boundary segment tag.
	SegB = "B"

	// RelatedSkillsHeading is the level-2 heading under which a SKILL.md lists
	// its Zettelkasten relationships. render and ParseRelated share it so the
	// SKILL.md format decision lives in exactly one place. It is in the
	// title-cased form a markdown formatter's heading-case rule produces;
	// ParseRelated matches it case-insensitively regardless.
	RelatedSkillsHeading = "Related Skills"

	headingPrefix = "## "
)

// SegmentTags returns the six segment tags in canonical document order. It
// returns a fresh slice each call so callers cannot mutate shared state.
func SegmentTags() []string {
	return []string{SegR, SegI, SegA1, SegA2, SegE, SegB}
}

// SegmentTagFromHeading returns the segment tag for a heading line and reports
// whether the line is a recognized segment heading. It accepts headings of the
// form "## <TAG>" optionally followed by whitespace and decorative text.
func SegmentTagFromHeading(line string) (string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(line), headingPrefix)
	if !ok {
		return "", false
	}
	tag := rest
	if i := strings.IndexAny(rest, " \t"); i >= 0 {
		tag = rest[:i]
	}
	switch tag {
	case SegR, SegI, SegA1, SegA2, SegE, SegB:
		return tag, true
	default:
		return "", false
	}
}

// ParseSegments splits SKILL.md body text into its RIA++ segments, keyed by tag.
// Content before the first recognized segment heading is ignored, and missing
// segments are simply absent from the returned map. Each value is the segment
// body with surrounding whitespace trimmed.
func ParseSegments(md string) map[string]string {
	segments := make(map[string]string)
	fenced := fencedLines(md)
	current := ""
	var buf strings.Builder
	flush := func() {
		if current != "" {
			segments[current] = strings.TrimSpace(buf.String())
		}
		buf.Reset()
	}
	for i, line := range strings.Split(md, "\n") {
		if tag, ok := SegmentTagFromHeading(line); ok && !fenced[i] {
			flush()
			current = tag
			continue
		}
		if current != "" {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	flush()
	return segments
}

// ParseTitle returns the text of the first level-1 heading ("# ...") in md, with
// surrounding whitespace trimmed. It returns "" when md has no such heading.
func ParseTitle(md string) string {
	fenced := fencedLines(md)
	for i, line := range strings.Split(md, "\n") {
		if fenced[i] {
			continue
		}
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "# "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// ParseRelated parses the "## Related skills" section of a SKILL.md body into
// relationships whose From is set to fromSlug. It is the inverse of
// render.renderRelated: each bullet has the shape
// "- <kind>: `<to>` — <rationale>". Bullets with an unknown kind are skipped.
func ParseRelated(fromSlug, md string) []Relationship {
	body, ok := sectionBody(md, RelatedSkillsHeading)
	if !ok {
		return nil
	}
	re := relatedItemRE()
	var rels []Relationship
	for _, line := range strings.Split(body, "\n") {
		m := re.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		kind := RelationshipKind(m[1])
		if !kind.Valid() {
			continue
		}
		rels = append(rels, Relationship{
			From: fromSlug, To: m[2], Kind: kind, Rationale: strings.TrimSpace(m[3]),
		})
	}
	return rels
}

// sectionBody returns the text beneath the "## <heading>" line, up to the next
// level-2 heading or end of document, and whether the heading was found.
func sectionBody(md, heading string) (string, bool) {
	lines := strings.Split(md, "\n")
	start, end, found := sectionRange(lines, fencedLines(md), heading)
	if !found {
		return "", false
	}
	return strings.Join(lines[start+1:end], "\n"), true
}

// sectionRange returns the [start, end) line range of the "## <heading>" section:
// start is the heading line, end is the next level-2 heading (or len(lines)).
// Matching is case-insensitive so title-cased headings (as a markdown formatter's
// heading-case rule produces, e.g. "## Related Skills" from "## Related skills")
// are still found, and a decorative suffix after the heading text is tolerated
// ("## Related Skills (Stage 3)"), matching SegmentTagFromHeading's convention.
func sectionRange(
	lines []string,
	fenced map[int]bool,
	heading string,
) (start, end int, found bool) {
	want := strings.ToLower(headingPrefix + heading)
	start = -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if fenced[i] || !strings.HasPrefix(trimmed, headingPrefix) {
			continue
		}
		if start >= 0 {
			return start, i, true // reached the next section
		}
		lower := strings.ToLower(trimmed)
		if lower == want || strings.HasPrefix(lower, want+" ") {
			start = i
		}
	}
	if start >= 0 {
		return start, len(lines), true
	}
	return -1, -1, false
}

// listItems returns the text of each "- " bullet in body, trimmed of the marker
// and surrounding whitespace. Non-bullet lines (prose, bold paragraphs) are
// ignored, so it counts a section's list without being confused by trailing
// notes.
func listItems(body string) []string {
	var items []string
	for _, line := range strings.Split(body, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "- "); ok {
			if item := strings.TrimSpace(rest); item != "" {
				items = append(items, item)
			}
		}
	}
	return items
}

// relatedItemRE matches one rendered "## Related skills" bullet:
// "- <kind>: `<to>` — <rationale>", with an optional rationale. Compiled per
// call to avoid a package-level global.
func relatedItemRE() *regexp.Regexp {
	return regexp.MustCompile("^-\\s+([a-z][a-z-]*):\\s+`([^`]+)`(?:\\s*[—-]\\s*(.*))?$")
}
