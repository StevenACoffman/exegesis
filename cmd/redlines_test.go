package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd/root"
)

const (
	riaDesc = "Invoke when a plan looks obviously correct and should be checked in reverse."
	riaFull = "## R\n\nq\n\n## I\n\nm\n\n## A1\n\ne\n\n## A2\n\nw\n\n## E\n\n1 2 3\n\n## B\n\nnot when\n"
	riaNoE  = "## R\n\nq\n\n## I\n\nm\n\n## A1\n\ne\n\n## A2\n\nw\n\n## B\n\nnot when\n" // missing E
	validTP = `{"tests":[
{"id":1,"type":"should_trigger","prompt":"p","expected":"e"},
{"id":2,"type":"should_trigger","prompt":"p","expected":"e"},
{"id":3,"type":"should_trigger","prompt":"p","expected":"e"},
{"id":4,"type":"should_not_trigger","prompt":"p","expected":"e"},
{"id":5,"type":"should_not_trigger","prompt":"p","expected":"e"},
{"id":6,"type":"edge_case","prompt":"p","expected":"e"}]}`
)

// writeSkillMD writes dir/SKILL.md; the frontmatter name is the folder name so
// the base lint's name==folder check passes.
func writeSkillMD(t *testing.T, dir, description, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\nname: " + filepath.Base(dir) +
		"\ndescription: " + description + "\n---\n\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func writeTestPrompts(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(
		filepath.Join(dir, "test-prompts.json"),
		[]byte(validTP),
		0o644,
	); err != nil {
		t.Fatalf("write test-prompts: %v", err)
	}
}

func TestLintCheckRedlinesClean(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "inversion")
	writeSkillMD(t, dir, riaDesc, riaFull)
	writeTestPrompts(t, dir)

	out, err := run(t, "lint", "--check", "redlines", dir)
	if err != nil {
		t.Fatalf("a complete RIA skill should pass: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("expected ok, got:\n%s", out)
	}
}

func TestLintCheckRedlinesDefects(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		desc    string
		body    string
		tests   bool // write test-prompts.json?
		wantSub string
	}{
		"missing RIA segment":  {riaDesc, riaNoE, true, `"E" RIA segment`},
		"missing test-prompts": {riaDesc, riaFull, false, "test-prompts.json is missing"},
		"generic description":  {"A skill about inversion.", riaFull, true, "trigger condition"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), "inversion")
			writeSkillMD(t, dir, tc.desc, tc.body)
			if tc.tests {
				writeTestPrompts(t, dir)
			}
			out, err := run(t, "lint", "--check", "redlines", dir)
			var exit root.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("expected a red-line failure, got %v\n%s", err, out)
			}
			if !strings.Contains(out, tc.wantSub) {
				t.Errorf("expected %q, got:\n%s", tc.wantSub, out)
			}
		})
	}
}

func TestLintRedlinesOptIn(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "inversion")
	writeSkillMD(t, dir, riaDesc, riaNoE) // missing E, no test-prompts

	// Plain lint (no --check) ignores the red lines.
	if out, err := run(t, "lint", dir); err != nil {
		t.Fatalf("plain lint should pass without --check: %v\n%s", err, out)
	}
	// An unknown --check value is an error.
	if _, err := run(t, "lint", "--check", "bogus", dir); err == nil ||
		!strings.Contains(err.Error(), "unknown --check") {
		t.Errorf("expected an unknown --check error, got %v", err)
	}
}

func TestVerifyCheckRedlines(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	dir := filepath.Join(tree, "inversion")
	writeSkillMD(t, dir, riaDesc, riaNoE) // missing E
	writeTestPrompts(t, dir)

	// Default verify ignores the red lines and passes.
	if out, err := run(t, "verify", tree); err != nil {
		t.Fatalf("plain verify should pass: %v\n%s", err, out)
	}
	// verify --check redlines enforces them and fails.
	out, err := run(t, "verify", "--check", "redlines", tree)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected verify --check redlines to fail, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "RIA segment") {
		t.Errorf("expected a RIA-segment finding, got:\n%s", out)
	}
}
