// Package lint validates a single Agent Skill against the agentskills.io spec
// plus book2skill's body red-lines and the runtime-neutrality gate (E3). Check
// is pure over an already-loaded skill; the caller does the file I/O and decides
// the exit code from the returned findings.
package lint

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/StevenACoffman/skillet/neutrality"
	"github.com/StevenACoffman/skillet/skill"
)

// descCharLimit is the Agent Skills description length cap.
const descCharLimit = 1024

// Severity levels. Only Error findings fail the gate.
const (
	Error Severity = "error"
	Warn  Severity = "warn"
)

var (
	reFence      = regexp.MustCompile("(?s)```.*?```|~~~.*?~~~")
	reInlineCode = regexp.MustCompile("`[^`]*`")
	reAngle      = regexp.MustCompile(`[<>]`)
)

// Severity classifies a finding.
type Severity string

// Finding is one lint result.
type Finding struct {
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// Options are the opt-in, registry/flag-driven budget and structure checks. The
// zero value enforces nothing, so existing trees keep passing.
type Options struct {
	MaxBodyWords        int      // 0 = unlimited
	MaxDescriptionWords int      // 0 = unlimited
	RequiredSections    []string // heading substrings that must be present and non-empty
}

// Check returns every lint finding for s. An empty slice means the skill passes.
//
// Requires: s is a loaded skill (Name/Description/Body/FrontmatterKeys populated
//
//	from its SKILL.md).
//
// Ensures:  the result contains a Severity==Error finding for every hard defect
//
//	(disallowed key, name/dir mismatch, over-long or empty or markup-laden
//	description, escaping/absolute body links, runtime binding); it is pure.
func Check(s *skill.Skill, opts Options) []Finding {
	var fs []Finding
	fs = append(fs, checkFrontmatter(s)...)
	fs = append(fs, checkBody(s.Body)...)
	fs = append(fs, checkBudget(s, opts)...)
	for _, h := range neutrality.Scan([]neutrality.NamedFile{{Name: "SKILL.md", Content: s.Raw}}) {
		fs = append(fs, Finding{
			Severity: Error,
			Message:  fmt.Sprintf("runtime-bound wording at SKILL.md:%d: %q", h.Line, h.Text),
		})
	}
	return fs
}

// checkBudget applies the opt-in registry/flag limits: description and body word
// budgets and required, non-empty sections. Zero limits and an empty section
// list are no-ops.
func checkBudget(s *skill.Skill, opts Options) []Finding {
	var fs []Finding
	if opts.MaxDescriptionWords > 0 {
		if n := len(strings.Fields(s.Description)); n > opts.MaxDescriptionWords {
			fs = append(fs, Finding{Error, fmt.Sprintf(
				"frontmatter: description %d words > max %d", n, opts.MaxDescriptionWords)})
		}
	}
	if opts.MaxBodyWords > 0 {
		if n := len(strings.Fields(s.Body)); n > opts.MaxBodyWords {
			fs = append(fs, Finding{Error, fmt.Sprintf(
				"body: %d words > max %d", n, opts.MaxBodyWords)})
		}
	}
	for _, name := range opts.RequiredSections {
		if !sectionSatisfied(s.Body, name) {
			fs = append(fs, Finding{Error, fmt.Sprintf(
				"body: required section %q is missing or empty", name)})
		}
	}
	return fs
}

// sectionSatisfied reports whether the body has a "## ..." heading whose text
// contains name (case-insensitive) followed by non-empty, non-comment content.
func sectionSatisfied(body, name string) bool {
	want := strings.ToLower(name)
	inSection, hasContent := false, false
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			if inSection && hasContent {
				return true
			}
			inSection = strings.HasPrefix(t, "## ") && strings.Contains(strings.ToLower(t), want)
			hasContent = false
			continue
		}
		if inSection && t != "" && !strings.HasPrefix(t, "<!--") {
			hasContent = true
		}
	}
	return inSection && hasContent
}

// allowedKey reports whether k is a spec-permitted frontmatter key.
func allowedKey(k string) bool {
	switch k {
	case "name", "description", "tags", "allowed-tools":
		return true
	default:
		return false
	}
}

func checkFrontmatter(s *skill.Skill) []Finding {
	var fs []Finding
	for _, k := range s.FrontmatterKeys {
		if !allowedKey(k) {
			fs = append(fs, Finding{Error, fmt.Sprintf("frontmatter: disallowed key %q", k)})
		}
	}
	if want := filepath.Base(s.Dir); s.Name != want {
		fs = append(
			fs,
			Finding{Error, fmt.Sprintf("frontmatter: name %q != folder %q", s.Name, want)},
		)
	}
	switch n := utf8.RuneCountInString(s.Description); {
	case n == 0:
		fs = append(fs, Finding{Error, "frontmatter: description is empty"})
	case n > descCharLimit:
		fs = append(
			fs,
			Finding{Error, fmt.Sprintf("frontmatter: description %d runes > %d", n, descCharLimit)},
		)
	}
	if reAngle.MatchString(s.Description) {
		fs = append(
			fs,
			Finding{Error, "frontmatter: description contains angle brackets (must be plain text)"},
		)
	}
	return fs
}

func checkBody(body string) []Finding {
	prose := stripCode(body)
	var fs []Finding
	add := func(cond bool, msg string) {
		if cond {
			fs = append(fs, Finding{Error, msg})
		}
	}
	add(
		strings.Contains(prose, "](../"),
		"body: parent-escaping link '](../ ...)' — reference related skills by slug, not path",
	)
	add(strings.Contains(prose, "](/"), "body: absolute-path link '](/ ...)' not allowed")
	add(strings.Contains(prose, "candidates/"), "body: 'candidates/' path leaked into skill body")
	return fs
}

// stripCode removes fenced blocks and inline code spans so that code samples
// (e.g. Go generics like New[T](x)) are not misread as broken links.
func stripCode(body string) string {
	body = reFence.ReplaceAllString(body, "")
	return reInlineCode.ReplaceAllString(body, "")
}
