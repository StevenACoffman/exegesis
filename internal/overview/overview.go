// Package overview implements the Stage-0 BOOK_OVERVIEW.md gate: it enforces
// that the overview has a one-sentence summary, 3-7 skeleton items, at least 5
// key terms, and at least 3 total critique items (era limitations + author blind
// spots + unproven assumptions). Check is pure over the file's bytes.
package overview

import (
	"fmt"
	"strings"
)

// Composition bounds for the Stage-0 gate.
const (
	MinSkeleton = 3
	MaxSkeleton = 7
	MinKeyTerms = 5
	MinCritique = 3
)

// Check returns one problem string per gate violation; empty means it passes.
//
// Requires: content is the full text of a BOOK_OVERVIEW.md.
// Ensures:  result is empty iff the summary is non-empty and the skeleton, key
//
//	term, and critique counts satisfy the bounds above; it is pure.
func Check(content string) []string {
	bulletsByHeading, textByHeading := parse(content)
	var problems []string

	if strings.TrimSpace(textByHeading["one-sentence summary"]) == "" {
		problems = append(problems, "missing a non-empty '## One-sentence summary'")
	}

	skeleton := bulletsByHeading["skeleton"]
	if skeleton < MinSkeleton || skeleton > MaxSkeleton {
		problems = append(problems, fmt.Sprintf(
			"skeleton has %d items, want %d-%d", skeleton, MinSkeleton, MaxSkeleton))
	}

	if terms := bulletsByHeading["key terms"]; terms < MinKeyTerms {
		problems = append(problems, fmt.Sprintf("key terms has %d, want >=%d", terms, MinKeyTerms))
	}

	critique := bulletsByHeading["era limitations"] +
		bulletsByHeading["author blind spots"] +
		bulletsByHeading["unproven assumptions"]
	if critique < MinCritique {
		problems = append(problems, fmt.Sprintf(
			"critique (era limitations + blind spots + unproven assumptions) has %d, want >=%d",
			critique, MinCritique))
	}
	return problems
}

// parse walks content once, attributing bullet lines and prose to the current
// gated H2 section. Any heading line ("#"-prefixed) ends the current section;
// a non-gated heading leaves current empty so its content is ignored. A bullet
// is a line whose trimmed form starts with "- " or "* ".
func parse(content string) (bullets map[string]int, text map[string]string) {
	bullets = map[string]int{}
	text = map[string]string{}
	current := ""
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			current = headingKey(trimmed)
			continue
		}
		if current == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
			bullets[current]++
		case trimmed != "" && !strings.HasPrefix(trimmed, "<!--"):
			text[current] += trimmed + " "
		}
	}
	return bullets, text
}

// headingKey returns the gated canonical key for a trimmed heading line, or ""
// when the line is a non-"## " heading or a "## " heading we do not gate.
func headingKey(trimmed string) string {
	if !strings.HasPrefix(trimmed, "## ") {
		return ""
	}
	title := strings.ToLower(strings.TrimSpace(trimmed[len("## "):]))
	gated := []string{
		"one-sentence summary",
		"skeleton",
		"key terms",
		"era limitations",
		"author blind spots",
		"unproven assumptions",
	}
	for _, key := range gated {
		if strings.Contains(title, key) {
			return key
		}
	}
	return ""
}
