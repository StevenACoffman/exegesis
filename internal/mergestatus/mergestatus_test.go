package mergestatus_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/mergestatus"
)

func TestValidateRequiresWhatTheStateNeeds(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		entry   mergestatus.Entry
		wantSub string // "" means it must validate cleanly
	}{
		"no-candidate needs only a run": {
			entry: mergestatus.Entry{Run: "r1", State: "no-candidate"},
		},
		"surface-resemblance carries its pair": {
			entry: mergestatus.Entry{Run: "r1", State: "surface-resemblance", Pair: "a-b-01"},
		},
		"rejected carries pair and reason": {
			entry: mergestatus.Entry{
				Run: "r1", State: "rejected", Pair: "a-b-01", Reason: "v1-failed",
			},
		},
		"merged carries what it merged into": {
			entry: mergestatus.Entry{Run: "r1", State: "merged", Into: "combined"},
		},
		"partial says what it left out": {
			entry: mergestatus.Entry{
				Run: "r1", State: "partial", Into: "combined", Excluded: "the worked example",
			},
		},
		"missing run": {
			entry:   mergestatus.Entry{State: "no-candidate"},
			wantSub: "run is required",
		},
		"unknown state": {
			entry:   mergestatus.Entry{Run: "r1", State: "invented"},
			wantSub: `unknown state "invented"`,
		},
		"rejected without a reason": {
			entry:   mergestatus.Entry{Run: "r1", State: "rejected", Pair: "a-b-01"},
			wantSub: `state "rejected" requires reason`,
		},
		"rejected with an unknown reason": {
			entry: mergestatus.Entry{
				Run: "r1", State: "rejected", Pair: "a-b-01", Reason: "because",
			},
			wantSub: `unknown reason "because"`,
		},
		"partial without excluded": {
			entry:   mergestatus.Entry{Run: "r1", State: "partial", Into: "combined"},
			wantSub: `state "partial" requires excluded`,
		},
		// A stray field is not harmless: a rejected entry naming what it merged into
		// is two contradictory accounts of one decision, in a file kept as evidence.
		"rejected must not claim an into": {
			entry: mergestatus.Entry{
				Run: "r1", State: "rejected", Pair: "a-b-01",
				Reason: "v1-failed", Into: "combined",
			},
			wantSub: `state "rejected" does not take into`,
		},
		"merged must not carry a reason": {
			entry: mergestatus.Entry{
				Run: "r1", State: "merged", Into: "combined", Reason: "v1-failed",
			},
			wantSub: `state "merged" does not take reason`,
		},
		"no-candidate must not carry a pair": {
			entry:   mergestatus.Entry{Run: "r1", State: "no-candidate", Pair: "a-b-01"},
			wantSub: `state "no-candidate" does not take pair`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := tc.entry.Validate()
			if tc.wantSub == "" {
				if len(got) != 0 {
					t.Errorf("a conforming entry was rejected: %q", got)
				}
				return
			}
			if !strings.Contains(strings.Join(got, "\n"), tc.wantSub) {
				t.Errorf("problems %q do not mention %q", got, tc.wantSub)
			}
		})
	}
}

func TestEveryStateInTheVocabularyCanBeSatisfied(t *testing.T) {
	t.Parallel()
	// A state whose required fields cannot all be supplied would be unreachable, and
	// nothing else in the suite would notice.
	for state, required := range mergestatus.States() {
		e := mergestatus.Entry{Run: "r1", State: state}
		for _, f := range required {
			switch f {
			case "pair":
				e.Pair = "a-b-01"
			case "into":
				e.Into = "combined"
			case "reason":
				e.Reason = "v1-failed"
			case "excluded":
				e.Excluded = "something"
			default:
				t.Fatalf("state %q requires unknown field %q", state, f)
			}
		}
		if got := e.Validate(); len(got) != 0 {
			t.Errorf("state %q cannot be satisfied by its own required fields: %q", state, got)
		}
	}
}

