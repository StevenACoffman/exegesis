package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config-backed adapter types, grouped so declarations precede their methods.
type (
	claude struct{} // .claude-plugin/plugin.json + marketplace.json (3a)
	gemini struct{} // gemini-extension.json (3b)
	codex  struct{} // per-skill agents/openai.yaml (3d)
)

// ---- Claude (3a) -----------------------------------------------------------

func (claude) Name() string      { return "claude" }
func (claude) SourceURL() string { return "https://code.claude.com/docs/en/skills" }
func (claude) Detect(root string) bool {
	return dirExists(filepath.Join(root, ".claude-plugin"))
}

func (claude) KnownFields() []string {
	return []string{
		"disable-model-invocation", "user-invocable", "argument-hint",
		"model", "context", "agent", "hooks",
	}
}

func (c claude) Check(root string, _ []Skill) []Diagnostic {
	pluginDir := filepath.Join(root, ".claude-plugin")
	var diags []Diagnostic
	diags = append(diags, c.checkPluginJSON(filepath.Join(pluginDir, "plugin.json"))...)
	diags = append(
		diags,
		c.checkMarketplaceJSON(root, filepath.Join(pluginDir, "marketplace.json"))...)
	diags = append(diags, c.checkConsistency(pluginDir)...)
	return diags
}

func (c claude) checkPluginJSON(path string) []Diagnostic {
	data, diags := loadJSONObject(path, "3a.plugin-json", "plugin.json", c.SourceURL())
	if data == nil {
		return diags
	}
	for _, f := range []string{"name", "version", "description"} {
		if _, ok := data[f]; !ok {
			diags = append(
				diags,
				c.warn("3a.plugin-json."+f, "plugin.json missing '"+f+"' field", path),
			)
		}
	}
	return diags
}

func (c claude) checkMarketplaceJSON(root, path string) []Diagnostic {
	data, diags := loadJSONObject(path, "3a.marketplace-json", "marketplace.json", c.SourceURL())
	if data == nil {
		return diags
	}
	if _, ok := data["name"]; !ok {
		diags = append(
			diags,
			c.warn("3a.marketplace-json.name", "marketplace.json missing 'name' field", path),
		)
	}
	plugins, ok := data["plugins"].([]any)
	if !ok {
		return diags
	}
	for i, p := range plugins {
		diags = append(diags, c.checkPlugin(root, path, i, p)...)
	}
	return diags
}

func (c claude) checkPlugin(root, path string, i int, p any) []Diagnostic {
	pm, ok := p.(map[string]any)
	if !ok {
		return []Diagnostic{c.err("3a.marketplace-json.plugin-type",
			"plugins["+strconv.Itoa(i)+"] must be an object", path)}
	}
	src, ok := pm["source"].(string)
	if !ok {
		return []Diagnostic{c.warn("3a.marketplace-json.plugin-source",
			"plugins["+strconv.Itoa(i)+"] missing or non-string 'source'", path)}
	}
	if !dirExists(filepath.Join(root, src)) {
		return []Diagnostic{c.err("3a.marketplace-json.plugin-source-missing",
			"plugins["+strconv.Itoa(i)+"] source '"+src+"' does not resolve to a directory", path)}
	}
	return nil
}

func (c claude) checkConsistency(pluginDir string) []Diagnostic {
	plugin := readJSONMeta(filepath.Join(pluginDir, "plugin.json"))
	market := readJSONMeta(filepath.Join(pluginDir, "marketplace.json"))
	if plugin == nil || market == nil {
		return nil
	}
	var diags []Diagnostic
	if mismatch(plugin["name"], market["name"]) {
		diags = append(diags, c.warn("3a.consistency.name",
			"name mismatch between plugin.json and marketplace.json", pluginDir))
	}
	if mismatch(plugin["version"], market["version"]) {
		diags = append(diags, c.warn("3a.consistency.version",
			"version mismatch between plugin.json and marketplace.json", pluginDir))
	}
	return diags
}

func (c claude) warn(check, msg, path string) Diagnostic {
	return Diagnostic{
		Level:     LevelWarning,
		Check:     check,
		Message:   msg,
		Path:      path,
		SourceURL: c.SourceURL(),
	}
}

