package book2skill

import "strings"

// RSegmentQuotes extracts the source quotes from a SKILL.md body's R segment.
// Each maximal run of blockquote (">"-prefixed) lines is one citation; within a
// run, blank and attribution lines ("> — Author") are dropped and the remaining
// lines are joined with single spaces. It supports the dual-citation R section a
// merged skill uses. Returns nil when there is no R segment or no quote.
func RSegmentQuotes(body string) []string {
	r := ParseSegments(body)[SegR]
	if r == "" {
		return nil
	}
	var (
		quotes  []string
		current []string
	)
	flush := func() {
		if len(current) > 0 {
			quotes = append(quotes, strings.Join(current, " "))
			current = nil
		}
	}
	for _, line := range strings.Split(r, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, ">") {
			flush() // a non-blockquote line ends the current citation
			continue
		}
		content := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
		if content == "" || strings.HasPrefix(content, "—") || strings.HasPrefix(content, "--") {
			continue // blank line within the block, or an attribution line
		}
		current = append(current, content)
	}
	flush()
	return quotes
}

// QuoteFound reports whether quote appears in source, comparing with runs of
// whitespace collapsed to single spaces (so line-wrapping differences do not
// matter). The comparison is otherwise verbatim; paraphrase-distance judgment is
// left to the caller.
func QuoteFound(quote, source string) bool {
	return strings.Contains(normalizeWhitespace(source), normalizeWhitespace(quote))
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
