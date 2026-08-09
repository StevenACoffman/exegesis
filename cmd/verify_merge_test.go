package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeAt writes content to path, failing the test on error.
func writeAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestVerifyMergeAcceptsMergedComposition is the measured bug: default verify validates
// test-prompts under the standard composition, so a merged skill's
// prefer_merged_over_source cases are rejected as an unknown type. --merge gates them
// under the merged composition instead.
func TestVerifyMergeAcceptsMergedComposition(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	skillDir := filepath.Join(tree, "skilla")
	writeSkill(t, skillDir)
	writeAt(t, filepath.Join(skillDir, "test-prompts.json"), mergedTP)

	// Without --merge: the merged category is an unknown type, so the skills gate fails.
	if out, err := run(t, "verify", "--gates", "skills", tree); err == nil ||
		!strings.Contains(out, "prefer_merged_over_source") {
		t.Fatalf("plain verify should reject the merged category; err=%v\n%s", err, out)
	}
	// With --merge: the merged composition accepts it.
	if out, err := run(t, "verify", "--merge", "--gates", "skills", tree); err != nil {
		t.Fatalf("verify --merge should accept a merged set: %v\n%s", err, out)
	}
}

// TestVerifyMergeRequiresOverview checks that a merged tree must carry MERGE_OVERVIEW.md.
func TestVerifyMergeRequiresOverview(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	skillDir := filepath.Join(tree, "skilla")
	writeSkill(t, skillDir)
	writeAt(t, filepath.Join(skillDir, "test-prompts.json"), mergedTP)
	// No MERGE_OVERVIEW.md at the tree root.

	out, err := run(t, "verify", "--merge", tree)
	if err == nil || !strings.Contains(out, "MERGE_OVERVIEW.md: not found") {
		t.Fatalf("verify --merge without MERGE_OVERVIEW.md should fail; err=%v\n%s", err, out)
	}
}
