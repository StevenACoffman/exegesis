package book2skill

import "strings"

// mdSection is one level-2 section of a markdown document.
type mdSection struct {
	heading string // text after "## ", trimmed
	text    string // the whole section, heading line through the next "##"/EOF
}

// AppendCustomSections returns generated with any level-2 section of existing
// whose heading generated does not itself contain, appended in existing's order.
// It preserves hand-authored sections (e.g. "## Notes") across regeneration of a
// tool-owned document. Heading comparison is case-insensitive, so a markdown
// formatter's heading title-casing does not defeat it, and "owned" is read from
// generated's own headings so it can never drift from what the renderer emits.
//
// A section a prior run generated but the current run omits (e.g. an optional
// section whose inputs were removed) is treated as custom and kept; regenerate
// with the same inputs to avoid stale carryover.
func AppendCustomSections(generated, existing string) string {
	owned := make(map[string]bool)
	for _, s := range markdownSections(generated) {
		owned[strings.ToLower(s.heading)] = true
	}
	var extra []string
	for _, s := range markdownSections(existing) {
		if !owned[strings.ToLower(s.heading)] {
			extra = append(extra, s.text)
		}
	}
	if len(extra) == 0 {
		return generated
	}
	return strings.TrimRight(generated, "\n") + "\n\n" + strings.Join(extra, "\n\n") + "\n"
}

// markdownSections splits md into its level-2 ("## ") sections. Content before
// the first level-2 heading (frontmatter, title, intro) is not a section.
func markdownSections(md string) []mdSection {
	lines := strings.Split(md, "\n")
	var sections []mdSection
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		heading := strings.TrimSpace(
			strings.TrimPrefix(strings.TrimSpace(lines[start]), headingPrefix),
		)
		text := strings.TrimRight(strings.Join(lines[start:end], "\n"), "\n")
		sections = append(sections, mdSection{heading: heading, text: text})
	}
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), headingPrefix) {
			flush(i)
			start = i
		}
	}
	flush(len(lines))
	return sections
}