func TestAppendCreatesTheSectionWhenAbsent(t *testing.T) {
	t.Parallel()
	md := "---\nname: a\n---\n\n## R\n\nbody\n"
	out, err := mergestatus.Append(md, &mergestatus.Entry{
		Run: "run-1", State: "merged", Into: "combined",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, mergestatus.Heading) {
		t.Errorf("no ledger section created:\n%s", out)
	}
	if !strings.HasPrefix(out, md[:len(md)-1]) {
		t.Errorf("existing body was disturbed:\n%s", out)
	}
	entries, err := mergestatus.Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Run != "run-1" || entries[0].Into != "combined" {
		t.Errorf("round trip lost the entry: %+v", entries)
	}
}

func TestAppendPreservesEveryEarlierEntryByteForByte(t *testing.T) {
	t.Parallel()
	// The property the whole package is built around. The prior entry here is written
	// in a style this package would never emit -- extra spacing, a trailing comment --
	// so a re-render would visibly rewrite it.
	prior := "- run: run-1\n  state:    merged        # decided by hand\n  into: combined"
	md := "## R\n\nbody\n\n## Merge Status\n\n```yaml\n" + prior + "\n```\n"
	out, err := mergestatus.Append(md, &mergestatus.Entry{
		Run: "run-2", State: "no-candidate",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, prior) {
		t.Errorf("an earlier entry was rewritten:\n%s", out)
	}
	entries, err := mergestatus.Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Run != "run-1" || entries[1].Run != "run-2" {
		t.Errorf("append did not preserve order: %+v", entries)
	}
}

// mustParse parses md's ledger or fails the test.
func mustParse(t *testing.T, md string) []mergestatus.Entry {
	t.Helper()
	entries, err := mergestatus.Parse(md)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, md)
	}
	return entries
}

func TestAppendAndParseFindTheHeadingInAnyCase(t *testing.T) {
	t.Parallel()
	// rumdl rewrites headings to title case and a hand-written ledger may use either.
	// A reader that saw only one spelling would call a populated ledger absent, then
	// append a second section below the first.
	for _, heading := range []string{"## Merge Status", "## Merge status", "## MERGE STATUS"} {
		t.Run(heading, func(t *testing.T) {
			t.Parallel()
			md := "## R\n\nbody\n\n" + heading +
				"\n\n```yaml\n- run: run-1\n  state: no-candidate\n```\n"
			if got := mustParse(t, md); len(got) != 1 {
				t.Fatalf("ledger not found under %q: %+v", heading, got)
			}
			out, err := mergestatus.Append(md, &mergestatus.Entry{
				Run: "run-2", State: "merged", Into: "combined",
			})
			if err != nil {
				t.Fatalf("append: %v", err)
			}
			if n := strings.Count(out, "```yaml"); n != 1 {
				t.Errorf("append created a second ledger block (%d) under %q:\n%s",
					n, heading, out)
			}
			if got := mustParse(t, out); len(got) != 2 {
				t.Errorf("want 2 entries after append, got %d: %+v", len(got), got)
			}
		})
	}
}

func TestAppendWritesTitleCase(t *testing.T) {
	t.Parallel()
	// rumdl's MD063 rewrites a lowercase heading, so writing lowercase would leave the
	// tool and the formatter changing the same line back and forth.
	out, err := mergestatus.Append("body\n", &mergestatus.Entry{
		Run: "r", State: "no-candidate",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Merge Status") {
		t.Errorf("heading is not title case:\n%s", out)
	}
}

func TestRenderEscapesFreeText(t *testing.T) {
	t.Parallel()
	// "excluded" is prose an author types; a colon or a quote in it must not produce a
	// ledger that no longer parses.
	e := mergestatus.Entry{
		Run: "r", State: "partial", Into: "combined",
		Excluded: `the "worked example": step 3, and a #comment`,
	}
	out, err := mergestatus.Append("body\n", &e)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := mergestatus.Parse(out)
	if err != nil {
		t.Fatalf("free text broke the ledger: %v\n%s", err, out)
	}
	if len(entries) != 1 || entries[0].Excluded != e.Excluded {
		t.Errorf("excluded did not survive: %+v", entries)
	}
}

func TestParseOfASkillWithNoLedger(t *testing.T) {
	t.Parallel()
	// Absent means never evaluated in any merge run, which is not an error.
	entries, err := mergestatus.Parse("---\nname: a\n---\n\n## R\n\nbody\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("want no entries, got %+v", entries)
	}
}
