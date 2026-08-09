// Package lint validates a single Agent Skill against the agentskills.io spec
// plus book2skill's Quality Red Lines and the runtime-neutrality gate (E3). Check
// is pure over an already-loaded skill; the caller does the file I/O and decides
// the exit code from the returned diagnostics.
//
// Two families of rule live in skillet so exegesis and skillsaw share one source of
// truth rather than drifting: the agentskills.io frontmatter rules (speclint) and
// the Quality Red Lines (redlines). The checks below are the ones that remain
// exegesis-specific: folder match, body links, runtime neutrality, and the opt-in
// registry budgets.
package lint

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/neutrality"
	"github.com/StevenACoffman/skillet/redlines"
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
	Redlines            bool     // opt-in: enforce book2skill's mechanical Quality Red Lines
	SkillLens           bool     // opt-in: the SkillLens quality tier (skillet/skilllens)
}

// Checks says which opt-in tiers a --check value enables. Each tier is on its own
// schedule — the spec moves when agentskills.io moves, the red lines when book2skill's
// methodology moves, and SkillLens when its rubric moves — so they are separate flags,
// not one.
type Checks struct {
	Redlines  bool
	SkillLens bool
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
	// Only compare the name when there was a name to read. A frontmatter block that
	// failed to parse leaves s.Name empty, and reporting that as a mismatch states a
	// consequence of the YAML error as though it were a second, independent defect —
	// speclint has already named the real one.
	//
	// The body checks below are deliberately still run: splitFrontmatter separates the
	// block before the parse is attempted, so s.Body is intact and its defects are
	// real. Suppressing them would make the author fix the YAML and lint again just to
	// be told what could have been said the first time.
	if s.FrontmatterErr == nil {
		if want := filepath.Base(s.Dir); s.Name != want {
			ds = append(ds, diagf("frontmatter: name %q != folder %q", s.Name, want))
		}
	}
	ds = append(ds, checkBody(s.Body)...)
	ds = append(ds, checkBudget(s, opts)...)
	for _, h := range neutrality.Scan([]neutrality.NamedFile{{Name: "SKILL.md", Content: s.Raw}}) {
		ds = append(ds, diagf("runtime-bound wording at SKILL.md:%d: %q", h.Line, h.Text))
	}
	if opts.Redlines {
		ds = append(ds, redlines.Check(s)...)
	}
	if opts.SkillLens {
		ds = append(ds, skillLens(s)...)
	}
	return ds
}

// ParseCheck maps a --check flag value to the opt-in tiers it enables. An empty value
// enables none; "redlines" and "skilllens" enable one each; "all" enables both;
// anything else errors. It is shared by the lint and verify commands so they agree on
// the flag.
func ParseCheck(value string) (Checks, error) {
	switch strings.TrimSpace(value) {
	case "":
		return Checks{}, nil
	case "redlines":
		return Checks{Redlines: true}, nil
	case "skilllens":
		return Checks{SkillLens: true}, nil
	case "all":
		return Checks{Redlines: true, SkillLens: true}, nil
	default:
		return Checks{}, fmt.Errorf("unknown --check %q (known: redlines, skilllens, all)", value)
	}
}

// checkRedlines returns the mechanical Quality Red Lines for s: the six RIA
// segments must be present (#2), quotations stay within the word limit (#3), and
// the description states a trigger condition (#5). Pure over s.Body and
// s.Description; code fences are ignored. The #4 test-prompts.json presence check
// needs the filesystem, so the caller does it.
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
