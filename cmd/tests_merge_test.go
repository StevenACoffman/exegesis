package cmd_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd/root"
)

// mergedTP is a set conforming to the merged composition: 3 should_trigger,
// 2 should_not_trigger, 2 edge_case (the raised floor), 2 prefer_merged_over_source.
const mergedTP = `{"tests":[
{"id":1,"type":"should_trigger","prompt":"p","expected":"e"},
{"id":2,"type":"should_trigger","prompt":"p","expected":"e"},
{"id":3,"type":"should_trigger","prompt":"p","expected":"e"},
{"id":4,"type":"should_not_trigger","prompt":"p","expected":"e"},
{"id":5,"type":"should_not_trigger","prompt":"p","expected":"e"},
{"id":6,"type":"edge_case","prompt":"p","expected":"e"},
{"id":7,"type":"edge_case","prompt":"p","expected":"e"},
{"id":8,"type":"prefer_merged_over_source","prompt":"p","expected":"e"},
{"id":9,"type":"prefer_merged_over_source","prompt":"p","expected":"e"}]}`

// writeTP writes an arbitrary test-prompts.json into a fresh directory.
func writeTP(t *testing.T, body string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "a-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "test-prompts.json"), []byte(body), 0o644,
	); err != nil {
		t.Fatalf("write: %v", err)
	}
	return dir
}

func TestTestsGateUnchangedByTheMergeWork(t *testing.T) {
	t.Parallel()
	// The plain gate must behave exactly as before: same verdict, same counts.
	dir := writeTP(t, validTP)
	out, err := run(t, "tests", dir)
	if err != nil {
		t.Fatalf("a standard-conforming set should pass: %v\n%s", err, out)
	}
	// The whole line, not three substrings: an earlier version of this test passed
	// while the column order had silently been reshuffled to alphabetical.
	const want = "a-skill: 3 should_trigger, 2 should_not_trigger, 1 edge_case"
	if !strings.Contains(out, want) {
		t.Errorf("gate line changed.\n want: %s\n got:\n%s", want, out)
	}
}

func TestTestsMergeAcceptsAMergedSet(t *testing.T) {
	t.Parallel()
	dir := writeTP(t, mergedTP)
	out, err := run(t, "tests", "--merge", dir)
	if err != nil {
		t.Fatalf("a conforming merged set should pass --merge: %v\n%s", err, out)
	}
	// The added category goes last, after the three standard ones in their usual order.
	const want = "a-skill: 3 should_trigger, 2 should_not_trigger, 2 edge_case, " +
		"2 prefer_merged_over_source"
	if !strings.Contains(out, want) {
		t.Errorf("merged tally wrong.\n want: %s\n got:\n%s", want, out)
	}
}

func TestTestsPlainGateRejectsAMergedSet(t *testing.T) {
	t.Parallel()
	// Without --merge the extra category is an unknown case type, so a merged set must
	// be checked with --merge or not at all. Silently accepting it would mean its
	// distinguishing category was never gated.
	dir := writeTP(t, mergedTP)
	out, err := run(t, "tests", dir)
	if err == nil {
		t.Fatalf("the plain gate accepted an unknown case type:\n%s", out)
	}
	if !strings.Contains(out, "unknown type") {
		t.Errorf("expected an unknown-type problem, got:\n%s", out)
	}
}

func TestTestsMergeRaisesTheEdgeCaseFloor(t *testing.T) {
	t.Parallel()
	// One edge_case satisfies the standard gate but not the merged one: a merged skill
	// inherits both parents' boundaries.
	oneEdge := strings.Replace(mergedTP,
		`{"id":7,"type":"edge_case","prompt":"p","expected":"e"},`, "", 1)
	dir := writeTP(t, oneEdge)
	out, err := run(t, "tests", "--merge", dir)
	if err == nil {
		t.Fatalf("--merge accepted a single edge_case:\n%s", out)
	}
	if !strings.Contains(out, "need >=2 edge_case") {
		t.Errorf("expected the raised edge floor to be reported, got:\n%s", out)
	}
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("a failing gate should be an ExitError, got %T", err)
	}
}

func TestTestsMergeReportsAMissingFourthCategory(t *testing.T) {
	t.Parallel()
	dir := writeTP(t, strings.Replace(mergedTP,
		`{"id":9,"type":"prefer_merged_over_source","prompt":"p","expected":"e"}`,
		`{"id":9,"type":"edge_case","prompt":"p","expected":"e"}`, 1))
	out, err := run(t, "tests", "--merge", dir)
	if err == nil {
		t.Fatalf("--merge accepted a short fourth category:\n%s", out)
	}
	if !strings.Contains(out, "need >=2 prefer_merged_over_source") {
		t.Errorf("expected the shortfall to name the category, got:\n%s", out)
	}
}

