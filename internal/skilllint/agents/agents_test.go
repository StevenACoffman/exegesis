package agents_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint/agents"
)

func checkIDs(diags []agents.Diagnostic) map[string]bool {
	ids := make(map[string]bool)
	for _, d := range diags {
		ids[d.Check] = true
	}
	return ids
}

func TestSelectAutoDetect(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "gemini-extension.json"),
		[]byte(`{}`),
		0o600,
	); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := make(map[string]bool)
	for _, a := range agents.Select(nil, root) {
		got[a.Name()] = true
	}
	if !got["claude"] || !got["gemini"] {
		t.Errorf("auto-detect should find claude and gemini, got %v", got)
	}
	if got["cursor"] {
		t.Error("cursor should not be detected without its markers")
	}
}

func TestClaudeMissingConfigs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	adapters := agents.Select([]string{"claude"}, root)
	if len(adapters) != 1 {
		t.Fatalf("expected claude adapter, got %d", len(adapters))
	}
	ids := checkIDs(adapters[0].Check(root, nil))
	if !ids["3a.plugin-json.missing"] || !ids["3a.marketplace-json.missing"] {
		t.Errorf("expected missing-config diagnostics, got %v", ids)
	}
}

func TestCursorDeprecation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cursor"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cursorrules"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	adapters := agents.Select([]string{"cursor"}, root)
	ids := checkIDs(adapters[0].Check(root, nil))
	if !ids["3f.cursorrules-deprecated"] {
		t.Errorf("expected 3f.cursorrules-deprecated, got %v", ids)
	}
}

func TestCrossAgentMismatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustJSON(t, filepath.Join(root, ".claude-plugin", "plugin.json"), `{"name":"alpha"}`)
	mustJSON(t, filepath.Join(root, "gemini-extension.json"), `{"name":"beta"}`)

	active := agents.Select([]string{"claude", "gemini"}, root)
	ids := checkIDs(agents.CrossAgent(root, active))
	if !ids["3c.name-mismatch"] {
		t.Errorf("expected 3c.name-mismatch, got %v", ids)
	}
}

func mustJSON(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}
