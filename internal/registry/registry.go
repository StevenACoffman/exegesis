// Package registry loads an optional exegesis registry file: the catalog of
// expected skills plus the token budgets and required sections a tree enforces.
// It is the deterministic, config-driven half of the budget/section lint adopted
// from cc-thinking-skills' registry + validate-skills.js. Load does I/O; the zero
// Registry enforces nothing (all limits opt-in).
package registry

import (
	"encoding/json"
	"fmt"
	"os"
)

// Registry is the parsed registry file. Every field is optional; a zero value
// enforces no budget and no required sections.
type Registry struct {
	ExpectedSkills      []string `json:"expected_skills,omitempty"`
	MaxBodyWords        int      `json:"max_body_words,omitempty"`
	MaxDescriptionWords int      `json:"max_description_words,omitempty"`
	RequiredSections    []string `json:"required_sections,omitempty"`
}

// Load reads and parses a registry JSON file.
func Load(path string) (*Registry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("registry: read %s: %w", path, err)
	}
	var r Registry
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("registry: parse %s: %w", path, err)
	}
	return &r, nil
}
