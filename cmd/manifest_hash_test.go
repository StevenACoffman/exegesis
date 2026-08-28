package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/skillet/identity"
	"github.com/StevenACoffman/skillet/manifest"
)

// readManifest verifies tree into a manifest at path and parses it back.
//
// A failing gate is not a failing helper: verify writes the manifest whatever the
// verdict, and a tree with no test-prompts.json is one of the states these tests are
// about. Only an error that is not the gate's own exit code aborts.
func readManifest(t *testing.T, tree, path string) manifest.Manifest {
	t.Helper()
	out, err := run(t, "verify", "--manifest", path, tree)
	var exit root.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("verify: %v\n%s", err, out)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	m, err := manifest.Parse(b)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}

// TestManifestSeesTestPromptsChange is the property the hash exists for: a skill whose
// prompts were rewritten and whose prose was not must show up in a diff of the two
// manifests. Without the hash, skillet's axes() compares "" against "" and reports the
// prompts unchanged whatever they say.
func TestManifestSeesTestPromptsChange(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	skillDir := filepath.Join(tree, "skilla")
	writeSkill(t, skillDir)
	if out, err := run(t, "tests", "--scaffold", skillDir); err != nil {
		t.Fatalf("scaffold: %v\n%s", err, out)
	}
	prompts := filepath.Join(skillDir, "test-prompts.json")

	base := readManifest(t, tree, filepath.Join(tree, "base.json"))

	// Rewrite only the prompts. SKILL.md is untouched, so the skill hash is unchanged
	// and the prompts axis is the only thing that can report this edit.
	// The edit stays inside a prompt's text, so the file remains valid and this test
	// measures the manifest rather than re-measuring the composition gate.
	before := readFileString(t, prompts)
	after := strings.Replace(before, "a prompt that SHOULD activate", "a sharper prompt for", 1)
	if after == before {
		t.Fatalf("fixture did not change; scaffolded prompts were:\n%s", before)
	}
	if err := os.WriteFile(prompts, []byte(after), 0o644); err != nil {
		t.Fatalf("write prompts: %v", err)
	}

	cur := readManifest(t, tree, filepath.Join(tree, "cur.json"))

	delta := manifest.Diff(base, cur)
	if len(delta.Changed) != 1 || delta.Changed[0] != "skilla" {
		t.Fatalf("expected skilla to be the one changed location, got %+v", delta)
	}
	axes := delta.ChangedAxes["skilla"]
	if !axes.TestPrompts {
		t.Errorf("test-prompts change is invisible to Diff: axes=%+v", axes)
	}
	if axes.Skill {
		t.Errorf("SKILL.md was not touched, so the skill axis must be false: axes=%+v", axes)
	}
}

// TestManifestHashIsTheSharedIdentity pins the cross-tool contract rather than mere
// presence: skillsaw and exegesis must compute the same 16 characters for the same
// bytes, or a manifest written by one tool cannot be compared with one written by the
// other.
func TestManifestHashIsTheSharedIdentity(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	skillDir := filepath.Join(tree, "skilla")
	writeSkill(t, skillDir)
	if out, err := run(t, "tests", "--scaffold", skillDir); err != nil {
		t.Fatalf("scaffold: %v\n%s", err, out)
	}

	m := readManifest(t, tree, filepath.Join(tree, "skills-manifest.json"))
	if len(m.Skills) != 1 {
		t.Fatalf("expected one skill in the manifest, got %d", len(m.Skills))
	}
	entry := m.Skills[0]
	want := identity.Hash(readFileString(t, filepath.Join(skillDir, "test-prompts.json")))
	if entry.TestPromptsHash != want {
		t.Errorf("test_prompts_sha256 = %q, want %q", entry.TestPromptsHash, want)
	}
	if entry.TestPrompts == "" {
		t.Error("a skill with prompts must still record their path")
	}
}

// TestManifestRecordsNoPromptsAsAbsent keeps empty meaning absent rather than unknown:
// Diff reads an empty hash on both sides as "neither has prompts", which is only correct
// when the file genuinely is not there.
func TestManifestRecordsNoPromptsAsAbsent(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeSkill(t, filepath.Join(tree, "skilla")) // no test-prompts.json

	m := readManifest(t, tree, filepath.Join(tree, "skills-manifest.json"))
	if len(m.Skills) != 1 {
		t.Fatalf("expected one skill in the manifest, got %d", len(m.Skills))
	}
	if entry := m.Skills[0]; entry.TestPrompts != "" || entry.TestPromptsHash != "" {
		t.Errorf("expected both prompt fields empty, got %+v", entry)
	}
}