func TestTestsMigrateRewritesLegacyShapes(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ body, wantReported string }{
		"bare top-level array": {
			body:         `[{"id":1,"type":"should_trigger","prompt":"p","expected":"e"}]`,
			wantReported: "top-level array",
		},
		"legacy test_cases key": {
			body:         `{"test_cases":[{"id":1,"type":"should_trigger","prompt":"p","expected":"e"}]}`,
			wantReported: `legacy "test_cases" key`,
		},
		"legacy expected_behavior": {
			body:         `{"tests":[{"id":1,"type":"should_trigger","prompt":"p","expected_behavior":"e"}]}`,
			wantReported: `legacy "expected_behavior"`,
		},
		"non-numeric id": {
			body:         `{"tests":[{"id":"st-01","type":"should_trigger","prompt":"p","expected":"e"}]}`,
			wantReported: "renumbered",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := writeTP(t, tc.body)
			out, err := run(t, "tests", "--migrate", dir)
			if err != nil {
				t.Fatalf("migrate failed: %v\n%s", err, out)
			}
			if !strings.Contains(out, tc.wantReported) {
				t.Errorf("change not reported (%q), got:\n%s", tc.wantReported, out)
			}
			// The rewritten file must itself be canonical: migrating again is a no-op.
			again, err := run(t, "tests", "--migrate", dir)
			if err != nil {
				t.Fatalf("second migrate failed: %v\n%s", err, again)
			}
			if !strings.Contains(again, "already canonical") {
				t.Errorf("migrate is not idempotent, second pass said:\n%s", again)
			}
		})
	}
}

func TestTestsMigrateLeavesACanonicalFileAlone(t *testing.T) {
	t.Parallel()
	dir := writeTP(t, validTP)
	before, err := os.ReadFile(filepath.Join(dir, "test-prompts.json"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := run(t, "tests", "--migrate", dir)
	if err != nil {
		t.Fatalf("migrate failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "already canonical") {
		t.Errorf("expected no changes, got:\n%s", out)
	}
	after, err := os.ReadFile(filepath.Join(dir, "test-prompts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("migrate rewrote a file it reported as already canonical")
	}
}

func TestTestsMigrateRefusesToDeleteCases(t *testing.T) {
	t.Parallel()
	// The one rewrite that destroys work rather than reshaping it: the reader keeps
	// "tests" and drops "test_cases", so writing back would delete cases still on disk.
	body := `{"tests":[{"id":1,"type":"should_trigger","prompt":"p","expected":"e"}],` +
		`"test_cases":[{"id":2,"type":"edge_case","prompt":"q","expected":"f"}]}`
	dir := writeTP(t, body)
	out, err := run(t, "tests", "--migrate", dir)
	if err == nil {
		t.Fatalf("migrate destroyed cases instead of refusing:\n%s", out)
	}
	if !strings.Contains(err.Error(), "merge them by hand") {
		t.Errorf("refusal should say what to do, got: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "test-prompts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != body {
		t.Errorf("the file was modified despite the refusal:\n%s", after)
	}
}

func TestTestsScaffoldAndMigrateAreRefusedTogether(t *testing.T) {
	t.Parallel()
	dir := writeTP(t, validTP)
	out, err := run(t, "tests", "--scaffold", "--migrate", dir)
	if err == nil {
		t.Fatalf("two writing modes were accepted at once:\n%s", out)
	}
	var usage root.UsageError
	if !errors.As(err, &usage) {
		t.Errorf("expected a UsageError for a bad flag combination, got %T", err)
	}
}

// TestTestsMigrateRefusalKeepsBothCounts pins the numbers, not just the refusal. The counts
// are what make it actionable — an author needs to know how much is on each side before
// merging by hand — and they are the thing most easily lost when the detector behind them
// changes. The kept count now comes from len(f.Tests) rather than a second raw read, which
// is only correct because normalize appends one case per input and never drops.
func TestTestsMigrateRefusalKeepsBothCounts(t *testing.T) {
	t.Parallel()
	// Deliberately lopsided: 1 kept against 3 dropped, so a transposition or a reused
	// variable shows up rather than reading plausibly.
	body := `{"tests":[{"id":1,"type":"should_trigger","prompt":"p","expected":"e"}],` +
		`"test_cases":[` +
		`{"id":2,"type":"edge_case","prompt":"q","expected":"f"},` +
		`{"id":3,"type":"edge_case","prompt":"r","expected":"g"},` +
		`{"id":4,"type":"edge_case","prompt":"s","expected":"h"}]}`
	_, err := run(t, "tests", "--migrate", writeTP(t, body))
	if err == nil {
		t.Fatal("migrate destroyed cases instead of refusing")
	}
	for _, want := range []string{`"tests" (1 cases)`, `"test_cases" (3 cases)`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal is missing %s; got: %v", want, err)
		}
	}
}

// TestTestsMigrateMalformedStillReportsTheParseError pins that moving the refusal after
// Parse changed nothing. It used to run first, on the raw bytes; the predicate it now asks
// only exists on a parsed file. A decode failure could never trip the old check either — it
// left both fields empty — so a malformed file has always surfaced its parse error, and must
// still.
func TestTestsMigrateMalformedStillReportsTheParseError(t *testing.T) {
	t.Parallel()
	// Both keys present and the document truncated, so the old order and the new one are
	// distinguishable if the reordering mattered.
	dir := writeTP(t, `{"tests":[{"id":1,"prompt":"p"}],"test_cases":[{"id":2,`)
	_, err := run(t, "tests", "--migrate", dir)
	if err == nil {
		t.Fatal("a malformed file was accepted")
	}
	if strings.Contains(err.Error(), "merge them by hand") {
		t.Errorf("reported the refusal for an unparseable file: %v", err)
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected a parse error, got: %v", err)
	}
}
