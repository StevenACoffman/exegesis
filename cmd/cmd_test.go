package cmd_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd"
	"github.com/StevenACoffman/exegesis/cmd/root"
	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/render"
)

func TestIndexCommandWritesOrderedIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "BOOK_OVERVIEW.md"),
		"# My Book — Book Overview\n\n- **Author:** A. Writer\n")
	// skill-a depends on skill-b, so the learning order must list skill-b first.
	writeFile(t, filepath.Join(dir, "skill-a", "SKILL.md"),
		"# Skill A\n\n## Related skills\n\n- depends-on: `skill-b` — needs it\n")
	writeFile(t, filepath.Join(dir, "skill-b", "SKILL.md"), "# Skill B\n")

	out, err := run(t, "index", dir)
	if err != nil {
		t.Fatalf("index: %v (%s)", err, out)
	}
	got := readFile(t, filepath.Join(dir, "INDEX.md"))
	if !strings.Contains(got, "# My Book — Skill Index") {
		t.Errorf("INDEX.md missing book title header:\n%s", got)
	}
	if !strings.Contains(got, "[`skill-a`]") || !strings.Contains(got, "[`skill-b`]") {
		t.Errorf("INDEX.md missing a skill entry:\n%s", got)
	}
	if idxB, idxA := learningPos(got, "skill-b"), learningPos(got, "skill-a"); idxB >= idxA {
		t.Errorf("learning order: skill-b (%d) should precede skill-a (%d)\n%s", idxB, idxA, got)
	}
}

func TestIndexPreservesHandAuthoredSections(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "skill-a", "SKILL.md"), "# Skill A\n")

	if _, err := run(t, "index", dir); err != nil {
		t.Fatalf("index: %v", err)
	}
	// Hand-add a section, then regenerate.
	path := filepath.Join(dir, "INDEX.md")
	writeFile(t, path, readFile(t, path)+"\n## Notes\n\nHand-authored observation.\n")
	if _, err := run(t, "index", dir); err != nil {
		t.Fatalf("index (regenerate): %v", err)
	}
	got := readFile(t, path)
	if !strings.Contains(got, "## Notes") || !strings.Contains(got, "Hand-authored observation.") {
		t.Errorf("regeneration clobbered the hand-authored section:\n%s", got)
	}
	// --check is not stale after preservation.
	if _, err := run(t, "index", "--check", dir); err != nil {
		t.Errorf("--check should pass with a preserved custom section: %v", err)
	}
}

func TestIndexCheckFailsWhenStale(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "skill-a", "SKILL.md"), "# Skill A\n")

	// No INDEX.md exists yet, so --check must report stale via ExitError(1).
	_, err := run(t, "index", "--check", dir)
	assertExit1(t, err)
}

func TestIndexWarnsOnSparseRelationships(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Four skills, zero relationships — below the heuristic band, so index warns.
	for _, slug := range []string{"a", "b", "c", "d"} {
		writeFile(t, filepath.Join(dir, slug, "SKILL.md"), "# Skill "+slug+"\n")
	}
	var out, errBuf bytes.Buffer
	err := cmd.Run(context.Background(), []string{"index", dir},
		strings.NewReader(""), &out, &errBuf)
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if !strings.Contains(errBuf.String(), "too independent") {
		t.Errorf("expected a sparse-relationship warning on stderr, got: %q", errBuf.String())
	}
}

func TestTestsScaffoldThenValidate(t *testing.T) {
	t.Parallel()
	skillDir := t.TempDir()

	if _, err := run(t, "tests", "--scaffold", skillDir); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	raw := readFile(t, filepath.Join(skillDir, "test-prompts.json"))
	cases, err := book2skill.DecodeTestPrompts([]byte(raw))
	if err != nil {
		t.Fatalf("decode scaffolded file: %v", err)
	}
	if problems := book2skill.ValidateTestSet(cases); len(problems) != 0 {
		t.Fatalf("scaffolded set should pass the gate, got %v", problems)
	}

	out, err := run(t, "tests", skillDir)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !strings.Contains(out, "gate: PASS") {
		t.Errorf("expected PASS, got:\n%s", out)
	}
}

func TestTestsValidateFailsWhenUndersized(t *testing.T) {
	t.Parallel()
	skillDir := t.TempDir()
	// Only one should_trigger case — fails all three thresholds but decode is fine.
	writeFile(t, filepath.Join(skillDir, "test-prompts.json"),
		`[{"id":1,"type":"should_trigger","prompt":"p","expected":"e"}]`)

	out, err := run(t, "tests", skillDir)
	assertExit1(t, err)
	if !strings.Contains(out, "gate: FAIL") {
		t.Errorf("expected FAIL, got:\n%s", out)
	}
}

