// Package lint validates a single Agent Skill against the agentskills.io spec
// plus book2skill's body red-lines and the runtime-neutrality gate (E3). Check
// is pure over an already-loaded skill; the caller does the file I/O and decides
// the exit code from the returned diagnostics. The agentskills.io frontmatter
// rules live in skillet's speclint so exegesis and skillsaw share one source of
// truth; the checks below are exegesis-specific (folder match, body red-lines,
// runtime neutrality, opt-in budgets).
package lint

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/neutrality"
	"github.com/StevenACoffman/skillet/skill"
	"github.com/StevenACoffman/skillet/speclint"
)

var (
	reFence      = regexp.MustCompile("(?s)```.*?```|~~~.*?~~~")
	reInlineCode = regexp.MustCompile("`[^`]*`")
)

// Options are the opt-in, registry/flag-driven budget and structure checks. The
// zero value enforces nothing, so existing trees keep passing.
type Options struct {
	MaxBodyWords        int      // 0 = unlimited
	MaxDescriptionWords int      // 0 = unlimited
	RequiredSections    []string // heading substrings that must be present and non-empty
}

// Check returns every lint diagnostic for s. An empty slice means the skill passes.
//
// Requires: s is a loaded skill (Name/Description/Body/FrontmatterKeys populated
//
//	from its SKILL.md).
//
// Ensures:  the result contains a Severity==error diagnostic for every hard defect
//
//	(disallowed key, name/dir mismatch, over-long or empty or markup-laden
//	description, escaping/absolute body links, runtime binding); it is pure.
func Check(s *skill.Skill, opts Options) []finding.Diagnostic {
	ds := speclint.Frontmatter(s)
	if want := filepath.Base(s.Dir); s.Name != want {
		ds = append(ds, diagf("frontmatter: name %q != folder %q", s.Name, want))
	}
	ds = append(ds, checkBody(s.Body)...)
	ds = append(ds, checkBudget(s, opts)...)
	for _, h := range neutrality.Scan([]neutrality.NamedFile{{Name: "SKILL.md", Content: s.Raw}}) {
		ds = append(ds, diagf("runtime-bound wording at SKILL.md:%d: %q", h.Line, h.Text))
	}
	return ds
}

// checkBudget applies the opt-in registry/flag limits: description and body word
// budgets and required, non-empty sections. Zero limits and an empty section
// list are no-ops.
func checkBudget(s *skill.Skill, opts Options) []finding.Diagnostic {
	var ds []finding.Diagnostic
	if opts.MaxDescriptionWords > 0 {
		if n := len(strings.Fields(s.Description)); n > opts.MaxDescriptionWords {
			ds = append(ds, diagf(
				"frontmatter: description %d words > max %d", n, opts.MaxDescriptionWords))
		}
	}
	if opts.MaxBodyWords > 0 {
		if n := len(strings.Fields(s.Body)); n > opts.MaxBodyWords {
			ds = append(ds, diagf("body: %d words > max %d", n, opts.MaxBodyWords))
		}
	}
	for _, name := range opts.RequiredSections {
		if !sectionSatisfied(s.Body, name) {
			ds = append(ds, diagf("body: required section %q is missing or empty", name))
		}
	}
	return ds
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

func checkBody(body string) []finding.Diagnostic {
	prose := stripCode(body)
	var ds []finding.Diagnostic
	add := func(cond bool, msg string) {
		if cond {
			ds = append(ds, diag(msg))
		}
	}
	add(
		strings.Contains(prose, "](../"),
		"body: parent-escaping link '](../ ...)' — reference related skills by slug, not path",
	)
	add(strings.Contains(prose, "](/"), "body: absolute-path link '](/ ...)' not allowed")
	add(strings.Contains(prose, "candidates/"), "body: 'candidates/' path leaked into skill body")
	return ds
}

// stripCode removes fenced blocks and inline code spans so that code samples
// (e.g. Go generics like New[T](x)) are not misread as broken links.
func stripCode(body string) string {
	body = reFence.ReplaceAllString(body, "")
	return reInlineCode.ReplaceAllString(body, "")
}

// diag builds an error-severity diagnostic. Category and Path are left empty so
// findings marshal as {severity, message}, matching exegesis's prior output.
func diag(message string) finding.Diagnostic {
	return finding.Diagnostic{Severity: finding.SeverityError, Message: message}
}

func diagf(format string, a ...any) finding.Diagnostic {
	return diag(fmt.Sprintf(format, a...))
}
