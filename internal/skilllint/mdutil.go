package skilllint

import (
	"regexp"
	"strconv"
	"strings"
)

const minFenceLen = 3

// LinkFragment is a markdown link that carries a "#fragment"; Path is the file
// part (empty for a same-document "#frag" link).
type LinkFragment struct {
	Path     string
	Fragment string
}

// SlugifyHeading converts a heading to a GitHub-style anchor slug: inline code
// and link syntax reduce to visible text, the result is lowercased, characters
// outside word/space/hyphen are dropped, and spaces become hyphens (runs are not
// collapsed). It is kept deliberately GitHub-compatible; goldmark's own auto-ID is
// only approximately GitHub's algorithm, so the slug policy stays here.
func SlugifyHeading(text string) string {
	text = reInlineCode().ReplaceAllString(text, "$1")
	text = reImageLink().ReplaceAllString(text, "$1")
	text = reLink().ReplaceAllString(text, "$1")
	text = strings.ToLower(text)
	text = reNonSlug().ReplaceAllString(text, "")
	return strings.ReplaceAll(text, " ", "-")
}

// ExtractHeadings returns the set of heading slugs in text (outside code blocks),
// suffixing duplicates GitHub-style (intro, intro-1, intro-2). Heading structure
// comes from goldmark, so a "#" inside a code fence and a list item followed by a
// line of dashes are correctly not headings.
func ExtractHeadings(text string) map[string]bool {
	slugs := make(map[string]bool)
	counts := make(map[string]int)
	for _, heading := range headingTexts(text) {
		base := SlugifyHeading(heading)
		if n := counts[base]; n == 0 {
			slugs[base] = true
		} else {
			slugs[base+"-"+strconv.Itoa(n)] = true
		}
		counts[base]++
	}
	return slugs
}

// ExtractLocalLinkTargets returns local (non-scheme, non-fragment-only) link and
// image targets in text, with any query/fragment suffix stripped.
func ExtractLocalLinkTargets(text string) []string {
	var targets []string
	for _, link := range mdLinks(text) {
		target := strings.TrimSpace(link.Destination)
		if hasNonLocalScheme(target) || strings.HasPrefix(target, "#") {
			continue
		}
		target = trimAfter(trimAfter(target, '?'), '#')
		if target != "" {
			targets = append(targets, target)
		}
	}
	return targets
}

// ExtractFragmentLinks returns links carrying a "#fragment" (excluding images and
// remote schemes), as (path, fragment) pairs.
func ExtractFragmentLinks(text string) []LinkFragment {
	var out []LinkFragment
	for _, link := range mdLinks(text) {
		if link.IsImage {
			continue
		}
		target := strings.TrimSpace(link.Destination)
		if strings.HasPrefix(target, "http://") ||
			strings.HasPrefix(target, "https://") ||
			strings.HasPrefix(target, "mailto:") ||
			!strings.Contains(target, "#") {
			continue
		}
		path, fragment, _ := strings.Cut(target, "#")
		path = trimAfter(path, '?')
		if fragment != "" {
			out = append(out, LinkFragment{Path: path, Fragment: fragment})
		}
	}
	return out
}

// FindUnclosedFence returns the 1-based line number of an unclosed code fence, or
// (0, false) when every fence is closed. Detecting an unclosed fence is a lint
// concern a spec parser abstracts away (goldmark auto-closes at EOF), so this stays
// a direct line scan.
func FindUnclosedFence(text string) (int, bool) {
	inFence := false
	fenceChar := byte(0)
	fenceLen := 0
	fenceLine := 0
	for i, line := range strings.Split(text, "\n") {
		stripped := trimLeadingSpaces(line, minFenceLen)
		char, count := fencePrefix(stripped)
		switch {
		case !inFence && count >= minFenceLen:
			inFence, fenceChar, fenceLen, fenceLine = true, char, count, i+1
		case inFence && count >= fenceLen && char == fenceChar && strings.TrimSpace(stripped[count:]) == "":
			inFence = false
		}
	}
	return fenceLine, inFence
}

func fencePrefix(line string) (byte, int) {
	if line == "" || (line[0] != '`' && line[0] != '~') {
		return 0, 0
	}
	char := line[0]
	count := 0
	for count < len(line) && line[count] == char {
		count++
	}
	if count < minFenceLen {
		return 0, 0
	}
	return char, count
}

func trimLeadingSpaces(line string, maxSpaces int) string {
	for i := 0; i < maxSpaces && strings.HasPrefix(line, " "); i++ {
		line = line[1:]
	}
	return line
}

func hasNonLocalScheme(target string) bool {
	lower := strings.ToLower(target)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:")
}

func trimAfter(s string, sep byte) string {
	if i := strings.IndexByte(s, sep); i >= 0 {
		return s[:i]
	}
	return s
}

func reInlineCode() *regexp.Regexp { return regexp.MustCompile("`([^`]+)`") }
func reImageLink() *regexp.Regexp  { return regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`) }
func reLink() *regexp.Regexp       { return regexp.MustCompile(`\[([^\]]*)\]\([^)]+\)`) }
func reNonSlug() *regexp.Regexp    { return regexp.MustCompile(`[^\p{L}\p{N}_\- ]`) }
