package skilllint

import (
	"regexp"
	"sort"
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

// StripCodeBlocks removes fenced and indented code blocks but leaves inline code
// spans intact.
func StripCodeBlocks(text string) string {
	return stripIndentedBlocks(stripFencedBlocks(text))
}

// StripCode removes code blocks and inline code spans.
func StripCode(text string) string {
	return reInlineCode().ReplaceAllString(StripCodeBlocks(text), "")
}

// SlugifyHeading converts a heading to a GitHub-style anchor slug: inline code
// and link syntax reduce to visible text, the result is lowercased, characters
// outside word/space/hyphen are dropped, and spaces become hyphens (runs are not
// collapsed).
func SlugifyHeading(text string) string {
	text = reInlineCode().ReplaceAllString(text, "$1")
	text = reImageLink().ReplaceAllString(text, "$1")
	text = reLink().ReplaceAllString(text, "$1")
	text = strings.ToLower(text)
	text = reNonSlug().ReplaceAllString(text, "")
	return strings.ReplaceAll(text, " ", "-")
}

// ExtractHeadings returns the set of heading slugs in text (outside code blocks),
// suffixing duplicates GitHub-style (intro, intro-1, intro-2).
func ExtractHeadings(text string) map[string]bool {
	clean := StripCodeBlocks(text)
	raw := orderedHeadingText(clean)

	slugs := make(map[string]bool)
	counts := make(map[string]int)
	for _, heading := range raw {
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

// ExtractLocalLinkTargets returns local (non-scheme, non-fragment-only) link
// targets in text, with any query/fragment suffix stripped.
func ExtractLocalLinkTargets(text string) []string {
	var targets []string
	for _, m := range reMDLink().FindAllStringSubmatch(StripCode(text), -1) {
		target := strings.TrimSpace(m[2])
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
	for _, m := range reMDLink().FindAllStringSubmatch(StripCode(text), -1) {
		if strings.HasPrefix(m[0], "!") {
			continue
		}
		target := strings.TrimSpace(m[2])
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
// (0, false) when every fence is closed.
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

func orderedHeadingText(clean string) []string {
	type positioned struct {
		pos  int
		text string
	}
	var items []positioned
	for _, m := range reATXHeading().FindAllStringSubmatchIndex(clean, -1) {
		items = append(items, positioned{pos: m[0], text: clean[m[4]:m[5]]})
	}
	for _, re := range []*regexp.Regexp{reSetextH1(), reSetextH2()} {
		for _, m := range re.FindAllStringSubmatchIndex(clean, -1) {
			line := strings.TrimSpace(clean[m[2]:m[3]])
			if !strings.HasPrefix(line, "#") {
				items = append(items, positioned{pos: m[0], text: line})
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].pos < items[j].pos })

	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.text
	}
	return out
}

func stripFencedBlocks(text string) string {
	var out []string
	inFence := false
	marker := ""
	for _, line := range strings.Split(text, "\n") {
		if inFence {
			if strings.TrimSpace(line) == marker {
				inFence = false
			}
			continue
		}
		if m := reFenceOpen().FindStringSubmatch(line); m != nil {
			inFence = true
			marker = m[1]
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func stripIndentedBlocks(text string) string {
	var out []string
	prevBlank := true
	for _, line := range strings.Split(text, "\n") {
		indented := strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")
		blank := strings.TrimSpace(line) == ""
		if indented && droppableIndent(out, prevBlank, blank) {
			prevBlank = false
			continue
		}
		out = append(out, line)
		prevBlank = blank
	}
	return strings.Join(out, "\n")
}

// droppableIndent reports whether an indented line should be treated as code and
// dropped, given the previous line's blankness and the lines kept so far.
func droppableIndent(kept []string, prevBlank, blank bool) bool {
	if prevBlank && !blank {
		return true
	}
	if prevBlank || len(kept) == 0 {
		return false
	}
	last := kept[len(kept)-1]
	return strings.TrimSpace(last) == "" ||
		strings.HasPrefix(last, "    ") || strings.HasPrefix(last, "\t")
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

func reMDLink() *regexp.Regexp     { return regexp.MustCompile(`!?\[([^\]]*)\]\(([^)]+)\)`) }
func reInlineCode() *regexp.Regexp { return regexp.MustCompile("`([^`]+)`") }
func reImageLink() *regexp.Regexp  { return regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`) }
func reLink() *regexp.Regexp       { return regexp.MustCompile(`\[([^\]]*)\]\([^)]+\)`) }
func reNonSlug() *regexp.Regexp    { return regexp.MustCompile(`[^\p{L}\p{N}_\- ]`) }
func reFenceOpen() *regexp.Regexp  { return regexp.MustCompile("^(`{3,}|~{3,})") }
func reATXHeading() *regexp.Regexp { return regexp.MustCompile(`(?m)^(#{1,6})\s+(.+?)(?:\s+#*)?$`) }
func reSetextH1() *regexp.Regexp   { return regexp.MustCompile(`(?m)^(.+)\n=+\s*$`) }
func reSetextH2() *regexp.Regexp   { return regexp.MustCompile(`(?m)^(.+)\n-+\s*$`) }
