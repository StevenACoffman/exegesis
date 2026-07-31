// Package skill loads and parses Agent Skills ("SKILL.md") files for the
// exegesis gates.
//
// A skill is a directory containing a SKILL.md in the Anthropic Agent Skills
// format: YAML frontmatter (name + description, optionally tags/allowed-tools)
// delimited by "---" lines, followed by a Markdown body. Parsing is pure once
// the bytes are read; only Load and Discover touch the filesystem.
package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// Skill is a parsed SKILL.md and its location.
type Skill struct {
	Dir             string   // skill directory
	Path            string   // <Dir>/SKILL.md
	Name            string   // frontmatter name
	Description     string   // frontmatter description
	FrontmatterKeys []string // top-level frontmatter keys, sorted (for lint)
	Frontmatter     string   // raw YAML frontmatter block (between the --- lines)
	Body            string   // markdown body after the frontmatter
	Raw             string   // full file contents
	Bytes           int      // byte size of Raw
}

// Load reads and parses <dir>/SKILL.md.
func Load(dir string) (*Skill, error) {
	p := filepath.Join(dir, "SKILL.md")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("load skill %s: %w", dir, err)
	}
	s := &Skill{Dir: dir, Path: p, Raw: string(b), Bytes: len(b)}
	s.parse()
	return s, nil
}

// Discover returns every immediate subdirectory of tree that contains a
// SKILL.md, sorted by name. It is the skill-tree walk the gates iterate over.
func Discover(tree string) ([]string, error) {
	entries, err := os.ReadDir(tree)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", tree, err)
	}
	dirs := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(tree, e.Name())
		if _, statErr := os.Stat(filepath.Join(dir, "SKILL.md")); statErr == nil {
			dirs = append(dirs, dir)
		}
	}
	return dirs, nil
}

// Hash returns the first 16 hex chars of sha256(content) — byte-identical to
// skillsaw's skill.Hash, so a skill has the same identity in both tools and a
// hash-pinned manifest can be cross-checked against skillsaw.
func Hash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])[:16]
}

func (s *Skill) parse() {
	text := strings.ReplaceAll(s.Raw, "\r\n", "\n")
	s.Frontmatter, s.Body = splitFrontmatter(text)

	// Ordered/unordered does not matter for the allowlist check; a map suffices.
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(s.Frontmatter), &fields); err != nil {
		return // malformed frontmatter leaves Name/Description empty; lint flags it
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s.FrontmatterKeys = keys
	s.Name = strings.TrimSpace(asString(fields["name"]))
	s.Description = strings.TrimSpace(asString(fields["description"]))
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// splitFrontmatter separates a leading "---"-delimited YAML block from the body.
// It returns ("", text) when there is no frontmatter and ("", rest) when the
// opening delimiter has no matching close.
func splitFrontmatter(text string) (frontmatter, body string) {
	if !strings.HasPrefix(text, "---\n") {
		return "", text
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", rest
	}
	after := rest[end+len("\n---"):]
	if nl := strings.IndexByte(after, '\n'); nl >= 0 {
		body = after[nl+1:]
	}
	return rest[:end], body
}
