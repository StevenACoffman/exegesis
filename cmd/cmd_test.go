package cmd_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/exegesis/cmd"
	"github.com/StevenACoffman/exegesis/cmd/root"
)

const validSkill = `---
name: skilla
description: Invoke when the user needs a demo thing done in a particular way.
---
# Body
Nothing runtime-bound here.
`

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := cmd.Run(context.Background(), args, strings.NewReader(""), &out, &out)
	return out.String(), err
}

func writeSkill(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(validSkill), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestScaffoldThenVerifyPasses(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	skillDir := filepath.Join(tree, "skilla")
	writeSkill(t, skillDir)

	if _, err := run(t, "tests", "--scaffold", skillDir); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	out, err := run(t, "verify", tree)
	if err != nil {
		t.Fatalf("verify returned error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "skilla: ok") {
		t.Errorf("verify output missing 'skilla: ok':\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(tree, "skills-manifest.json")); statErr != nil {
		t.Errorf("expected skills-manifest.json to be written: %v", statErr)
	}
}

func TestLintReportsDefectWithExitError(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	// name != folder ("skilla" vs folder "wrongname") is an error-severity finding.
	skillDir := filepath.Join(tree, "wrongname")
	writeSkill(t, skillDir)

	out, err := run(t, "lint", skillDir)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected root.ExitError, got %v", err)
	}
	if !strings.Contains(out, "!= folder") {
		t.Errorf("expected a name/folder finding, got:\n%s", out)
	}
}

func TestUnknownSubcommandIsAnError(t *testing.T) {
	t.Parallel()
	out, err := run(t, "definitely-not-a-command")
	if err == nil {
		t.Fatal("expected an error for an unknown subcommand")
	}
	if errors.Is(err, ff.ErrNoExec) {
		t.Errorf("unknown subcommand must not be ErrNoExec (that path exits 0): %v", err)
	}
	if !strings.Contains(err.Error(), "unknown subcommand") {
		t.Errorf("error should name the problem, got %v", err)
	}
	if !strings.Contains(out, "unknown subcommand") && !strings.Contains(out, "SUBCOMMANDS") {
		t.Errorf("expected usage/help on stderr, got:\n%s", out)
	}
}

func TestBareInvocationStaysErrNoExec(t *testing.T) {
	t.Parallel()
	// No args is a genuine bare invocation: ErrNoExec, which main.go maps to
	// exit 0. It must not be treated as an unknown subcommand.
	_, err := run(t)
	if !errors.Is(err, ff.ErrNoExec) {
		t.Fatalf("bare invocation should return ff.ErrNoExec, got %v", err)
	}
}

func TestVerifyWithRegistryHashAndCatalog(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	skillDir := filepath.Join(tree, "skilla")
	writeSkill(t, skillDir)
	if _, err := run(t, "tests", "--scaffold", skillDir); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	reg := filepath.Join(tree, "registry.json")
	if err := os.WriteFile(
		reg,
		[]byte(`{"expected_skills":["skilla"],"max_body_words":100}`),
		0o644,
	); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	out, err := run(t, "verify", "--registry", reg, tree)
	if err != nil {
		t.Fatalf("verify with registry returned error: %v\n%s", err, out)
	}
	b, readErr := os.ReadFile(filepath.Join(tree, "skills-manifest.json"))
	if readErr != nil {
		t.Fatalf("read manifest: %v", readErr)
	}
	if !strings.Contains(string(b), "\"sha256\"") {
		t.Errorf("manifest should carry a per-skill sha256 hash:\n%s", b)
	}
}

func TestVerifyRegistryCatalogMismatchFails(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	skillDir := filepath.Join(tree, "skilla")
	writeSkill(t, skillDir)
	if _, err := run(t, "tests", "--scaffold", skillDir); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	reg := filepath.Join(tree, "registry.json")
	// Expect a skill that is not present -> catalog failure.
	if err := os.WriteFile(
		reg,
		[]byte(`{"expected_skills":["skilla","missing-one"]}`),
		0o644,
	); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	out, err := run(t, "verify", "--registry", reg, tree)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected catalog mismatch to fail, got %v", err)
	}
	if !strings.Contains(out, "missing-one") {
		t.Errorf("expected a catalog problem naming the missing skill, got:\n%s", out)
	}
}

func TestVerifyFailsWithoutTestPrompts(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeSkill(t, filepath.Join(tree, "skilla")) // no test-prompts.json

	out, err := run(t, "verify", tree)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected verify to fail with ExitError, got %v", err)
	}
	if !strings.Contains(out, "structure_verified=false") {
		t.Errorf("expected manifest to record structure_verified=false, got:\n%s", out)
	}
}

func writeSchema(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "candidates.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	return path
}

func TestScaffoldCommandWritesGatedTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	schema := writeSchema(t, dir, `{"skills":[
	  {"slug":"widget-maker","description":"Use this when the user asks to build or assemble a widget from parts."},
	  {"slug":"widget-parts","description":"Use this when the user needs to look up available widget parts and specs."}
	]}`)
	out := filepath.Join(dir, "out")
	stdout, err := run(t, "scaffold", "--schema", schema, "--output-dir", out)
	if err != nil {
		t.Fatalf("scaffold: %v\n%s", err, stdout)
	}
	for _, slug := range []string{"widget-maker", "widget-parts"} {
		for _, name := range []string{"SKILL.md", "test-prompts.json"} {
			if _, statErr := os.Stat(filepath.Join(out, slug, name)); statErr != nil {
				t.Errorf("expected %s/%s to be written: %v", slug, name, statErr)
			}
		}
	}
	// Closed loop: the emitted tree passes the same gate on its own.
	if vout, verr := run(t, "verify", "--check", "redlines", out); verr != nil {
		t.Errorf("verify on the scaffolded tree failed: %v\n%s", verr, vout)
	}
}

func TestScaffoldCommandRemovesFailingSkill(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Angle brackets in the description violate speclint's plain-text rule.
	schema := writeSchema(t, dir,
		`{"skills":[{"slug":"bad-skill","description":"Use this when a <tag> appears."}]}`)
	out := filepath.Join(dir, "out")
	_, err := run(t, "scaffold", "--schema", schema, "--output-dir", out)
	var exit root.ExitError
	if !errors.As(err, &exit) || int(exit) != 1 {
		t.Fatalf("got %v, want root.ExitError(1)", err)
	}
	if _, statErr := os.Stat(filepath.Join(out, "bad-skill")); !os.IsNotExist(statErr) {
		t.Error("the failing skill dir should have been removed")
	}
}

func TestScaffoldCommandRequiresSchema(t *testing.T) {
	t.Parallel()
	if _, err := run(t, "scaffold", "--output-dir", t.TempDir()); err == nil ||
		!strings.Contains(err.Error(), "--schema is required") {
		t.Errorf("got %v, want a missing --schema error", err)
	}
}
