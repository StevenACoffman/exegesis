package testprompts_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/testprompts"
)

func TestValidateComposition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		file       testprompts.File
		wantOK     bool
		wantSubstr string
	}{
		{
			name:   "passing scaffold",
			file:   *testprompts.Scaffold("x"),
			wantOK: true,
		},
		{
			name: "too few triggers",
			file: testprompts.File{Tests: []testprompts.Case{
				{ID: 1, Type: testprompts.TypeShouldTrigger, Prompt: "p", Expected: "e"},
				{ID: 2, Type: testprompts.TypeShouldNotTrigger, Prompt: "p", Expected: "e"},
				{ID: 3, Type: testprompts.TypeShouldNotTrigger, Prompt: "p", Expected: "e"},
				{ID: 4, Type: testprompts.TypeEdgeCase, Prompt: "p", Expected: "e"},
			}},
			wantOK:     false,
			wantSubstr: "should_trigger",
		},
		{
			name: "unknown type and empty prompt",
			file: testprompts.File{Tests: []testprompts.Case{
				{ID: 1, Type: "bogus", Prompt: "", Expected: "e"},
			}},
			wantOK:     false,
			wantSubstr: "unknown type",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := tc.file
			problems := f.Validate()
			switch {
			case tc.wantOK && len(problems) != 0:
				t.Fatalf("expected pass, got problems: %v", problems)
			case !tc.wantOK && len(problems) == 0:
				t.Fatal("expected problems, got none")
			case !tc.wantOK && tc.wantSubstr != "" && !containsAny(problems, tc.wantSubstr):
				t.Errorf("problems %v missing substring %q", problems, tc.wantSubstr)
			}
		})
	}
}

func TestDeriveChecks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expected string
		want     []testprompts.Check
	}{
		{
			name:     "quoted section",
			expected: `output contains a "Risks" section`,
			want:     []testprompts.Check{{Op: "section_present", Arg: "Risks"}},
		},
		{
			name:     "max chars",
			expected: "a terse answer under 4000 characters",
			want:     []testprompts.Check{{Op: "max_chars", Arg: "4000"}},
		},
		{
			name:     "min chars",
			expected: "a thorough answer of at least 200 characters",
			want:     []testprompts.Check{{Op: "min_chars", Arg: "200"}},
		},
		{
			name:     "tool called",
			expected: "the response invokes the \"search\" tool",
			want:     []testprompts.Check{{Op: "tool_called", Arg: "search"}},
		},
		{
			name:     "explicit contains cue",
			expected: `the answer mentions "error budget" somewhere`,
			want:     []testprompts.Check{{Op: "contains", Arg: "error budget"}},
		},
		{
			name:     "nothing inferable yields nil",
			expected: "a generally reasonable and helpful response",
			want:     nil,
		},
		{
			name:     "dedup identical cues",
			expected: `a "Risks" section; again a "Risks" section`,
			want:     []testprompts.Check{{Op: "section_present", Arg: "Risks"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := testprompts.DeriveChecks(tc.expected)
			if !equalChecks(got, tc.want) {
				t.Errorf("DeriveChecks(%q) = %v, want %v", tc.expected, got, tc.want)
			}
		})
	}
}

func TestScaffoldSeedsChecksAndPassesGate(t *testing.T) {
	t.Parallel()
	f := testprompts.Scaffold("demo")
	if problems := f.Validate(); len(problems) != 0 {
		t.Fatalf("scaffold should pass its own gate, got %v", problems)
	}
	// The trigger placeholder's expected names a "Result" section, so E2 seeding
	// must have produced a section_present check.
	if len(f.Tests[0].Checks) == 0 {
		t.Fatal("expected scaffold trigger case to have a derived check")
	}
}

func TestLoadWriteRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test-prompts.json")
	orig := testprompts.Scaffold("round")
	if err := testprompts.Write(path, orig); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := testprompts.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Tests) != len(orig.Tests) {
		t.Errorf("round-trip case count = %d, want %d", len(got.Tests), len(orig.Tests))
	}
	if got.Skill != "round" {
		t.Errorf("round-trip skill = %q, want round", got.Skill)
	}
}

func containsAny(problems []string, substr string) bool {
	for _, p := range problems {
		if strings.Contains(p, substr) {
			return true
		}
	}
	return false
}

func equalChecks(a, b []testprompts.Check) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
