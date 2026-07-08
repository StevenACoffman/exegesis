package skilllint

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const (
	redlineAllowedKeysMsg = "allowed: name, description, tags, allowed-tools, author, version"
	maxDescRunes          = 1024
)

// CheckRedlines enforces the book2skill Quality Red Lines on a parsed skill.
// It returns nothing when the skill failed to parse — the spec category reports
// that via 1a.*.
func CheckRedlines(s *Skill) []Diagnostic {
	if s.ParseError != "" {
		return nil
	}
	var diags []Diagnostic
	diags = append(diags, redlineAllowedKeys(s)...)
	diags = append(diags, redlineNameDirMatch(s)...)
	diags = append(diags, redlineDescription(s)...)
	diags = append(diags, redlineSegments(s)...)
	diags = append(diags, redlineQuote(s)...)
	diags = append(diags, redlineRelated(s)...)
	diags = append(diags, redlineTestPrompts(s)...)
	return diags
}

func redlineAllowedKeys(s *Skill) []Diagnostic {
	var diags []Diagnostic
	for _, key := range s.FrontmatterKeys {
		if !isRedlineAllowedKey(key) {
			diags = append(diags, Diagnostic{
				Level:   LevelError,
				Check:   "rl.frontmatter.allowed-keys",
				Message: "frontmatter key '" + key + "' is not allowed (" + redlineAllowedKeysMsg + ")",
				Path:    s.SkillMDPath,
			})
		}
	}
	return diags
}

func redlineNameDirMatch(s *Skill) []Diagnostic {
	name, ok := stringField(s.Frontmatter, "name")
	if !ok || name == s.DirName {
		return nil
	}
	return []Diagnostic{{
		Level:   LevelError,
		Check:   "rl.name.dir-match",
		Message: "name '" + name + "' does not match directory name '" + s.DirName + "'",
		Path:    s.SkillMDPath,
	}}
}

func redlineDescription(s *Skill) []Diagnostic {
	desc, ok := stringField(s.Frontmatter, "description")
	if !ok {
		return nil
	}
	var diags []Diagnostic
	if n := utf8.RuneCountInString(desc); n > maxDescRunes {
		diags = append(diags, Diagnostic{
			Level:   LevelError,
			Check:   "rl.description.length",
			Message: "description is " + strconv.Itoa(n) + " runes (max 1024)",
			Path:    s.SkillMDPath,
		})
	}
	if xmlTagRE().MatchString(desc) {
		diags = append(diags, Diagnostic{
			Level:   LevelError,
			Check:   "rl.description.plaintext",
			Message: "description must be plain text (contains an angle-bracket/XML tag)",
			Path:    s.SkillMDPath,
		})
	}
	return diags
}

func redlineSegments(s *Skill) []Diagnostic {
	segments := book2skill.ParseSegments(s.Body)
	var diags []Diagnostic
	for _, tag := range book2skill.SegmentTags() {
		if _, ok := segments[tag]; !ok {
			diags = append(diags, Diagnostic{
				Level:   LevelError,
				Check:   "rl.segments.present",
				Message: "missing RIA++ segment '" + tag + "'",
				Path:    s.SkillMDPath,
			})
		}
	}
	return diags
}

func redlineQuote(s *Skill) []Diagnostic {
	segments := book2skill.ParseSegments(s.Body)
	reading, ok := segments[book2skill.SegR]
	if !ok {
		return nil
	}
	quote := extractQuote(reading)
	if quote == "" {
		return nil
	}
	if err := book2skill.ValidateQuote(quote, book2skill.QuoteMaxRunes(quote)); err != nil {
		return []Diagnostic{{
			Level:   LevelError,
			Check:   "rl.quote.length",
			Message: book2skill.ErrorMessage(err),
			Path:    s.SkillMDPath,
		}}
	}
	return nil
}

func redlineRelated(s *Skill) []Diagnostic {
	var diags []Diagnostic
	inRelated := false
	for _, line := range strings.Split(s.Body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inRelated = strings.Contains(strings.ToLower(trimmed), "related")
			continue
		}
		if inRelated && relatedLinkRE().MatchString(line) {
			diags = append(diags, Diagnostic{
				Level:   LevelError,
				Check:   "rl.related.slug-form",
				Message: "related skills must be backticked slugs, not '../…/SKILL.md' links",
				Path:    s.SkillMDPath,
			})
		}
	}
	return diags
}

func redlineTestPrompts(s *Skill) []Diagnostic {
	if fileExists(filepath.Join(s.DirPath, "test-prompts.json")) {
		return nil
	}
	return []Diagnostic{{
		Level:   LevelError,
		Check:   "rl.test-prompts.present",
		Message: "no test-prompts.json in the skill directory",
		Path:    s.DirPath,
	}}
}

func isRedlineAllowedKey(key string) bool {
	switch key {
	case "name", "description", "tags", "allowed-tools", "author", "version":
		return true
	default:
		return false
	}
}

// extractQuote pulls the quoted text out of a rendered R segment: blockquote
// lines with the leading marker removed, stopping at the attribution line
// (which starts with an em dash).
func extractQuote(reading string) string {
	var quoted []string
	for _, line := range strings.Split(reading, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, ">") {
			continue
		}
		content := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
		if strings.HasPrefix(content, "—") {
			break
		}
		if content != "" {
			quoted = append(quoted, content)
		}
	}
	return strings.Join(quoted, " ")
}

// xmlTagRE matches an HTML/XML-style tag such as <b>, </p>, or <a href="x">.
func xmlTagRE() *regexp.Regexp {
	return regexp.MustCompile(`</?[A-Za-z][^>]*>`)
}

// relatedLinkRE matches a markdown link whose target escapes the skill directory
// (contains ../ or points at a SKILL.md).
func relatedLinkRE() *regexp.Regexp {
	return regexp.MustCompile(`\]\([^)]*(\.\./|SKILL\.md)[^)]*\)`)
}
