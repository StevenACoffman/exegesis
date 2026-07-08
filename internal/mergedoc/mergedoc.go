// Package mergedoc reads and writes the `## Merge Status` YAML ledger inside a
// SKILL.md body. It is the adapter that isolates gopkg.in/yaml.v3 from the pure
// book2skill domain package; book2skill defines the entry type and its
// validation, and this package marshals it into (and parses it out of) the
// fenced block.
package mergedoc

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const (
	statusHeading = "merge status" // matched case-insensitively (## Merge Status)
	headingPrefix = "## "
	fenceOpen     = "```yaml"
	fence         = "```"
)

// Parse extracts the merge-status entries from a SKILL.md body. It returns
// (nil, nil) when there is no `## Merge Status` section or no YAML block within
// it, and an error when the block is present but not valid YAML.
func Parse(md string) ([]book2skill.MergeStatusEntry, error) {
	block, ok := yamlBlock(md)
	if !ok {
		return nil, nil
	}
	var entries []book2skill.MergeStatusEntry
	if err := yaml.Unmarshal([]byte(block), &entries); err != nil {
		return nil, fmt.Errorf("mergedoc: parse merge-status block: %w", err)
	}
	return entries, nil
}

// ParseVerification reads the leading YAML frontmatter of a source-verification
// artifact into a SourceVerification. ok is false (with a nil error) when the
// document has no `---` frontmatter block.
func ParseVerification(md string) (*book2skill.SourceVerification, bool, error) {
	front, ok := frontmatter(md)
	if !ok {
		return nil, false, nil
	}
	var sv book2skill.SourceVerification
	if err := yaml.Unmarshal([]byte(front), &sv); err != nil {
		return nil, false, fmt.Errorf("mergedoc: parse verification header: %w", err)
	}
	return &sv, true, nil
}

// frontmatter returns the text between a leading "---" line and the next "---",
// and whether such a block was found.
func frontmatter(md string) (string, bool) {
	lines := strings.Split(md, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}
	var buf strings.Builder
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			return buf.String(), true
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	return "", false
}

// Append adds e to the skill's merge-status ledger and returns the updated
// markdown. It is append-only: existing entries are preserved and the section is
// rewritten with the full list, created at the end of the document when absent.
func Append(md string, e *book2skill.MergeStatusEntry) (string, error) {
	entries, err := Parse(md)
	if err != nil {
		return "", err
	}
	entries = append(entries, *e)
	block, err := yaml.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("mergedoc: encode merge-status block: %w", err)
	}
	body := stripSection(md)
	section := "## Merge Status\n\n" + fenceOpen + "\n" + strings.TrimRight(string(block), "\n") +
		"\n" + fence + "\n"
	return strings.TrimRight(body, "\n") + "\n\n" + section, nil
}

// sectionBounds returns the [start, end) line range of the merge-status section
// (heading through the line before the next level-2 heading or end of file).
func sectionBounds(lines []string) (start, end int, found bool) {
	start = -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, headingPrefix) {
			continue
		}
		if start >= 0 {
			return start, i, true // next heading ends the section
		}
		if isStatusHeading(trimmed) {
			start = i
		}
	}
	if start >= 0 {
		return start, len(lines), true
	}
	return -1, -1, false
}

func isStatusHeading(trimmed string) bool {
	rest := strings.ToLower(strings.TrimSpace(trimmed[len(headingPrefix):]))
	return rest == statusHeading || strings.HasPrefix(rest, statusHeading+" ")
}

// yamlBlock returns the content between the ```yaml fence and its closing fence
// inside the merge-status section.
func yamlBlock(md string) (string, bool) {
	lines := strings.Split(md, "\n")
	start, end, found := sectionBounds(lines)
	if !found {
		return "", false
	}
	inFence := false
	var buf strings.Builder
	for _, line := range lines[start:end] {
		trimmed := strings.TrimSpace(line)
		switch {
		case !inFence && trimmed == fenceOpen:
			inFence = true
		case inFence && trimmed == fence:
			return buf.String(), true
		case inFence:
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	return "", false
}

// stripSection returns md with the merge-status section removed (if present).
func stripSection(md string) string {
	lines := strings.Split(md, "\n")
	start, end, found := sectionBounds(lines)
	if !found {
		return md
	}
	kept := append(append([]string{}, lines[:start]...), lines[end:]...)
	return strings.Join(kept, "\n")
}