func TestTestsMergeScaffoldThenValidate(t *testing.T) {
	t.Parallel()
	skillDir := t.TempDir()
	if _, err := run(t, "tests", "--merge", "--scaffold", skillDir); err != nil {
		t.Fatalf("scaffold --merge: %v", err)
	}
	out, err := run(t, "tests", "--merge", skillDir)
	if err != nil {
		t.Fatalf("validate --merge: %v", err)
	}
	if !strings.Contains(out, "gate: PASS") || !strings.Contains(out, "prefer_merged_over_source") {
		t.Errorf("expected merge PASS with prefer_merged count, got:\n%s", out)
	}
}

func TestTestsMergeRejectsThreeCategorySet(t *testing.T) {
	t.Parallel()
	skillDir := t.TempDir()
	// A book2skill-shaped set (no prefer_merged_over_source) passes the plain gate
	// but must fail under --merge.
	if _, err := run(t, "tests", "--scaffold", skillDir); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	out, err := run(t, "tests", "--merge", skillDir)
	assertExit1(t, err)
	if !strings.Contains(out, "gate: FAIL") {
		t.Errorf("expected merge FAIL, got:\n%s", out)
	}
}

func TestTestsMigrateWrapperToCanonical(t *testing.T) {
	t.Parallel()
	skillDir := t.TempDir()
	// Object-wrapper shape with expected_behavior and string ids, sized to pass.
	writeFile(t, filepath.Join(skillDir, "test-prompts.json"), `{"skill":"x","version":"1",
		"test_cases":[
			{"id":"st-1","type":"should_trigger","prompt":"a","expected_behavior":"fires"},
			{"id":"st-2","type":"should_trigger","prompt":"b","expected_behavior":"fires"},
			{"id":"st-3","type":"should_trigger","prompt":"c","expected_behavior":"fires"},
			{"id":"snt-1","type":"should_not_trigger","prompt":"d","expected_behavior":"quiet"},
			{"id":"snt-2","type":"should_not_trigger","prompt":"e","expected_behavior":"quiet"},
			{"id":"ec-1","type":"edge_case","prompt":"f","expected_behavior":"either"}]}`)

	stdout, _, err := runCapture(t, "tests", "--migrate", skillDir)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !strings.Contains(stdout, "migrated") || !strings.Contains(stdout, "gate: PASS") {
		t.Errorf("expected migrated + PASS, got:\n%s", stdout)
	}
	// The rewritten file must now decode with the strict parser, with int ids.
	cases, decErr := book2skill.DecodeTestPrompts(
		[]byte(readFile(t, filepath.Join(skillDir, "test-prompts.json"))))
	if decErr != nil {
		t.Fatalf("migrated file must decode: %v", decErr)
	}
	if len(cases) != 6 || cases[0].ID != 1 || cases[5].ID != 6 {
		t.Errorf("want 6 cases renumbered 1..6, got %d (first=%d last=%d)",
			len(cases), cases[0].ID, cases[len(cases)-1].ID)
	}
}

func TestTestsMigrateReportsMissingExpected(t *testing.T) {
	t.Parallel()
	skillDir := t.TempDir()
	writeFile(t, filepath.Join(skillDir, "test-prompts.json"),
		`{"prompts":[{"id":"a","type":"should_invoke","prompt":"p","rationale":"why"}]}`)
	_, stderr, _ := runCapture(t, "tests", "--migrate", skillDir)
	if !strings.Contains(stderr, "no expected") {
		t.Errorf("expected a 'no expected' warning on stderr, got:\n%s", stderr)
	}
}

func TestMergeStatusAppendThenCheck(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	skill := filepath.Join(dir, "inversion-thinking")
	writeFile(t, filepath.Join(skill, "SKILL.md"), "# Inversion Thinking\n\nbody\n")

	if _, err := run(t, "merge-status", "append", "--run", "run-1",
		"--state", "merged", "--into", "merged-decisions", skill); err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, err := run(t, "merge-status", "check", dir); err != nil {
		t.Fatalf("check should pass after a valid append: %v", err)
	}
	// An invalid state is rejected at append time.
	if _, err := run(
		t,
		"merge-status",
		"append",
		"--run",
		"r",
		"--state",
		"bogus",
		skill,
	); err == nil {
		t.Error("append with an unknown state should fail")
	}
}

