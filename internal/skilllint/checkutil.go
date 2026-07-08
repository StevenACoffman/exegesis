package skilllint

import "strings"

// tokensPerChar approximates cl100k_base: skillscheck counts tokens with tiktoken;
// this port uses ~4 characters per token until an exact tokenizer is wired in, so
// 1e.body.tokens and 4b.reference.large are approximate.
const tokensPerChar = 4

// placeholderPrefixes are the case-insensitive description openings skillscheck
// treats as placeholder text.
func placeholderPrefixes() []string {
	return []string{
		"todo", "fixme", "tbd", "placeholder", "a skill that", "this skill",
		"description goes here", "enter description", "replace this",
	}
}

// isPlaceholder reports whether desc opens with placeholder/template text.
func isPlaceholder(desc string) bool {
	trimmed := strings.TrimSpace(desc)
	if trimmed == "..." {
		return true
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range placeholderPrefixes() {
		if hasWordPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// hasWordPrefix reports whether s begins with prefix followed by a word boundary
// (end of string or a non-word byte), reproducing skillscheck's `\b` anchoring.
func hasWordPrefix(s, prefix string) bool {
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	if len(s) == len(prefix) {
		return true
	}
	return !isWordByte(s[len(prefix)])
}

func isWordByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9', b == '_':
		return true
	default:
		return false
	}
}

// isKnownTool reports whether an allowed-tools entry names a recognized tool. The
// part before any "(" is matched; "mcp__" prefixes are always accepted.
func isKnownTool(name string) bool {
	base := name
	if i := strings.IndexByte(name, '('); i >= 0 {
		base = name[:i]
	}
	if strings.HasPrefix(base, "mcp__") {
		return true
	}
	switch base {
	case "Read", "Write", "Edit", "Bash", "Glob", "Grep", "Agent", "WebFetch",
		"WebSearch", "Skill", "NotebookEdit", "LSP", "AskUserQuestion", "TaskCreate",
		"TaskGet", "TaskList", "TaskOutput", "TaskStop", "TaskUpdate", "CronCreate",
		"CronDelete", "CronList", "EnterPlanMode", "ExitPlanMode", "EnterWorktree",
		"ExitWorktree", "TodoRead", "TodoWrite", "ToolSearch", "computer",
		"text_editor", "bash":
		return true
	default:
		return false
	}
}

// approxTokens estimates the token count of text (see tokensPerChar).
func approxTokens(text string) int {
	return len(text) / tokensPerChar
}
