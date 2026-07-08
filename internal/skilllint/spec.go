package skilllint

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// SpecURL is attached to spec/quality/disclosure diagnostics.
const SpecURL = "https://agentskills.io/specification"

const (
	maxNameLen          = 64
	maxCompatLen        = 500
	maxBodyLines        = 500
	bodyTokenGuardBytes = 8000
	maxBodyTokens       = 5000
)

// CheckSpec runs the agentskills.io spec-compliance checks (1a–1e) on one skill.
// extensionFields are the frontmatter keys contributed by active agent adapters.
// An optional TokenCounter overrides the default approximation for 1e.body.tokens.
func CheckSpec(s *Skill, extensionFields map[string]bool, count ...TokenCounter) []Diagnostic {
	if s.ParseError != "" {
		if s.ParseError == "SKILL.md not found" {
			return []Diagnostic{diag(LevelError, "1a.presence", s.ParseError, s.DirPath)}
		}
		return []Diagnostic{diag(LevelError, "1a.frontmatter", s.ParseError, s.SkillMDPath)}
	}
	var diags []Diagnostic
	diags = append(diags, checkName(s)...)
	diags = append(diags, checkDescription(s)...)
	diags = append(diags, checkOptionalFields(s)...)
	diags = append(diags, checkUnknownFields(s, extensionFields)...)
	diags = append(diags, checkBody(s, resolveCounter(count))...)
	return diags
}

// CheckCrossSkill reports duplicate names (error) and identical descriptions
// (warning) across all parsed skills.
func CheckCrossSkill(skills []*Skill) []Diagnostic {
	var diags []Diagnostic
	seenName := make(map[string]string)
	seenDesc := make(map[string]string)
	for _, s := range skills {
		if s.Frontmatter == nil {
			continue
		}
		if name, ok := stringField(s.Frontmatter, "name"); ok {
			if first, dup := seenName[name]; dup {
				diags = append(diags, diag(
					LevelError,
					"1g.duplicate-name",
					"skill name '"+name+"' is used by both "+first+" and "+s.SkillMDPath,
					s.SkillMDPath,
				))
			} else {
				seenName[name] = s.SkillMDPath
			}
		}
		if desc, ok := stringField(s.Frontmatter, "description"); ok {
			key := strings.ToLower(strings.TrimSpace(desc))
			if first, dup := seenDesc[key]; key != "" && dup {
				diags = append(diags, diag(LevelWarning, "1g.duplicate-description",
					"description is identical to skill at "+first, s.SkillMDPath))
			} else if key != "" {
				seenDesc[key] = s.SkillMDPath
			}
		}
	}
	return diags
}

func checkName(s *Skill) []Diagnostic {
	name, ok := stringField(s.Frontmatter, "name")
	if !ok {
		return []Diagnostic{
			diag(LevelError, "1b.name.missing", "required field 'name' is missing", s.SkillMDPath),
		}
	}
	if name == "" {
		return []Diagnostic{
			diag(LevelError, "1b.name.empty", "field 'name' must not be empty", s.SkillMDPath),
		}
	}
	var diags []Diagnostic
	if n := utf8.RuneCountInString(name); n > maxNameLen {
		diags = append(diags, diag(LevelError, "1b.name.length",
			"field 'name' is "+strconv.Itoa(n)+" chars (max 64)", s.SkillMDPath))
	}
	diags = append(diags, checkNameFormat(name, s.SkillMDPath)...)
	if strings.Contains(name, "--") {
		diags = append(diags, fixable(LevelError, "1b.name.consecutive-hyphens",
			"name must not contain consecutive hyphens (--)", s.SkillMDPath))
	}
	if name != s.DirName {
		diags = append(diags, fixable(LevelError, "1b.name.dir-match",
			"name '"+name+"' does not match directory name '"+s.DirName+"'", s.SkillMDPath))
	}
	return diags
}

func checkNameFormat(name, path string) []Diagnostic {
	if nameRE().MatchString(name) {
		return nil
	}
	switch {
	case name[0] == '-':
		return []Diagnostic{
			diag(LevelError, "1b.name.format", "name must not start with a hyphen", path),
		}
	case name[len(name)-1] == '-':
		return []Diagnostic{
			diag(LevelError, "1b.name.format", "name must not end with a hyphen", path),
		}
	case name != strings.ToLower(name):
		return []Diagnostic{fixable(LevelError, "1b.name.format", "name must be lowercase", path)}
	default:
		return []Diagnostic{diag(LevelError, "1b.name.format",
			"name '"+name+"' contains invalid characters (only lowercase a-z, 0-9, hyphens)", path)}
	}
}

func checkDescription(s *Skill) []Diagnostic {
	desc, ok := stringField(s.Frontmatter, "description")
	if !ok {
		return []Diagnostic{
			diag(
				LevelError,
				"1b.description.missing",
				"required field 'description' is missing",
				s.SkillMDPath,
			),
		}
	}
	if desc == "" {
		return []Diagnostic{
			diag(
				LevelError,
				"1b.description.empty",
				"field 'description' must not be empty",
				s.SkillMDPath,
			),
		}
	}
	var diags []Diagnostic
	if n := utf8.RuneCountInString(desc); n > maxDescRunes {
		diags = append(diags, diag(LevelError, "1b.description.length",
			"field 'description' is "+strconv.Itoa(n)+" chars (max 1024)", s.SkillMDPath))
	}
	if isPlaceholder(desc) {
		diags = append(diags, diag(LevelWarning, "1b.description.placeholder",
			"description looks like a placeholder or template text", s.SkillMDPath))
	}
	return diags
}

