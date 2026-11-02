package lint_test

import (
	"testing"

	"github.com/StevenACoffman/exegesis/internal/lint"
)

func TestIsDevArtifact(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"test-results.md":                   true,
		"merge-audit.md":                    true,
		"TODO.md":                           true,
		"GITHUB_REPO.md":                    true,
		"unconventional-commits-workspace":  true,
		"x-workspace/iteration-1/inputs.md": true,
		"test-prompts.json":                 false, // the test contract ships
		"references/todo.md":                false,
		"methodology/00-overview.md":        false,
		"workspace-notes.md":                false,
	}
	for p, want := range cases {
		t.Run(p, func(t *testing.T) {
			t.Parallel()
			if got := lint.IsDevArtifact(p); got != want {
				t.Errorf("IsDevArtifact(%q) = %v, want %v", p, got, want)
			}
		})
	}
}
