// Package agents implements the per-agent compatibility checks (skillscheck 3*):
// eight adapters plus cross-agent consistency. It defines its own neutral
// Diagnostic/Skill value types so the parent skilllint package can depend on it
// without an import cycle; skilllint converts the results into its own model.
package agents

import (
	"io/fs"
	"os"
	"strings"
)

// Levels for agent diagnostics (plain strings; skilllint maps them to its Level).
const (
	LevelError   = "error"
	LevelWarning = "warning"
	LevelInfo    = "info"
)

// Diagnostic is a neutral finding record produced by an adapter.
type Diagnostic struct {
	Level     string
	Check     string
	Message   string
	Path      string
	SourceURL string
}

// Skill is the subset of a parsed skill an adapter needs.
type Skill struct {
	DirName     string
	DirPath     string
	SkillMDPath string
	Frontmatter map[string]any
	Body        string
}

// Adapter validates one agent platform's conventions.
type Adapter interface {
	Name() string
	SourceURL() string
	Detect(root string) bool
	Check(root string, skills []Skill) []Diagnostic
	// KnownFields returns the frontmatter keys this agent recognizes, so the base
	// spec's unknown-field check does not flag them.
	KnownFields() []string
}

// fieldRule describes an optional frontmatter field and how to validate it.
type fieldRule struct {
	name  string
	label string
	valid func(any) bool
}

// All returns every adapter in registry order.
func All() []Adapter {
	return []Adapter{
		claude{}, codex{}, copilot{}, cursor{}, gemini{}, roo{}, swival{}, windsurf{},
	}
}

// Select returns the adapters to run. A nil names slice (or {"all"}) auto-detects
// via Detect; otherwise adapters are matched by name (unknown names are dropped).
func Select(names []string, root string) []Adapter {
	if len(names) == 0 || (len(names) == 1 && names[0] == "all") {
		var out []Adapter
		for _, a := range All() {
			if a.Detect(root) {
				out = append(out, a)
			}
		}
		return out
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[strings.TrimSpace(n)] = true
	}
	var out []Adapter
	for _, a := range All() {
		if want[a.Name()] {
			out = append(out, a)
		}
	}
	return out
}

// KnownFields returns the union of the given adapters' recognized frontmatter
// keys.
func KnownFields(adapters []Adapter) map[string]bool {
	fields := make(map[string]bool)
	for _, a := range adapters {
		for _, f := range a.KnownFields() {
			fields[f] = true
		}
	}
	return fields
}

func dirExists(path string) bool {
	info, err := statPath(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := statPath(path)
	return err == nil && !info.IsDir()
}

// underPrefix reports whether the skill's directory sits under any of the given
// absolute-path prefixes (each already ending with the intended separator/marker).
func underPrefix(skill Skill, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(skill.DirPath, p) {
			return true
		}
	}
	return false
}

// checkFieldTypes validates optional frontmatter fields' types. Absent fields are
// OK; present fields that fail wantString/wantBool/wantStringList produce a
// {prefix}.frontmatter.{field}-type error.
func checkFieldTypes(skill Skill, prefix, sourceURL string, fields []fieldRule) []Diagnostic {
	var diags []Diagnostic
	for _, rule := range fields {
		v, ok := skill.Frontmatter[rule.name]
		if !ok || v == nil {
			continue
		}
		if !rule.valid(v) {
			diags = append(diags, Diagnostic{
				Level:     LevelError,
				Check:     prefix + ".frontmatter." + rule.name + "-type",
				Message:   "'" + rule.name + "' must be " + rule.label + ", got " + typeName(v),
				Path:      skill.SkillMDPath,
				SourceURL: sourceURL,
			})
		}
	}
	return diags
}

func isString(v any) bool { _, ok := v.(string); return ok }
func isBool(v any) bool   { _, ok := v.(bool); return ok }

func isStringList(v any) bool {
	list, ok := v.([]any)
	if !ok {
		return false
	}
	for _, item := range list {
		if _, isStr := item.(string); !isStr {
			return false
		}
	}
	return true
}

func typeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case string:
		return "string"
	case int, int64:
		return "int"
	case float64:
		return "float"
	case []any:
		return "list"
	case map[string]any:
		return "mapping"
	default:
		return "value"
	}
}

func statPath(path string) (fs.FileInfo, error) {
	return os.Stat(path) //nolint:wrapcheck // internal helper; callers only check existence
}
