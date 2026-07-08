package agents

import (
	"path/filepath"
	"unicode/utf8"
)

const (
	swivalMaxDesc = 1024
	swivalMaxBody = 20000
)

// Adapter types are grouped so their declarations precede their methods, which
// satisfies the const/var/type/func ordering lint.
type (
	copilot  struct{} // .github/skills/ frontmatter (3e)
	cursor   struct{} // .cursor or .agents/skills frontmatter (3f)
	windsurf struct{} // deprecated .windsurfrules (3g)
	roo      struct{} // .roo/skills frontmatter + rule-file deprecations (3h)
	swival   struct{} // description/body length limits (3i)
)

func (copilot) Name() string { return "copilot" }
func (copilot) SourceURL() string {
	return "https://code.visualstudio.com/docs/copilot/chat/chat-agent-skills"
}

func (copilot) Detect(root string) bool {
	return dirExists(filepath.Join(root, ".github", "skills"))
}

func (copilot) KnownFields() []string {
	return []string{"user-invocable", "disable-model-invocation", "argument-hint"}
}

func (c copilot) Check(root string, skills []Skill) []Diagnostic {
	prefix := filepath.Join(root, ".github", "skills") + string(filepath.Separator)
	rules := []fieldRule{
		{"user-invocable", "a boolean", isBool},
		{"disable-model-invocation", "a boolean", isBool},
		{"argument-hint", "a string", isString},
	}
	var diags []Diagnostic
	for _, s := range skills {
		if underPrefix(s, prefix) {
			diags = append(diags, checkFieldTypes(s, "3e", c.SourceURL(), rules)...)
		}
	}
	return diags
}

func (cursor) Name() string          { return "cursor" }
func (cursor) SourceURL() string     { return "https://docs.cursor.com/context/rules" }
func (cursor) KnownFields() []string { return []string{"disable-model-invocation"} }
func (cursor) Detect(root string) bool {
	return dirExists(filepath.Join(root, ".cursor")) ||
		dirExists(filepath.Join(root, ".agents", "skills"))
}

func (c cursor) Check(root string, skills []Skill) []Diagnostic {
	sep := string(filepath.Separator)
	prefixes := []string{
		filepath.Join(root, ".cursor", "skills") + sep,
		filepath.Join(root, ".agents", "skills") + sep,
	}
	rules := []fieldRule{{"disable-model-invocation", "a boolean", isBool}}
	var diags []Diagnostic
	for _, s := range skills {
		if underPrefix(s, prefixes...) {
			diags = append(diags, checkFieldTypes(s, "3f", c.SourceURL(), rules)...)
		}
	}
	diags = append(diags, deprecated(root, ".cursorrules",
		"3f.cursorrules-deprecated", ".cursorrules is deprecated; migrate to .cursor/rules/")...)
	return diags
}

func (windsurf) Name() string { return "windsurf" }

func (windsurf) SourceURL() string     { return "https://docs.windsurf.com/windsurf/cascade/skills" }
func (windsurf) KnownFields() []string { return nil }
func (windsurf) Detect(root string) bool {
	return dirExists(filepath.Join(root, ".windsurf")) ||
		dirExists(filepath.Join(root, ".agents", "skills")) ||
		fileExists(filepath.Join(root, ".windsurfrules"))
}

func (windsurf) Check(root string, _ []Skill) []Diagnostic {
	return deprecated(root, ".windsurfrules",
		"3g.windsurfrules-deprecated", ".windsurfrules is deprecated; migrate to .windsurf/rules/")
}

func (roo) Name() string          { return "roo" }
func (roo) SourceURL() string     { return "https://docs.roocode.com/features/skills" }
func (roo) KnownFields() []string { return []string{"modeSlugs", "mode"} }
func (roo) Detect(root string) bool {
	return dirExists(filepath.Join(root, ".roo")) ||
		fileExists(filepath.Join(root, ".roomodes")) ||
		fileExists(filepath.Join(root, ".roorules"))
}

func (r roo) Check(root string, skills []Skill) []Diagnostic {
	sep := string(filepath.Separator)
	rooSkills := filepath.Join(root, ".roo", "skills")
	agentSkills := filepath.Join(root, ".agents", "skills")
	prefixes := []string{rooSkills + sep, rooSkills + "-", agentSkills + sep, agentSkills + "-"}

	var diags []Diagnostic
	for _, s := range skills {
		if underPrefix(s, prefixes...) {
			diags = append(diags, r.frontmatter(s)...)
		}
	}
	diags = append(diags, deprecated(root, ".roorules",
		"3h.roorules-deprecated", ".roorules is deprecated; migrate to .roo/rules/")...)
	diags = append(diags, deprecated(root, ".clinerules",
		"3h.clinerules-deprecated", ".clinerules is deprecated; migrate to .roo/rules/")...)
	return diags
}

func (r roo) frontmatter(s Skill) []Diagnostic {
	rules := []fieldRule{
		{"modeSlugs", "a list of strings", isStringList},
		{"mode", "a string", isString},
	}
	diags := checkFieldTypes(s, "3h", r.SourceURL(), rules)
	if mode, ok := s.Frontmatter["mode"]; ok {
		if _, isStr := mode.(string); isStr {
			diags = append(diags, Diagnostic{
				Level: LevelInfo, Check: "3h.frontmatter.mode-deprecated",
				Message: "'mode' is deprecated; use 'modeSlugs' instead",
				Path:    s.SkillMDPath, SourceURL: r.SourceURL(),
			})
		}
	}
	return diags
}

func (swival) Name() string          { return "swival" }
func (swival) SourceURL() string     { return "https://github.com/swival/swival" }
func (swival) KnownFields() []string { return nil }
func (swival) Detect(root string) bool {
	return fileExists(filepath.Join(root, "swival.toml")) ||
		dirExists(filepath.Join(root, ".swival"))
}

func (s swival) Check(_ string, skills []Skill) []Diagnostic {
	var diags []Diagnostic
	for _, sk := range skills {
		if desc, ok := sk.Frontmatter["description"].(string); ok &&
			utf8.RuneCountInString(desc) > swivalMaxDesc {
			diags = append(diags, Diagnostic{
				Level: LevelWarning, Check: "3i.description-length",
				Message: "Swival truncates descriptions over 1024 chars",
				Path:    sk.SkillMDPath, SourceURL: s.SourceURL(),
			})
		}
		if utf8.RuneCountInString(sk.Body) > swivalMaxBody {
			diags = append(diags, Diagnostic{
				Level: LevelWarning, Check: "3i.body-length",
				Message: "Swival truncates skill body over 20000 chars",
				Path:    sk.SkillMDPath, SourceURL: s.SourceURL(),
			})
		}
	}
	return diags
}

// deprecated returns a single warning when a deprecated rule file exists at root.
func deprecated(root, file, check, message string) []Diagnostic {
	path := filepath.Join(root, file)
	if !fileExists(path) {
		return nil
	}
	return []Diagnostic{{Level: LevelWarning, Check: check, Message: message, Path: path}}
}
