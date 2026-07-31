package registry_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/registry"
)

func TestLoad(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "registry.json")
	content := `{"expected_skills":["a","b"],"max_body_words":500,` +
		`"max_description_words":60,"required_sections":["When NOT to Use"]}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r, err := registry.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(r.ExpectedSkills) != 2 || r.MaxBodyWords != 500 || r.MaxDescriptionWords != 60 {
		t.Errorf("unexpected registry: %+v", r)
	}
	if len(r.RequiredSections) != 1 || r.RequiredSections[0] != "When NOT to Use" {
		t.Errorf("required sections = %v", r.RequiredSections)
	}
}

func TestLoadMissing(t *testing.T) {
	t.Parallel()
	if _, err := registry.Load(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing registry")
	}
}