func (c claude) err(check, msg, path string) Diagnostic {
	return Diagnostic{
		Level:     LevelError,
		Check:     check,
		Message:   msg,
		Path:      path,
		SourceURL: c.SourceURL(),
	}
}

// ---- Gemini (3b) -----------------------------------------------------------

func (gemini) Name() string          { return "gemini" }
func (gemini) SourceURL() string     { return "https://geminicli.com/docs/cli/skills/" }
func (gemini) KnownFields() []string { return nil }
func (gemini) Detect(root string) bool {
	return fileExists(filepath.Join(root, "gemini-extension.json"))
}

func (g gemini) Check(root string, _ []Skill) []Diagnostic {
	path := filepath.Join(root, "gemini-extension.json")
	data, diags := loadJSONObject(path, "3b.gemini-ext", "gemini-extension.json", g.SourceURL())
	if data == nil {
		return diags
	}
	for _, f := range []string{"name", "version", "description"} {
		if _, ok := data[f]; !ok {
			diags = append(diags, Diagnostic{
				Level:     LevelWarning,
				Check:     "3b.gemini-ext." + f,
				Message:   "gemini-extension.json missing '" + f + "' field",
				Path:      path,
				SourceURL: g.SourceURL(),
			})
		}
	}
	if ctx, ok := data["contextFileName"].(string); ok && ctx != "" {
		if !fileExists(filepath.Join(root, ctx)) {
			diags = append(diags, Diagnostic{
				Level:     LevelError,
				Check:     "3b.gemini-ext.context-missing",
				Message:   "contextFileName '" + ctx + "' does not exist",
				Path:      path,
				SourceURL: g.SourceURL(),
			})
		}
	} else if !fileExists(filepath.Join(root, "GEMINI.md")) {
		diags = append(diags, Diagnostic{
			Level: LevelInfo, Check: "3b.gemini-ext.no-context",
			Message: "no contextFileName and no GEMINI.md — Gemini will get no root context",
			Path:    path, SourceURL: g.SourceURL(),
		})
	}
	return diags
}

// ---- Codex (3d) ------------------------------------------------------------

func (codex) Name() string          { return "codex" }
func (codex) SourceURL() string     { return "https://github.com/openai/codex" }
func (codex) KnownFields() []string { return nil }
func (codex) Detect(root string) bool {
	return dirExists(filepath.Join(root, ".codex")) || dirExists(filepath.Join(root, ".agents"))
}

func (c codex) Check(_ string, skills []Skill) []Diagnostic {
	var diags []Diagnostic
	for _, s := range skills {
		path := filepath.Join(s.DirPath, "agents", "openai.yaml")
		if !fileExists(path) {
			continue
		}
		diags = append(diags, c.checkYAML(path)...)
	}
	return diags
}

func (c codex) checkYAML(path string) []Diagnostic {
	var doc any
	if err := yaml.Unmarshal([]byte(readFileOrEmpty(path)), &doc); err != nil {
		return []Diagnostic{c.yaml(LevelError, "invalid", "invalid YAML: "+err.Error(), path)}
	}
	top, ok := doc.(map[string]any)
	if !ok {
		return []Diagnostic{c.yaml(LevelError, "type", "openai.yaml must be a YAML mapping", path)}
	}
	var diags []Diagnostic
	if unknown := unknownKeys(
		top,
		"interface",
		"dependencies",
		"policy",
		"permissions",
	); len(
		unknown,
	) > 0 {
		diags = append(diags, c.yaml(LevelInfo, "unknown-fields",
			"unrecognized top-level fields: "+strings.Join(unknown, ", "), path))
	}
	diags = append(diags, c.checkInterface(top["interface"], path)...)
	diags = append(diags, c.checkDependencies(top["dependencies"], path)...)
	diags = append(diags, c.checkPolicy(top["policy"], path)...)
	diags = append(diags, c.checkPermissions(top["permissions"], path)...)
	return diags
}

