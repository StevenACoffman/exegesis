package agents_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint/agents"
)

func TestCodexOpenAIYAML(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	skillDir := filepath.Join(root, "skills", "demo")
	yamlPath := filepath.Join(skillDir, "agents", "openai.yaml")
	if err := os.MkdirAll(filepath.Dir(yamlPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// interface.display_name wrong type; a tool missing value with unknown type;
	// policy.allow_implicit_invocation wrong type; unknown top-level + interface fields.
	content := `interface:
  display_name: 123
  bogus_field: x
dependencies:
  tools:
    - type: nonsense
      description: no value here
policy:
  allow_implicit_invocation: "yes"
surprise: 1
`
	if err := os.WriteFile(yamlPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	adapters := agents.Select([]string{"codex"}, root)
	if len(adapters) != 1 {
		t.Fatalf("expected codex adapter, got %d", len(adapters))
	}
	skills := []agents.Skill{{DirName: "demo", DirPath: skillDir, SkillMDPath: yamlPath}}
	ids := checkIDs(adapters[0].Check(root, skills))

	for _, want := range []string{
		"3d.openai-yaml.unknown-fields",
		"3d.openai-yaml.interface-unknown",
		"3d.openai-yaml.interface-display_name-type",
		"3d.openai-yaml.tool-unknown-type",
		"3d.openai-yaml.tool-missing-value",
		"3d.openai-yaml.policy-aii-type",
	} {
		if !ids[want] {
			t.Errorf("expected %q to fire", want)
		}
	}
}
