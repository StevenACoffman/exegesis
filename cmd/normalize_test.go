package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd/root"
)

// legacySkill uses the bullet dialect that dominates the real book trees: a bold
// kind and a markdown-linked target.
const legacySkill = `---
name: skilla
description: Invoke when the user needs a demo thing done in a particular way.
---
# Body

Nothing runtime-bound here.

## Related skills (Stage 3 Filling)

- **composes-with** → [` + "`skillb`" + `](../skillb/SKILL.md): they are used together
- contrasts-with: (an idea that is not a skill)
`

func TestNormalizeRewritesLegacySectionsAndIsIdempotent(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	dirA := filepath.Join(tree, "skilla")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(dirA, "SKILL.md"), []byte(legacySkill), 0o644,
	); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeNamedSkill(t, tree, "skillb")

	// --check reports the legacy skill and exits non-zero.
	out, err := run(t, "normalize", "--check", tree)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected root.ExitError from --check, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "not canonical") {
		t.Errorf("expected --check to report a non-canonical skill, got:\n%s", out)
	}

	if out, err = run(t, "normalize", tree); err != nil {
		t.Fatalf("normalize: %v\n%s", err, out)
	}
	got := readFileString(t, filepath.Join(dirA, "SKILL.md"))
	if !strings.Contains(got, "- composes-with: `skillb` — they are used together") {
		t.Errorf("expected a canonical bullet, got:\n%s", got)
	}
	if !strings.Contains(got, "## Related Skills\n") ||
		strings.Contains(got, "Stage 3 Filling") {
		t.Errorf("expected the heading to be canonicalised, got:\n%s", got)
	}
	// The bullet naming no skill must survive the rewrite.
	if !strings.Contains(got, "- contrasts-with: (an idea that is not a skill)") {
		t.Errorf("prose bullet was not preserved, got:\n%s", got)
	}

	// A second run is a no-op and --check now passes.
	if out, err = run(t, "normalize", "--check", tree); err != nil {
		t.Fatalf("--check should pass after normalizing: %v\n%s", err, out)
	}
	if !strings.Contains(out, "already canonical") {
		t.Errorf("expected an all-canonical report, got:\n%s", out)
	}
}