func TestMergeStatusAppendLink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	skill := filepath.Join(dir, "inversion")
	writeFile(t, filepath.Join(skill, "SKILL.md"), "# Inversion\n\nbody\n")

	if _, err := run(t, "merge-status", "append", "--link", "--run", "r1",
		"--state", "merged", "--into", "combined", skill); err != nil {
		t.Fatalf("append --link: %v", err)
	}
	got := readFile(t, filepath.Join(skill, "SKILL.md"))
	if !strings.Contains(got, "## Merge Status") || !strings.Contains(got, "state: merged") {
		t.Errorf("ledger entry missing:\n%s", got)
	}
	if !strings.Contains(got, "## Related Skills") ||
		!strings.Contains(got, "superseded-by: `combined`") {
		t.Errorf("superseded-by link missing:\n%s", got)
	}
}

func TestMergeStatusCheckFlagsBadLedger(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A hand-written ledger with a merged entry missing `into`.
	writeFile(t, filepath.Join(dir, "s", "SKILL.md"),
		"# S\n\n## Merge Status\n\n```yaml\n- run: r1\n  state: merged\n```\n")
	out, err := run(t, "merge-status", "check", dir)
	assertExit1(t, err)
	if !strings.Contains(out, "requires into") {
		t.Errorf("expected an 'into' problem, got:\n%s", out)
	}
}

func TestQuoteCheckFindsAndFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	skill := filepath.Join(dir, "inversion")
	writeFile(t, filepath.Join(skill, "SKILL.md"),
		"# Inversion\n\n## R — Reading\n\n> Invert, always invert.\n>\n> — Jacobi\n\n## I\n\nx\n")
	src := filepath.Join(dir, "source.txt")

	// Quote present → pass.
	writeFile(t, src, "Some preamble. Invert, always invert. And more.")
	if _, err := run(t, "quotecheck", "--source-text", src, skill); err != nil {
		t.Fatalf("quotecheck should pass when the quote is present: %v", err)
	}

	// Quote absent → fail with a MISS.
	writeFile(t, src, "This text does not contain the citation at all.")
	out, err := run(t, "quotecheck", "--source-text", src, skill)
	assertExit1(t, err)
	if !strings.Contains(out, "MISS") {
		t.Errorf("expected a MISS for the unlocated quote, got:\n%s", out)
	}
}

func TestLinkAppendsAndIsIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	skill := filepath.Join(dir, "inversion")
	writeFile(t, filepath.Join(skill, "SKILL.md"), "# Inversion\n\nbody\n")

	if _, err := run(t, "link", "--kind", "superseded-by", "--to", "merged-x", skill); err != nil {
		t.Fatalf("link: %v", err)
	}
	got := readFile(t, filepath.Join(skill, "SKILL.md"))
	if !strings.Contains(got, "## Related Skills") ||
		!strings.Contains(got, "superseded-by: `merged-x`") {
		t.Fatalf("link not written:\n%s", got)
	}
	// Re-linking is a no-op.
	out, err := run(t, "link", "--kind", "superseded-by", "--to", "merged-x", skill)
	if err != nil {
		t.Fatalf("re-link: %v", err)
	}
	if !strings.Contains(out, "already present") {
		t.Errorf("expected idempotent 'already present', got: %s", out)
	}
	// Unknown kind is rejected.
	if _, err := run(t, "link", "--kind", "bogus", "--to", "x", skill); err == nil {
		t.Error("expected an unknown-kind error")
	}
}

