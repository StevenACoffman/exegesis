package cmd_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestUsageOutputOnlyForUsageErrors pins the rule that usage output answers a misuse
// of the command line and nothing else. The cases are one per class of failure rather
// than one per command: the dispatcher makes this decision in a single place, so
// repeating it for all ten commands would measure coverage, not behaviour.
func TestUsageOutputOnlyForUsageErrors(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	goodDir := writeNamedSkill(t, tree, "skilla")
	// writeSkill's frontmatter name is "skilla", so a "wrongname" folder is an
	// error-severity lint finding — a gate failure, reported as an ExitError.
	badDir := filepath.Join(tree, "wrongname")
	writeSkill(t, badDir)

	cases := map[string]struct {
		args      []string
		wantUsage bool
		wantMsg   string
	}{
		"missing required flag": {
			args:      []string{"relate", tree},
			wantUsage: true,
			wantMsg:   "--edges is required",
		},
		"wrong positional count": {
			args:      []string{"verify", tree, "extra-tree"},
			wantUsage: true,
			wantMsg:   "expected at most one tree path",
		},
		"invalid flag value survives wrapping": {
			// parseGates marks this, and exec wraps it with fmt.Errorf("verify: %w").
			// The mark must still be found through the wrap.
			args:      []string{"verify", "--gates", "bogus", tree},
			wantUsage: true,
			wantMsg:   "unknown gate",
		},
		"invalid flag value classified at the internal boundary": {
			// internal/lint returns a plain error; cmd/lint marks it as usage.
			args:      []string{"lint", "--check", "bogus", goodDir},
			wantUsage: true,
			wantMsg:   "unknown --check",
		},
		"unreadable input file is runtime": {
			args:      []string{"relate", "--edges", filepath.Join(tree, "absent.json"), tree},
			wantUsage: false,
			wantMsg:   "read edges",
		},
		"missing tree is runtime": {
			args:      []string{"index", filepath.Join(tree, "no-such-tree")},
			wantUsage: false,
			wantMsg:   "discover skills",
		},
		"gate failure is runtime": {
			args:      []string{"lint", badDir},
			wantUsage: false,
			wantMsg:   "",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := run(t, tc.args...)
			if err == nil {
				t.Fatalf("expected an error, got none\n%s", out)
			}
			if got := strings.Contains(out, "USAGE"); got != tc.wantUsage {
				t.Errorf("usage printed = %v, want %v; output:\n%s", got, tc.wantUsage, out)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error %q should name the problem (%q)", err, tc.wantMsg)
			}
		})
	}
}
