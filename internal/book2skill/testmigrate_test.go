package book2skill_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

func TestMigrateTestPrompts(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in          string
		wantIDs     []int
		wantTypes   []book2skill.TestType
		wantExp     []string // expected value per case (index-aligned); "" = skip check
		wantWarnSub []string // substrings that must appear among warnings
		noWarn      bool
	}{
		"wrapper test_cases + expected_behavior + string ids": {
			in: `{"skill":"x","version":"1","test_cases":[
				{"id":"st-1","type":"should_trigger","prompt":"p1","expected_behavior":"fires"},
				{"id":"snt-1","type":"should_not_trigger","prompt":"p2","expected_behavior":"quiet"}]}`,
			wantIDs:   []int{1, 2},
			wantTypes: []book2skill.TestType{book2skill.ShouldTrigger, book2skill.ShouldNotTrigger},
			wantExp:   []string{"fires", "quiet"},
			noWarn:    true,
		},
		"prompts key with should_invoke synonym and rationale, no expected": {
			in: `{"prompts":[
				{"id":"a","type":"should_invoke","prompt":"p","rationale":"why"}]}`,
			wantIDs:     []int{1},
			wantTypes:   []book2skill.TestType{book2skill.ShouldTrigger},
			wantWarnSub: []string{"no expected"},
		},
		"category-grouped arrays synthesize type": {
			in: `{"should_trigger":[{"id":"st-01","prompt":"a","expected":"x"}],
				"should_not_trigger":[{"id":"snt-01","prompt":"b"}],
				"edge_cases":[{"id":"ec-01","prompt":"c","expected":"y"}]}`,
			wantIDs: []int{1, 2, 3},
			wantTypes: []book2skill.TestType{
				book2skill.ShouldTrigger, book2skill.ShouldNotTrigger, book2skill.EdgeCase,
			},
			// should_not_trigger gets the mechanical default, so no warning for it.
			wantExp: []string{"x", "skill should not activate", "y"},
			noWarn:  true,
		},
		"already-canonical bare array is a no-op": {
			in:        `[{"id":1,"type":"edge_case","prompt":"p","expected":"either"}]`,
			wantIDs:   []int{1},
			wantTypes: []book2skill.TestType{book2skill.EdgeCase},
			wantExp:   []string{"either"},
			noWarn:    true,
		},
		"wrapper key holding a grouped object": {
			in: `{"skill":"x","test_cases":{
				"should_trigger":[{"id":"ST-01","prompt":"a","expected":"x"}],
				"edge_cases":[{"id":"EC-01","prompt":"b","expected":"y"}]}}`,
			wantIDs:   []int{1, 2},
			wantTypes: []book2skill.TestType{book2skill.ShouldTrigger, book2skill.EdgeCase},
			wantExp:   []string{"x", "y"},
			noWarn:    true,
		},
		"expected recovered from an expected_* variant": {
			in: `[{"id":1,"type":"should_trigger","prompt":"p",
				"expected_skill_elements":["uses loopback","no mock net.Conn"]}]`,
			wantIDs:   []int{1},
			wantTypes: []book2skill.TestType{book2skill.ShouldTrigger},
			wantExp:   []string{"uses loopback, no mock net.Conn"},
			noWarn:    true,
		},
		"unknown type is kept verbatim and warned": {
			in:          `[{"id":1,"type":"weird","prompt":"p","expected":"e"}]`,
			wantIDs:     []int{1},
			wantTypes:   []book2skill.TestType{book2skill.TestType("weird")},
			wantWarnSub: []string{"unknown type"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, warnings, err := book2skill.MigrateTestPrompts([]byte(tc.in))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertMigrated(t, got, tc.wantIDs, tc.wantTypes, tc.wantExp)
			assertWarnings(t, warnings, tc.wantWarnSub, tc.noWarn)
		})
	}
}

func assertMigrated(
	t *testing.T,
	got []book2skill.TestCase,
	ids []int,
	types []book2skill.TestType,
	exp []string,
) {
	t.Helper()
	if len(got) != len(ids) {
		t.Fatalf("got %d cases, want %d", len(got), len(ids))
	}
	for i := range got {
		if got[i].ID != ids[i] {
			t.Errorf("case %d: id = %d, want %d", i, got[i].ID, ids[i])
		}
		if got[i].Type != types[i] {
			t.Errorf("case %d: type = %q, want %q", i, got[i].Type, types[i])
		}
		if i < len(exp) && exp[i] != "" && got[i].Expected != exp[i] {
			t.Errorf("case %d: expected = %q, want %q", i, got[i].Expected, exp[i])
		}
	}
}

func assertWarnings(t *testing.T, warnings, wantSub []string, noWarn bool) {
	t.Helper()
	if noWarn && len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	for _, sub := range wantSub {
		found := false
		for _, w := range warnings {
			if strings.Contains(w, sub) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("want a warning containing %q; got %v", sub, warnings)
		}
	}
}

func TestMigrateTestPromptsPreservesRationaleInNotes(t *testing.T) {
	t.Parallel()
	got, _, err := book2skill.MigrateTestPrompts([]byte(
		`{"prompts":[{"id":"ms-si-01","type":"should_invoke","prompt":"p","rationale":"slow plan"}]}`,
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	notes := got[0].Notes
	if !strings.Contains(notes, "rationale: slow plan") {
		t.Errorf("notes should preserve rationale; got %q", notes)
	}
	if !strings.Contains(notes, "source id: ms-si-01") {
		t.Errorf("notes should preserve the non-numeric source id; got %q", notes)
	}
}

func TestMigrateTestPromptsPreservesUnknownKeysInNotes(t *testing.T) {
	t.Parallel()
	got, _, err := book2skill.MigrateTestPrompts([]byte(
		`[{"id":"tp01","type":"should_trigger","prompt":"p","expected":"e",
			"segment":"A2","should_not":false,"tags":["a","b"]}]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	notes := got[0].Notes
	for _, want := range []string{"segment: A2", "should_not: false", "tags: a, b", "source id: tp01"} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes should preserve %q; got %q", want, notes)
		}
	}
}

func TestMigrateTestPromptsErrors(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]string{
		"invalid json":  `{not json`,
		"no case array": `{"skill":"x","version":"1"}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := book2skill.MigrateTestPrompts([]byte(in)); err == nil {
				t.Errorf("want error for %s", name)
			}
		})
	}
}

// TestMigrateThenDecode confirms migrate output round-trips through the strict
// decoder the gate uses.
func TestMigrateThenDecode(t *testing.T) {
	t.Parallel()
	cases, _, err := book2skill.MigrateTestPrompts([]byte(
		`{"test_cases":[{"id":"x","type":"should_trigger","prompt":"p","expected_behavior":"e"}]}`))
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	encoded, err := book2skill.EncodeTestPrompts(cases)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := book2skill.DecodeTestPrompts(encoded); err != nil {
		t.Errorf("migrated output must decode with the strict parser: %v", err)
	}
}