func TestA2CheckSharpAndDull(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a2 := func(signals ...string) string {
		var b strings.Builder
		b.WriteString("# S\n\n## A2 — Trigger\n\n### Language Signals\n\n")
		for _, s := range signals {
			b.WriteString("- \"" + s + "\"\n")
		}
		b.WriteString("\n## E — Execution\n\n1. x\n")
		return b.String()
	}
	merged := filepath.Join(dir, "merged")
	srcA := filepath.Join(dir, "src-a")
	writeFile(t, filepath.Join(merged, "SKILL.md"), a2("shared", "unique-one", "unique-two"))
	writeFile(t, filepath.Join(srcA, "SKILL.md"), a2("shared"))

	// Two unique signals → OK (advisory).
	out, err := run(t, "a2check", "--source-skill", srcA, merged)
	if err != nil {
		t.Fatalf("a2check: %v", err)
	}
	if !strings.Contains(out, "OK:") {
		t.Errorf("expected OK for a sharp A2, got:\n%s", out)
	}

	// A dull merged A2 (no unique signals) warns, and --strict fails.
	writeFile(t, filepath.Join(merged, "SKILL.md"), a2("shared"))
	out, err = run(t, "a2check", "--source-skill", srcA, merged)
	if err != nil {
		t.Fatalf("a2check advisory should not error: %v", err)
	}
	if !strings.Contains(out, "WARN") {
		t.Errorf("expected WARN for a dull A2, got:\n%s", out)
	}
	if _, err := run(t, "a2check", "--strict", "--source-skill", srcA, merged); err == nil {
		t.Error("--strict should fail a dull A2")
	}
}

func TestMergeIndexVerificationSummary(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	src := filepath.Join(base, "munger")
	tree := filepath.Join(base, "decisions")
	writeFile(
		t,
		filepath.Join(src, "BOOK_OVERVIEW.md"),
		"# PC — Book Overview\n\n- **Author:** M\n",
	)
	// A rejected pair recorded in the ledger supplies the V1–V4 column.
	writeFile(
		t,
		filepath.Join(src, "inv", "SKILL.md"),
		"# Inv\n\n## Merge Status\n\n```yaml\n- run: decisions\n  state: rejected\n  pair: inv-vs-ctrl\n  reason: v3-failed\n```\n",
	)
	writeFile(t, filepath.Join(tree, "combined", "SKILL.md"), "# Combined\n\nbody\n")
	// A verification artifact header for the same pair.
	writeFile(t, filepath.Join(tree, "source-verification", "inv-vs-ctrl-r.md"),
		"---\npair: inv-vs-ctrl\ncheck: r-quote-accuracy\nsources:\n"+
			"  - book: munger\n    skill: inv\n    status: not-found\n---\n\nnotes\n")

	if _, err := run(t, "merge-index", "--source-book", src, tree); err != nil {
		t.Fatalf("merge-index: %v", err)
	}
	got := readFile(t, filepath.Join(tree, "INDEX.md"))
	for _, want := range []string{
		"## Source Verification Summary", "`inv-vs-ctrl`",
		"munger/inv: not-found", "v3-failed",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("INDEX.md summary missing %q:\n%s", want, got)
		}
	}
}

func TestMergeIndexAutoDiscoversSources(t *testing.T) {
	t.Parallel()
	books := filepath.Join(t.TempDir(), "books")
	tree := filepath.Join(books, "merged", "decisions")
	// A sibling source book whose skill's ledger references the run "decisions".
	src := filepath.Join(books, "munger")
	writeFile(
		t,
		filepath.Join(src, "inv", "SKILL.md"),
		"# Inv\n\n## Merge Status\n\n```yaml\n- run: decisions\n  state: merged\n  into: combined\n```\n",
	)
	writeFile(t, filepath.Join(tree, "combined", "SKILL.md"), "# Combined\n\nbody\n")

	// No --source-book: it is discovered from the books/merged/ layout.
	if _, err := run(t, "merge-index", tree); err != nil {
		t.Fatalf("merge-index without --source-book should auto-discover: %v", err)
	}
	got := readFile(t, filepath.Join(tree, "INDEX.md"))
	if !strings.Contains(got, "`munger/inv`") {
		t.Errorf("auto-discovered source not in INDEX:\n%s", got)
	}

	// A non-canonical layout (tree not under books/merged/) cannot infer sources.
	flat := t.TempDir()
	writeFile(t, filepath.Join(flat, "combined", "SKILL.md"), "# Combined\n")
	if _, err := run(t, "merge-index", flat); err == nil {
		t.Error("merge-index should error when --source-book is absent and cannot be inferred")
	}
}