func (c codex) checkInterface(v any, path string) []Diagnostic {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return []Diagnostic{
			c.yaml(LevelError, "interface-type", "'interface' must be a mapping", path),
		}
	}
	fields := []string{
		"display_name", "short_description", "icon_small",
		"icon_large", "brand_color", "default_prompt",
	}
	var diags []Diagnostic
	if unknown := unknownKeys(m, fields...); len(unknown) > 0 {
		diags = append(diags, c.yaml(LevelInfo, "interface-unknown",
			"unrecognized interface fields: "+strings.Join(unknown, ", "), path))
	}
	for _, f := range fields {
		if val, present := m[f]; present && val != nil && !isString(val) {
			diags = append(diags, c.yaml(LevelError, "interface-"+f+"-type",
				"interface."+f+" must be a string", path))
		}
	}
	return diags
}

func (c codex) checkDependencies(v any, path string) []Diagnostic {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return []Diagnostic{
			c.yaml(LevelError, "dependencies-type", "'dependencies' must be a mapping", path),
		}
	}
	toolsVal, present := m["tools"]
	if !present {
		return nil
	}
	tools, ok := toolsVal.([]any)
	if !ok {
		return []Diagnostic{c.yaml(LevelError, "dependencies-tools-type",
			"'dependencies.tools' must be a list", path)}
	}
	var diags []Diagnostic
	for i, t := range tools {
		diags = append(diags, c.checkTool(i, t, path)...)
	}
	return diags
}

func (c codex) checkTool(i int, t any, path string) []Diagnostic {
	tm, ok := t.(map[string]any)
	if !ok {
		return []Diagnostic{c.yaml(LevelError, "tool-type",
			"dependencies.tools["+strconv.Itoa(i)+"] must be a mapping", path)}
	}
	idx := strconv.Itoa(i)
	var diags []Diagnostic
	if typ, has := tm["type"]; !has {
		diags = append(diags, c.yaml(LevelWarning, "tool-missing-type",
			"dependencies.tools["+idx+"] missing 'type' field", path))
	} else if s, _ := typ.(string); !isToolType(s) {
		diags = append(diags, c.yaml(LevelWarning, "tool-unknown-type",
			"dependencies.tools["+idx+"] has unknown type (expected: cli, env_var, mcp)", path))
	}
	if _, has := tm["value"]; !has {
		diags = append(diags, c.yaml(LevelWarning, "tool-missing-value",
			"dependencies.tools["+idx+"] missing 'value' field", path))
	}
	if unknown := unknownKeys(
		tm,
		"type",
		"value",
		"description",
		"transport",
		"command",
		"url",
	); len(
		unknown,
	) > 0 {
		diags = append(diags, c.yaml(
			LevelInfo,
			"tool-unknown-fields",
			"dependencies.tools["+idx+"] has unrecognized fields: "+strings.Join(
				unknown,
				", ",
			),
			path,
		))
	}
	return diags
}

func (c codex) checkPolicy(v any, path string) []Diagnostic {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return []Diagnostic{c.yaml(LevelError, "policy-type", "'policy' must be a mapping", path)}
	}
	if aii, present := m["allow_implicit_invocation"]; present && aii != nil && !isBool(aii) {
		return []Diagnostic{c.yaml(LevelError, "policy-aii-type",
			"policy.allow_implicit_invocation must be a boolean", path)}
	}
	return nil
}

func (c codex) checkPermissions(v any, path string) []Diagnostic {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return []Diagnostic{
			c.yaml(LevelError, "permissions-type", "'permissions' must be a mapping", path),
		}
	}
	var diags []Diagnostic
	if unknown := unknownKeys(m, "network", "file_system", "macos"); len(unknown) > 0 {
		diags = append(diags, c.yaml(LevelInfo, "permissions-unknown",
			"unrecognized permissions fields: "+strings.Join(unknown, ", "), path))
	}
	diags = append(diags, c.mustMap(m, "network", "permissions-network-type",
		"permissions.network must be a mapping", path)...)
	diags = append(diags, c.mustMap(m, "macos", "permissions-macos-type",
		"permissions.macos must be a mapping", path)...)
	diags = append(diags, c.checkFileSystem(m["file_system"], path)...)
	return diags
}

