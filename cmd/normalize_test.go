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

// TestNormalizeEmptyTreeIsNotClean is the measured defect: a run that examined nothing
// printed "all 0 skill(s) already canonical" and exited 0, which is the same sentence and
// the same exit code as a run that examined everything and found it well.
//
// It matters more since the corpus went fully canonical, because that made the good outcome
// and the misconfiguration print identically.
func TestNormalizeEmptyTreeIsNotClean(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()

	out, err := run(t, "normalize", "--check", tree)
	if err == nil {
		t.Fatalf("--check over a tree holding no skills should not pass, got:\n%s", out)
	}
	if strings.Contains(out, "already canonical") {
		t.Errorf("an unexamined tree must not be reported as canonical, got:\n%s", out)
	}
	if !strings.Contains(out, "no skills found under "+tree) {
		t.Errorf("want the path named so a reader can see where it looked, got:\n%s", out)
	}
	// The discovery rule is the trap -- one level down, root excluded -- so naming it is
	// what turns the report into a diagnosis.
	if !strings.Contains(out, "immediate subdirectory") {
		t.Errorf("want the discovery rule stated, got:\n%s", out)
	}
}

// TestNormalizeEmptyTreeWriteModeDoesNotFail pins the asymmetry. The write mode is not a
// gate: a sweep over several trees legitimately meets empty ones, and failing that sweep
// would break a working script to fix a gate bug.
func TestNormalizeEmptyTreeWriteModeDoesNotFail(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()

	out, err := run(t, "normalize", tree)
	if err != nil {
		t.Fatalf("the write mode must not fail on an empty tree: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no skills found under "+tree) {
		t.Errorf("want the empty tree reported even when it does not fail, got:\n%s", out)
	}
}

// TestNormalizeCanonicalTreeStillPasses is the other half: the fix must not have made a
// genuinely clean tree fail, or it would have traded a silent false pass for a loud false
// failure on the case that actually matters.
func TestNormalizeCanonicalTreeStillPasses(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeNamedSkill(t, tree, "skilla")
	writeNamedSkill(t, tree, "skillb")
	if out, err := run(t, "normalize", tree); err != nil {
		t.Fatalf("normalize: %v\n%s", err, out)
	}

	out, err := run(t, "normalize", "--check", tree)
	if err != nil {
		t.Fatalf("a canonical tree must still pass: %v\n%s", err, out)
	}
	if !strings.Contains(out, "already canonical") {
		t.Errorf("want the clean verdict on a populated canonical tree, got:\n%s", out)
	}
}