func TestMergeIndexGeneratesAndChecks(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	// A source book whose skill's ledger records it merged into "combined".
	src := filepath.Join(base, "munger")
	writeFile(t, filepath.Join(src, "BOOK_OVERVIEW.md"),
		"# Poor Charlie's Almanack — Book Overview\n\n- **Author:** Munger\n")
	writeFile(
		t,
		filepath.Join(src, "inversion", "SKILL.md"),
		"# Inversion\n\n## Merge Status\n\n```yaml\n- run: decisions\n  state: merged\n  into: combined\n```\n",
	)
	// The merged tree with the merged skill.
	tree := filepath.Join(base, "decisions")
	writeFile(t, filepath.Join(tree, "combined", "SKILL.md"), "# Combined\n\nbody\n")

	if _, err := run(t, "merge-index", "--source-book", src, tree); err != nil {
		t.Fatalf("merge-index: %v", err)
	}
	got := readFile(t, filepath.Join(tree, "INDEX.md"))
	if !strings.Contains(got, "`munger/inversion`") || !strings.Contains(got, "superseded-by") {
		t.Fatalf("INDEX.md missing provenance/graph:\n%s", got)
	}
	// --check passes right after generation, and is padding-tolerant.
	if _, err := run(t, "merge-index", "--check", "--source-book", src, tree); err != nil {
		t.Errorf("--check should pass a freshly generated INDEX.md: %v", err)
	}
	// Padding the table (as a formatter would) must not make --check stale.
	padded := strings.ReplaceAll(got, "| `munger/inversion` |", "| `munger/inversion`   |")
	writeFile(t, filepath.Join(tree, "INDEX.md"), padded)
	if _, err := run(t, "merge-index", "--check", "--source-book", src, tree); err != nil {
		t.Errorf("--check should tolerate table padding: %v", err)
	}
}

func TestVerifyPassesOnConsistentTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	buildValidTree(t, dir)

	out, err := run(t, "verify", dir)
	if err != nil {
		t.Fatalf("verify should pass a consistent tree, got %v\n%s", err, out)
	}
	if strings.Contains(out, "FAIL") {
		t.Errorf("no gate should fail:\n%s", out)
	}
}

func TestVerifyFailsOnMissingTestPrompts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	buildValidTree(t, dir)
	if err := os.Remove(filepath.Join(dir, "inversion-thinking", "test-prompts.json")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	out, err := run(t, "verify", dir)
	assertExit1(t, err)
	if !strings.Contains(out, "test-prompts") || !strings.Contains(out, "FAIL") {
		t.Errorf("expected a test-prompts FAIL, got:\n%s", out)
	}
}

func TestVerifyMergePassesAndFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "MERGE_OVERVIEW.md"), "# Merge\n\nsources and rationale\n")
	s := validSkill()
	writeFile(t, filepath.Join(dir, s.Slug, "SKILL.md"), render.Skill(s))
	merged, err := book2skill.EncodeTestPrompts(book2skill.TemplateMergedTestCases())
	if err != nil {
		t.Fatalf("encode merged prompts: %v", err)
	}
	promptsPath := filepath.Join(dir, s.Slug, "test-prompts.json")
	writeFile(t, promptsPath, string(merged))

	if _, err := run(t, "verify", "--merge", dir); err != nil {
		t.Fatalf("verify --merge should pass a consistent merged tree: %v", err)
	}

	// Replace with a 3-category set: the merge tests gate must fail.
	plain, err := book2skill.EncodeTestPrompts(book2skill.TemplateTestCases())
	if err != nil {
		t.Fatalf("encode plain prompts: %v", err)
	}
	writeFile(t, promptsPath, string(plain))
	out, err := run(t, "verify", "--merge", dir)
	assertExit1(t, err)
	if !strings.Contains(out, "test-prompts") || !strings.Contains(out, "FAIL") {
		t.Errorf("expected merge test-prompts FAIL, got:\n%s", out)
	}
}