func checkOptionalFields(s *Skill) []Diagnostic {
	var diags []Diagnostic
	if v, ok := s.Frontmatter["compatibility"]; ok && v != nil {
		if n := utf8.RuneCountInString(toString(v)); n == 0 {
			diags = append(diags, diag(LevelError, "1c.compatibility.empty",
				"field 'compatibility' must not be empty if present", s.SkillMDPath))
		} else if n > maxCompatLen {
			diags = append(diags, diag(LevelError, "1c.compatibility.length",
				"field 'compatibility' is "+strconv.Itoa(n)+" chars (max 500)", s.SkillMDPath))
		}
	}
	if v, ok := s.Frontmatter["metadata"]; ok && v != nil {
		diags = append(diags, checkMetadata(v, s.SkillMDPath)...)
	}
	diags = append(diags, checkAllowedTools(s)...)
	return diags
}

func checkMetadata(v any, path string) []Diagnostic {
	m, ok := v.(map[string]any)
	if !ok {
		return []Diagnostic{
			diag(LevelError, "1c.metadata.type", "field 'metadata' must be a mapping", path),
		}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var diags []Diagnostic
	for _, k := range keys {
		if _, isStr := m[k].(string); !isStr {
			diags = append(diags, diag(LevelError, "1c.metadata.value-type",
				"metadata value for '"+k+"' must be a string", path))
		}
	}
	return diags
}

func checkAllowedTools(s *Skill) []Diagnostic {
	v, ok := s.Frontmatter["allowed-tools"]
	if !ok {
		return nil
	}
	var diags []Diagnostic
	var names []string
	switch val := v.(type) {
	case []any:
		diags = append(diags, diag(LevelInfo, "1c.allowed-tools.list-form",
			"allowed-tools uses list form; accepted for compatibility but not portable "+
				"base-spec syntax (spec defines space-delimited string)", s.SkillMDPath))
		for _, item := range val {
			if str, isStr := item.(string); isStr {
				names = append(names, str)
			} else {
				diags = append(diags, diag(LevelError, "1c.allowed-tools.item-type",
					"allowed-tools list items must be strings", s.SkillMDPath))
			}
		}
	case string:
		names = strings.Fields(val)
	default:
		diags = append(diags, diag(LevelError, "1c.allowed-tools.type",
			"field 'allowed-tools' must be a string or list", s.SkillMDPath))
	}
	for _, name := range names {
		if !isKnownTool(name) {
			diags = append(diags, diag(
				LevelInfo,
				"1c.allowed-tools.unknown-tool",
				"tool '"+name+"' in allowed-tools is not recognized by any known agent",
				s.SkillMDPath,
			))
		}
	}
	return diags
}

func checkUnknownFields(s *Skill, extensionFields map[string]bool) []Diagnostic {
	var diags []Diagnostic
	for _, key := range s.FrontmatterKeys {
		if isBaseSpecField(key) || extensionFields[key] {
			continue
		}
		diags = append(diags, diag(LevelInfo, "1d.unknown-field",
			"field '"+key+"' is not in the base spec or any active adapter", s.SkillMDPath))
	}
	return diags
}

func checkBody(s *Skill, count TokenCounter) []Diagnostic {
	stripped := strings.TrimSpace(s.Body)
	if stripped == "" {
		return []Diagnostic{diag(LevelWarning, "1e.body.empty",
			"SKILL.md body is empty (no instructions after frontmatter)", s.SkillMDPath)}
	}
	var diags []Diagnostic
	if first := strings.SplitN(stripped, "\n", 2)[0]; !strings.HasPrefix(
		strings.TrimSpace(first),
		"#",
	) {
		diags = append(diags, diag(
			LevelInfo,
			"1e.body.no-heading",
			"SKILL.md body does not start with a heading — consider adding a descriptive heading",
			s.SkillMDPath,
		))
	}
	if lines := strings.Count(stripped, "\n") + 1; lines > maxBodyLines {
		diags = append(diags, diag(
			LevelWarning,
			"1e.body.length",
			"SKILL.md body is "+strconv.Itoa(
				lines,
			)+" lines (spec recommends < 500)",
			s.SkillMDPath,
		))
	}
	if len(stripped) >= bodyTokenGuardBytes {
		if tokens := count(stripped); tokens > maxBodyTokens {
			diags = append(diags, diag(
				LevelWarning,
				"1e.body.tokens",
				"SKILL.md body is ~"+strconv.Itoa(
					tokens,
				)+" tokens (spec recommends < 5000)",
				s.SkillMDPath,
			))
		}
	}
	return diags
}

func diag(level Level, check, message, path string) Diagnostic {
	return Diagnostic{Level: level, Check: check, Message: message, Path: path, SourceURL: SpecURL}
}

func fixable(level Level, check, message, path string) Diagnostic {
	d := diag(level, check, message, path)
	d.Fixable = true
	return d
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func nameRE() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
}

func isBaseSpecField(key string) bool {
	switch key {
	case "name", "description", "license", "compatibility", "metadata", "allowed-tools":
		return true
	default:
		return false
	}
}