func (c codex) checkFileSystem(v any, path string) []Diagnostic {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return []Diagnostic{c.yaml(LevelError, "permissions-fs-type",
			"permissions.file_system must be a mapping", path)}
	}
	var diags []Diagnostic
	for _, key := range []string{"read", "write"} {
		if val, present := m[key]; present {
			if _, isList := val.([]any); !isList {
				diags = append(diags, c.yaml(LevelError, "permissions-fs-"+key+"-type",
					"permissions.file_system."+key+" must be a list", path))
			}
		}
	}
	return diags
}

func (c codex) mustMap(m map[string]any, key, suffix, msg, path string) []Diagnostic {
	if val, present := m[key]; present && val != nil {
		if _, ok := val.(map[string]any); !ok {
			return []Diagnostic{c.yaml(LevelError, suffix, msg, path)}
		}
	}
	return nil
}

func (c codex) yaml(level, suffix, msg, path string) Diagnostic {
	return Diagnostic{
		Level: level, Check: "3d.openai-yaml." + suffix,
		Message: msg, Path: path, SourceURL: c.SourceURL(),
	}
}

func isToolType(s string) bool {
	switch s {
	case "cli", "env_var", "mcp":
		return true
	default:
		return false
	}
}

// ---- Cross-agent (3c) ------------------------------------------------------

// CrossAgent flags name/version/description mismatches across agent config files
// (currently plugin.json and gemini-extension.json). It runs only when at least
// two adapters are active.
func CrossAgent(root string, active []Adapter) []Diagnostic {
	if len(active) < 2 {
		return nil
	}
	configs := map[string]map[string]any{
		"plugin.json":           readJSONMeta(filepath.Join(root, ".claude-plugin", "plugin.json")),
		"gemini-extension.json": readJSONMeta(filepath.Join(root, "gemini-extension.json")),
	}
	var diags []Diagnostic
	for _, field := range []string{"name", "version", "description"} {
		if d, ok := crossField(root, configs, field); ok {
			diags = append(diags, d)
		}
	}
	return diags
}

func crossField(root string, configs map[string]map[string]any, field string) (Diagnostic, bool) {
	values := make(map[string]bool)
	var detail []string
	for _, label := range sortedKeys(configs) {
		cfg := configs[label]
		if cfg == nil {
			continue
		}
		if v, ok := cfg[field].(string); ok {
			values[v] = true
			detail = append(detail, label+"="+strconv.Quote(v))
		}
	}
	if len(values) < 2 {
		return Diagnostic{}, false
	}
	return Diagnostic{
		Level:   LevelWarning,
		Check:   "3c." + field + "-mismatch",
		Message: field + " mismatch across agent configs: " + strings.Join(detail, ", "),
		Path:    root,
	}, true
}

// ---- shared config helpers -------------------------------------------------

func loadJSONObject(path, prefix, label, sourceURL string) (map[string]any, []Diagnostic) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []Diagnostic{{
			Level: LevelError, Check: prefix + ".missing",
			Message: label + " not found", Path: path, SourceURL: sourceURL,
		}}
	}
	var v any
	if jerr := json.Unmarshal(data, &v); jerr != nil {
		return nil, []Diagnostic{{
			Level: LevelError, Check: prefix + ".invalid",
			Message: "invalid JSON: " + jerr.Error(), Path: path, SourceURL: sourceURL,
		}}
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, []Diagnostic{{
			Level: LevelError, Check: prefix + ".type",
			Message: label + " must be a JSON object", Path: path, SourceURL: sourceURL,
		}}
	}
	return m, nil
}

func readJSONMeta(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var v any
	if json.Unmarshal(data, &v) != nil {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}

func readFileOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func mismatch(a, b any) bool {
	as, aok := a.(string)
	bs, bok := b.(string)
	return aok && bok && as != "" && bs != "" && as != bs
}

func unknownKeys(m map[string]any, allowed ...string) []string {
	allow := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allow[a] = true
	}
	var unknown []string
	for k := range m {
		if !allow[k] {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	return unknown
}

func sortedKeys(m map[string]map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