func TestVerifyMergeA2SharpnessAdvisory(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	tree := filepath.Join(base, "decisions")
	writeFile(t, filepath.Join(tree, "MERGE_OVERVIEW.md"), "# Merge\n\nsources and rationale\n")
	s := validSkill() // slug inversion-thinking, one A2 language signal
	writeFile(t, filepath.Join(tree, s.Slug, "SKILL.md"), render.Skill(s))
	merged, err := book2skill.EncodeTestPrompts(book2skill.TemplateMergedTestCases())
	if err != nil {
		t.Fatalf("encode merged prompts: %v", err)
	}
	writeFile(t, filepath.Join(tree, s.Slug, "test-prompts.json"), string(merged))
	// A source skill that merged into it, sharing the one A2 signal — so the
	// merged A2 has no unique signal and the advisory gate warns.
	src := filepath.Join(base, "munger")
	writeFile(t, filepath.Join(src, "src-inv", "SKILL.md"),
		"# Src\n\n## A2 — Trigger\n\n### Language Signals\n\n- \"how do I succeed at X\"\n\n"+
			"## Merge Status\n\n```yaml\n- run: decisions\n  state: merged\n  into: inversion-thinking\n```\n")

	out, err := run(t, "verify", "--merge", "--source-book", src, tree)
	if err != nil {
		t.Fatalf("advisory a2-sharpness must not fail the run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "a2-sharpness") || !strings.Contains(out, "WARN") {
		t.Errorf("expected an a2-sharpness WARN, got:\n%s", out)
	}
	// --strict escalates the advisory to a failure.
	if _, err := run(t, "verify", "--merge", "--strict", "--source-book", src, tree); err == nil {
		t.Error("--strict should fail when A2 is not sharp")
	}
}

// buildValidTree writes a self-consistent book tree using the real renderers,
// then generates a current INDEX.md via the index command — so every verify gate
// should pass.
func buildValidTree(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "BOOK_OVERVIEW.md"), render.BookOverview(validOverview()))
	s := validSkill()
	writeFile(t, filepath.Join(dir, s.Slug, "SKILL.md"), render.Skill(s))
	prompts, err := book2skill.EncodeTestPrompts(book2skill.TemplateTestCases())
	if err != nil {
		t.Fatalf("encode prompts: %v", err)
	}
	writeFile(t, filepath.Join(dir, s.Slug, "test-prompts.json"), string(prompts))
	if _, err := run(t, "index", dir); err != nil {
		t.Fatalf("index (tree setup): %v", err)
	}
}

func validOverview() *book2skill.BookOverview {
	return &book2skill.BookOverview{
		Title: "Poor Charlie's Almanack", Author: "Charlie Munger", Year: "2005",
		Structure: book2skill.Structure{
			Genre: "essays", OneSentenceSummary: "Worldly wisdom via a latticework of models.",
			Skeleton: []string{"models", "inversion", "incentives"},
		},
		Interpretation: book2skill.Interpretation{KeyTerms: []book2skill.KeyTerm{
			{Term: "latticework"},
			{Term: "circle of competence"},
			{Term: "lollapalooza"},
			{Term: "inversion"},
			{Term: "incentive-caused bias"},
		}},
		Critique: book2skill.Critique{
			EraLimitations:      []string{"pre-2008 finance"},
			AuthorBlindSpots:    []string{"survivorship"},
			UnprovenAssumptions: []string{"rationality is teachable"},
		},
	}
}

func validSkill() *book2skill.Skill {
	return &book2skill.Skill{
		Slug:  "inversion-thinking",
		Title: "Inversion Thinking",
		Description: "Invoke when a user is stuck on a decision and keeps " +
			"listing only reasons in favor of one option.",
		Reading:        book2skill.Reading{Quote: "Invert, always invert.", Attribution: "Jacobi"},
		Interpretation: "Ask what would guarantee failure, then avoid it.",
		Application: []book2skill.ApplicationCase{{
			Name: "Avoiding ruin", Problem: "risk", MethodologyUse: "listed failures",
			Conclusion: "avoid them", Result: "survived",
		}},
		Trigger: book2skill.Trigger{
			Scenarios:       []string{"stuck on a decision"},
			LanguageSignals: []string{"how do I succeed at X"},
		},
		Execution: []book2skill.Step{{
			Text: "List failure modes", CompletionCriterion: "at least three listed",
		}},
		Boundary:   book2skill.Boundary{AntiScenarios: []string{"pure information lookup"}},
		Provenance: "Poor Charlie's Almanack by Charlie Munger",
	}
}

// run dispatches args through the real cmd.Run with injected I/O and returns
// captured stdout and the dispatch error. Stderr is discarded.
func run(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	var out bytes.Buffer
	err = cmd.Run(context.Background(), args, strings.NewReader(""), &out, io.Discard)
	return out.String(), err
}

// runCapture is run but also returns captured stderr, for commands that emit
// advisory diagnostics there.
func runCapture(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errBuf bytes.Buffer
	err = cmd.Run(context.Background(), args, strings.NewReader(""), &out, &errBuf)
	return out.String(), errBuf.String(), err
}

// assertExit1 asserts err is a root.ExitError(1) — the failure code every gate
// command returns.
func assertExit1(t *testing.T, err error) {
	t.Helper()
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected root.ExitError, got %v", err)
	}
	if int(exit) != 1 {
		t.Fatalf("exit code = %d, want 1", int(exit))
	}
}

func learningPos(index, slug string) int {
	return strings.Index(index, "`"+slug+"`\n")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
