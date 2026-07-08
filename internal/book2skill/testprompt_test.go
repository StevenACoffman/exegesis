package book2skill_test

import (
	"encoding/json"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

// TestEncodeTestPromptsDarwinShape pins the darwin-skill contract: a bare JSON
// array whose elements each carry the id/prompt/expected keys darwin reads,
// alongside book2skill's own type and notes.
func TestEncodeTestPromptsDarwinShape(t *testing.T) {
	t.Parallel()
	cases := []book2skill.TestCase{
		{
			ID:       1,
			Type:     book2skill.ShouldTrigger,
			Prompt:   "I can't decide whether to take this project.",
			Expected: "invokes inversion-thinking",
			Notes:    "positive: decision dilemma",
		},
		{
			ID:       2,
			Type:     book2skill.ShouldNotTrigger,
			Prompt:   "What are the parameters of this API?",
			Expected: "no decision skill invoked",
		},
	}

	got, err := book2skill.EncodeTestPrompts(cases)
	if err != nil {
		t.Fatalf("EncodeTestPrompts: %v", err)
	}

	// Top level must be a bare array (darwin consumes an array, not an object).
	var arr []map[string]any
	if err := json.Unmarshal(got, &arr); err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, got)
	}
	if len(arr) != len(cases) {
		t.Fatalf("array length = %d, want %d", len(arr), len(cases))
	}

	// darwin reads exactly these three keys; they must be present on every element.
	for i, elem := range arr {
		for _, key := range []string{"id", "prompt", "expected"} {
			if _, ok := elem[key]; !ok {
				t.Errorf("element %d missing darwin key %q", i, key)
			}
		}
	}

	// id must be a JSON number (darwin uses integer ids).
	if _, ok := arr[0]["id"].(float64); !ok {
		t.Errorf("id is %T, want JSON number", arr[0]["id"])
	}

	// omitempty: the second case has no notes, so the key must be absent.
	if _, ok := arr[1]["notes"]; ok {
		t.Errorf("element 1 should omit empty notes, got %v", arr[1]["notes"])
	}
}

func TestValidateTestSet(t *testing.T) {
	t.Parallel()
	pass := book2skill.TemplateTestCases()
	if problems := book2skill.ValidateTestSet(pass); len(problems) != 0 {
		t.Fatalf("TemplateTestCases should pass the gate, got %v", problems)
	}

	cases := map[string]struct {
		cases    []book2skill.TestCase
		wantHits int
	}{
		"empty fails all three": {nil, 3},
		"only triggers": {
			[]book2skill.TestCase{
				{Type: book2skill.ShouldTrigger},
				{Type: book2skill.ShouldTrigger},
				{Type: book2skill.ShouldTrigger},
			},
			2, // missing decoys + edge
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := len(book2skill.ValidateTestSet(tc.cases)); got != tc.wantHits {
				t.Errorf("ValidateTestSet problems = %d, want %d", got, tc.wantHits)
			}
		})
	}
}

func TestValidateMergedTestSet(t *testing.T) {
	t.Parallel()
	merged := book2skill.ValidateMergedTestSet(book2skill.TemplateMergedTestCases())
	if len(merged) != 0 {
		t.Fatalf("TemplateMergedTestCases should pass the merge gate, got %v", merged)
	}
	// A book2skill (3-category) set is missing prefer_merged_over_source and one
	// edge case, so it fails the stricter merge gate.
	threeCategory := book2skill.ValidateMergedTestSet(book2skill.TemplateTestCases())
	if len(threeCategory) == 0 {
		t.Error("a 3-category set should fail the merge gate (no prefer_merged, too few edge)")
	}
}

func TestDecodeTestPromptsRoundTrip(t *testing.T) {
	t.Parallel()
	want := book2skill.TemplateTestCases()
	data, err := book2skill.EncodeTestPrompts(want)
	if err != nil {
		t.Fatalf("EncodeTestPrompts: %v", err)
	}
	got, err := book2skill.DecodeTestPrompts(data)
	if err != nil {
		t.Fatalf("DecodeTestPrompts: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("decoded %d cases, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("case %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestDecodeTestPromptsRejectsNonArray(t *testing.T) {
	t.Parallel()
	if _, err := book2skill.DecodeTestPrompts([]byte(`{"not":"an array"}`)); err == nil {
		t.Error("DecodeTestPrompts accepted a JSON object, want error")
	}
}

func TestTestTypeValid(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		tt   book2skill.TestType
		want bool
	}{
		"should_trigger":     {book2skill.ShouldTrigger, true},
		"should_not_trigger": {book2skill.ShouldNotTrigger, true},
		"edge_case":          {book2skill.EdgeCase, true},
		"unknown":            {book2skill.TestType("nonsense"), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := tc.tt.Valid(); got != tc.want {
				t.Errorf("TestType(%q).Valid() = %v, want %v", tc.tt, got, tc.want)
			}
		})
	}
}
