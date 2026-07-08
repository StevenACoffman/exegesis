package book2skill_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

func TestMergeStatusEntryValidate(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		entry   book2skill.MergeStatusEntry
		wantOK  bool
		hasRule string // a substring expected in some problem when invalid
	}{
		"merged ok": {
			book2skill.MergeStatusEntry{Run: "r1", State: book2skill.StateMerged, Into: "m"},
			true, "",
		},
		"rejected ok": {
			book2skill.MergeStatusEntry{
				Run: "r1", State: book2skill.StateRejected,
				Pair: "p1", Reason: book2skill.ReasonV4FailedMergeNotAdditive,
			},
			true, "",
		},
		"missing run": {
			book2skill.MergeStatusEntry{State: book2skill.StateNoCandidate},
			false, "run is required",
		},
		"unknown state": {
			book2skill.MergeStatusEntry{Run: "r", State: "bogus"},
			false, "unknown state",
		},
		"merged without into": {
			book2skill.MergeStatusEntry{Run: "r", State: book2skill.StateMerged},
			false, "requires into",
		},
		"rejected without reason": {
			book2skill.MergeStatusEntry{Run: "r", State: book2skill.StateRejected, Pair: "p"},
			false, "requires reason",
		},
		"partial without excluded": {
			book2skill.MergeStatusEntry{Run: "r", State: book2skill.StatePartial, Into: "m"},
			false, "requires excluded",
		},
		"reason on non-rejected": {
			book2skill.MergeStatusEntry{
				Run: "r", State: book2skill.StateMerged, Into: "m",
				Reason: book2skill.ReasonV1Failed,
			},
			false, "only valid with state rejected",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry := tc.entry
			problems := entry.Validate()
			if tc.wantOK && len(problems) != 0 {
				t.Fatalf("expected valid, got %v", problems)
			}
			if !tc.wantOK && !containsSubstring(problems, tc.hasRule) {
				t.Errorf("expected a problem containing %q, got %v", tc.hasRule, problems)
			}
		})
	}
}

func containsSubstring(problems []string, want string) bool {
	for _, p := range problems {
		if want != "" && strings.Contains(p, want) {
			return true
		}
	}
	return false
}
