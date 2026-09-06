package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// a2Body renders a skill body whose A2 segment states the given language signals, plus
// an optional related-skills bullet declaring this skill superseded by a merged one.
func a2Body(supersededBy string, signals ...string) string {
	var b strings.Builder
	b.WriteString("# Skill\n\n## A2 — Trigger Scenario\n\n### Language Signals\n\n")
	for _, s := range signals {
		b.WriteString("- \"" + s + "\"\n")
	}
	if supersededBy != "" {
		b.WriteString("\n## Related Skills\n\n- superseded-by: `" +
			supersededBy + "` — merged\n")
	}
	return b.String()
}

// writeA2Skill creates a skill directory whose body carries the given signals.
func writeA2Skill(t *testing.T, tree, slug, supersededBy string, signals ...string) {
	t.Helper()
	dir := filepath.Join(tree, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	doc := "---\nname: " + slug + "\ndescription: Invoke when the situation applies.\n---\n\n" +
		a2Body(supersededBy, signals...)
	writeAt(t, filepath.Join(dir, "SKILL.md"), doc)
	writeAt(t, filepath.Join(dir, "test-prompts.json"), mergedTP)
}

// mergedTreeWith builds a merged tree holding one merged skill with the given signals,
// plus a source tree whose single skill declares itself superseded by it.
func mergedTreeWith(
	t *testing.T,
	mergedSignals, sourceSignals []string,
) (mergedTree, sourceTree string) {
	t.Helper()
	mergedTree, sourceTree = t.TempDir(), t.TempDir()
	writeAt(t, filepath.Join(mergedTree, "MERGE_OVERVIEW.md"), "# Merge Overview\n")
	writeA2Skill(t, mergedTree, "target", "", mergedSignals...)
	writeA2Skill(t, sourceTree, "src", "merged/run/target", sourceSignals...)
	return mergedTree, sourceTree
}

// TestVerifySourceBookReportsAddedSignals is the advisory case: the merged skill states
// signals its source does not, and verify says how many over how many sources.
func TestVerifySourceBookReportsAddedSignals(t *testing.T) {
	t.Parallel()
	merged, source := mergedTreeWith(t,
		[]string{"the budget is exhausted again", "nobody agreed to this freeze policy"},
		[]string{"we need five nines of availability"})

	out, err := run(t, "verify", "--merge", "--gates", "skills",
		"--source-book", source, merged)
	if err != nil {
		t.Fatalf("verify: %v\n%s", err, out)
	}
	if !strings.Contains(out, "target: a2: 2 new signal(s) over 1 source(s)") {
		t.Errorf("want the added-signal count reported, got:\n%s", out)
	}
}

// TestVerifyStrictFailsAThinMerge is the gate: under --strict a merged skill that only
// restates its source's signals fails the tree.
func TestVerifyStrictFailsAThinMerge(t *testing.T) {
	t.Parallel()
	shared := []string{"we need five nines of availability"}
	merged, source := mergedTreeWith(t, shared, shared)

	out, err := run(t, "verify", "--merge", "--gates", "skills",
		"--source-book", source, "--strict", merged)
	if err == nil {
		t.Fatalf("verify --strict should fail a merge adding nothing, got:\n%s", out)
	}
	if !strings.Contains(out, "want at least") {
		t.Errorf("want the shortfall named, got:\n%s", out)
	}
}

// TestVerifyStrictPassesASharpMerge is the other half: the same gate must accept a merge
// that does add signals, or it would only ever be able to say no.
func TestVerifyStrictPassesASharpMerge(t *testing.T) {
	t.Parallel()
	merged, source := mergedTreeWith(t,
		[]string{"the budget is exhausted again", "nobody agreed to this freeze policy"},
		[]string{"we need five nines of availability"})

	if out, err := run(t, "verify", "--merge", "--gates", "skills",
		"--source-book", source, "--strict", merged); err != nil {
		t.Fatalf("verify --strict should accept a sharper merge: %v\n%s", err, out)
	}
}

// TestVerifyReportsAMergedSkillWithNoSources pins the fail-open hazard: a merged skill
// nothing declares itself superseded by has no sources to be sharper than, so every
// signal in it would count as new. It must say it did not check rather than pass quietly.
func TestVerifyReportsAMergedSkillWithNoSources(t *testing.T) {
	t.Parallel()
	merged, source := t.TempDir(), t.TempDir()
	writeAt(t, filepath.Join(merged, "MERGE_OVERVIEW.md"), "# Merge Overview\n")
	writeA2Skill(t, merged, "orphan", "", "a signal nobody else states")
	writeA2Skill(t, source, "src", "merged/run/something-else", "an unrelated signal")

	out, err := run(t, "verify", "--merge", "--gates", "skills",
		"--source-book", source, "--strict", merged)
	if err != nil {
		t.Fatalf("an unattributed merged skill must not fail the tree: %v\n%s", err, out)
	}
	if !strings.Contains(out, "orphan: a2: not checked") {
		t.Errorf("want the unchecked skill named, got:\n%s", out)
	}
}

// TestVerifyReportsASourceBookHoldingNoSkills covers the nesting trap: books that keep
// their skills a level down discover nothing, and saying so is what tells a wrong path
// from a book with none built.
func TestVerifyReportsASourceBookHoldingNoSkills(t *testing.T) {
	t.Parallel()
	merged, source := mergedTreeWith(t,
		[]string{"the budget is exhausted again"},
		[]string{"we need five nines of availability"})
	bare := t.TempDir()

	out, err := run(t, "verify", "--merge", "--gates", "skills",
		"--source-book", source+","+bare, merged)
	if err != nil {
		t.Fatalf("verify: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no skills found") {
		t.Errorf("want the empty source tree reported, got:\n%s", out)
	}
}

func TestVerifyAttributionFlagGuards(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	cases := map[string]struct {
		args []string
		want string
	}{
		"--source-book without --merge": {
			args: []string{"verify", "--source-book", tree, tree},
			want: "--source-book applies to --merge",
		},
		"--strict without --source-book": {
			args: []string{"verify", "--merge", "--strict", tree},
			want: "--strict applies to --source-book",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := run(t, tc.args...)
			if err == nil {
				t.Fatalf("want a usage error, got:\n%s", out)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to name %q", err, tc.want)
			}
		})
	}
}
