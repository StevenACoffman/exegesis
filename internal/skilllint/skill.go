package skilllint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill is a parsed skill directory. Frontmatter is the decoded YAML mapping;
// FrontmatterKeys preserves the source key order (for deterministic
// unknown-key diagnostics). ParseError is non-empty when SKILL.md is missing or
// its frontmatter could not be parsed.
type Skill struct {
	DirName         string
	DirPath         string
	SkillMDPath     string
	Frontmatter     map[string]any
	FrontmatterKeys []string
	Body            string
	BodyLineOffset  int
	ParseError      string
}

// Parse reads and parses the SKILL.md in skillDir.
func Parse(skillDir string) *Skill {
	skillMD := filepath.Join(skillDir, "SKILL.md")
	s := &Skill{
		DirName:     filepath.Base(skillDir),
		DirPath:     skillDir,
		SkillMDPath: skillMD,
	}

	data, err := os.ReadFile(skillMD)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.ParseError = "SKILL.md not found"
		} else {
			s.ParseError = "cannot read SKILL.md: " + err.Error()
		}
		return s
	}

	fm, keys, body, offset, parseErr := splitFrontmatter(string(data))
	if parseErr != "" {
		s.ParseError = parseErr
		return s
	}
	s.Frontmatter = fm
	s.FrontmatterKeys = keys
	s.Body = body
	s.BodyLineOffset = offset
	return s
}

// splitFrontmatter separates YAML frontmatter from the body following
// skillscheck's rules. On failure it returns an error message string; on success
// it returns the mapping, its ordered keys, the body, and the body's 0-based
// starting line index.
func splitFrontmatter(text string) (map[string]any, []string, string, int, string) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, nil, text, 0, "missing opening frontmatter delimiter (---)"
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, nil, text, 0, "missing closing frontmatter delimiter (---)"
	}

	fm, keys, decodeErr := decodeFrontmatter(strings.Join(lines[1:end], "\n"))
	if decodeErr != "" {
		return nil, nil, text, 0, decodeErr
	}
	bodyStart := end + 1
	return fm, keys, strings.Join(lines[bodyStart:], "\n"), bodyStart, ""
}

// decodeFrontmatter decodes fmText into a mapping and its ordered keys. An empty
// or null document decodes to an empty mapping.
func decodeFrontmatter(fmText string) (map[string]any, []string, string) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fmText), &doc); err != nil {
		return nil, nil, "invalid YAML in frontmatter: " + err.Error()
	}
	if len(doc.Content) == 0 {
		return map[string]any{}, nil, ""
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil, "frontmatter must be a YAML mapping"
	}

	fm := make(map[string]any, len(root.Content)/2)
	keys := make([]string, 0, len(root.Content)/2)
	for i := 0; i+1 < len(root.Content); i += 2 {
		var value any
		if err := root.Content[i+1].Decode(&value); err != nil {
			return nil, nil, "invalid YAML in frontmatter: " + err.Error()
		}
		key := root.Content[i].Value
		fm[key] = value
		keys = append(keys, key)
	}
	return fm, keys, ""
}

// stringField returns fm[key] coerced to a string and whether the key was
// present. A non-string present value is stringified, mirroring skillscheck's
// str() coercion.
func stringField(fm map[string]any, key string) (string, bool) {
	v, ok := fm[key]
	if !ok || v == nil {
		return "", ok
	}
	if s, isStr := v.(string); isStr {
		return s, true
	}
	return fmt.Sprintf("%v", v), true
}
