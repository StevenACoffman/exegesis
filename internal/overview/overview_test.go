package overview_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/overview"
)

const validDoc = `# Book — Overview

## One-sentence summary

This is a single clear sentence about the whole book.

## Skeleton

- arg one
- arg two
- arg three

## Key terms

- **a:** x
- **b:** x
- **c:** x
- **d:** x
- **e:** x

## Core propositions

- prop that must NOT count toward key terms

## Era limitations

- era one

## Author blind spots

- blind one

## Unproven assumptions

- unproven one
`

func TestCheckPassing(t *testing.T) {
	t.Parallel()
	if problems := overview.Check(validDoc); len(problems) != 0 {
		t.Fatalf("expected pass, got %v", problems)
	}
}

func TestUngatedHeadingBulletsDoNotLeak(t *testing.T) {
	t.Parallel()
	// "Core propositions" has a bullet; it must not push key terms to 6.
	// Remove one key term so key terms = 4; if leakage happened it would read 5
	// and wrongly pass.
	reduced := strings.Replace(validDoc, "- **e:** x\n", "", 1)
	problems := overview.Check(reduced)
	if !containsSub(problems, "key terms has 4") {
		t.Fatalf("expected 'key terms has 4' problem (no leakage), got %v", problems)
	}
}

func TestCheckFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(string) string
		wantSub string
	}{
		{
			name: "empty summary",
			mutate: func(s string) string {
				return strings.Replace(
					s,
					"This is a single clear sentence about the whole book.",
					"",
					1,
				)
			},
			wantSub: "One-sentence summary",
		},
		{
			name:    "too few skeleton",
			mutate:  func(s string) string { return strings.Replace(s, "- arg three\n", "", 1) },
			wantSub: "skeleton has 2",
		},
		{
			name:    "too few key terms",
			mutate:  func(s string) string { return strings.Replace(s, "- **d:** x\n- **e:** x\n", "", 1) },
			wantSub: "key terms has 3",
		},
		{
			name:    "too few critique",
			mutate:  func(s string) string { return strings.Replace(s, "- blind one\n", "", 1) },
			wantSub: "critique",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			problems := overview.Check(tc.mutate(validDoc))
			if !containsSub(problems, tc.wantSub) {
				t.Errorf("problems %v missing %q", problems, tc.wantSub)
			}
		})
	}
}

func containsSub(problems []string, sub string) bool {
	for _, p := range problems {
		if strings.Contains(p, sub) {
			return true
		}
	}
	return false
}
