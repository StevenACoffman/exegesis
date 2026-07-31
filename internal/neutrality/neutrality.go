// Package neutrality is the runtime-neutrality red-light scan (E3): it flags
// wording or paths that bind a skill to one agent runtime, which makes other
// agents refuse to install it. book2skill skills MUST be agent-agnostic, so
// exegesis lint runs this at authoring time — the same check skillsaw scan runs
// downstream. The red-light pattern is byte-identical to skillsaw's so the two
// tools never disagree about what "runtime-bound" means.
package neutrality

import (
	"regexp"
	"strings"
)

// redLight matches the darwin runtime-neutrality red lights (SKILL.md §"Runtime
// 适配性审查"). Applied per line; the "^" anchor matches each line's start.
var redLight = regexp.MustCompile(
	`(在 Claude Code|Claude Code skill|Claude Code 用户|Cursor only|Codex 中|^\[!\[Claude Code|~/\.claude/skills/[a-z]|/plugin install\b)`,
)

// Hit is one red-light match.
type Hit struct {
	File string `json:"file"`
	Line int    `json:"line"` // 1-indexed
	Text string `json:"text"` // the matching line, trimmed
}

// NamedFile pairs a display name with file contents for Scan.
type NamedFile struct {
	Name    string
	Content string
}

// Scan runs the red-light regex line-by-line over each file. Returned hits
// follow the given file order, then line number. It is pure: values in, values
// out, no filesystem access.
func Scan(files []NamedFile) []Hit {
	var hits []Hit
	for _, f := range files {
		lines := strings.Split(strings.ReplaceAll(f.Content, "\r\n", "\n"), "\n")
		for i, line := range lines {
			if redLight.MatchString(line) {
				hits = append(hits, Hit{File: f.Name, Line: i + 1, Text: strings.TrimSpace(line)})
			}
		}
	}
	return hits
}
