package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd/root"
)

// buildLinkedTree writes two skills under a fresh tree and links beta -> alpha
// (beta depends-on alpha), returning the tree path.
func buildLinkedTree(t *testing.T) string {
	t.Helper()
	tree := t.TempDir()
	writeSkill(t, filepath.Join(tree, "alpha"))
	writeSkill(t, filepath.Join(tree, "beta"))
	if _, err := run(t, "link", "--kind", "depends-on", "--to", "alpha",
		"--rationale", "learn the base", filepath.Join(tree, "beta")); err != nil {
		t.Fatalf("link: %v", err)
	}
	return tree
}

func TestIndexGeneratesAndChecksClean(t *testing.T) {
	t.Parallel()
	tree := buildLinkedTree(t)

	if _, err := run(t, "index", tree); err != nil {
		t.Fatalf("index: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(tree, "INDEX.md"))
	if err != nil {
		t.Fatalf("read INDEX.md: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"## Skills", "**alpha**", "**beta**",
		"```mermaid", "beta -->|depends-on| alpha",
		"## Learning path", "1. alpha", "2. beta",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("INDEX.md missing %q:\n%s", want, text)
		}
	}

	// Freshly generated -> --check must pass without rewriting.
	out, err := run(t, "index", "--check", tree)
	if err != nil {
		t.Fatalf("index --check on fresh tree returned error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "up to date") {
		t.Errorf("expected 'up to date', got:\n%s", out)
	}
}

func TestIndexCheckDetectsStale(t *testing.T) {
	t.Parallel()
	tree := buildLinkedTree(t)
	if _, err := run(t, "index", tree); err != nil {
		t.Fatalf("index: %v", err)
	}
	// Add another edge so the on-disk INDEX.md no longer matches.
	if _, err := run(t, "link", "--kind", "composes-with", "--to", "alpha",
		"--rationale", "used together", filepath.Join(tree, "beta")); err != nil {
		t.Fatalf("link: %v", err)
	}

	out, err := run(t, "index", "--check", tree)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected a stale ExitError, got %v", err)
	}
	if !strings.Contains(out, "stale") {
		t.Errorf("expected 'stale' in output, got:\n%s", out)
	}
}

func TestIndexPreservesHandAddedTail(t *testing.T) {
	t.Parallel()
	tree := buildLinkedTree(t)
	path := filepath.Join(tree, "INDEX.md")
	if _, err := run(t, "index", tree); err != nil {
		t.Fatalf("index: %v", err)
	}
	// Append a hand-written section below the generated block.
	generated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	tail := "\n## Notes\n\nkeep these notes\n"
	if err := os.WriteFile(path, append(generated, []byte(tail)...), 0o644); err != nil {
		t.Fatalf("append notes: %v", err)
	}

	if _, err := run(t, "index", tree); err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after regen: %v", err)
	}
	if !strings.Contains(string(body), "keep these notes") {
		t.Errorf("hand-added tail was not preserved:\n%s", body)
	}
}
