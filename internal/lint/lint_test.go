package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/skill"
)

func TestCheckClean(t *testing.T) {
	t.Parallel()
	s := &skill.Skill{
		Dir:             "/tmp/good-skill",
		Name:            "good-skill",
		Description:     "Invoke when the user needs a well-formed thing done a specific way.",
		FrontmatterKeys: []string{"description", "name", "tags"},
		Body:            "# Body\nSee `New[T](x)` for generics.\nRelated: `other-skill`.\n",
		Raw:             "runtime neutral content",
	}
	if fs := lint.Check(s, lint.Options{}); len(fs) != 0 {
		t.Fatalf("expected clean, got %v", fs)
	}
}

func TestCheckBudgetAndSections(t *testing.T) {
	t.Parallel()
	s := &skill.Skill{
		Dir:         "/x/s",
		Name:        "s",
		Description: "one two three four five six",
		Body:        "## When to Use\nsome content here\n\n## Other\nmore\n",
		Raw:         "neutral",
	}
	tests := []struct {
		name    string
		opts    lint.Options
		wantSub string // "" => expect clean
	}{
		{name: "zero options is a no-op", opts: lint.Options{}},
		{
			name:    "desc over budget",
			opts:    lint.Options{MaxDescriptionWords: 3},
			wantSub: "description 6 words > max 3",
		},
		{name: "body over budget", opts: lint.Options{MaxBodyWords: 3}, wantSub: "words > max"},
		{
			name: "required section present",
			opts: lint.Options{RequiredSections: []string{"When to Use"}},
		},
		{
			name:    "required section missing",
			opts:    lint.Options{RequiredSections: []string{"Verification"}},
			wantSub: "required section \"Verification\"",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := lint.Check(s, tc.opts)
			switch {
			case tc.wantSub == "" && len(fs) != 0:
				t.Errorf("expected clean, got %v", fs)
			case tc.wantSub != "" && !hasFinding(fs, tc.wantSub):
				t.Errorf("expected a finding containing %q, got %v", tc.wantSub, fs)
			}
		})
	}
}

func TestSectionRequiresNonEmpty(t *testing.T) {
	t.Parallel()
	// A heading present but with no content under it does not satisfy the check.
	s := &skill.Skill{
		Dir: "/x/s", Name: "s", Description: "d", Raw: "neutral",
		Body: "## When NOT to Use\n\n## Next\ncontent\n",
	}
	fs := lint.Check(s, lint.Options{RequiredSections: []string{"When NOT to Use"}})
	if !hasFinding(fs, "required section \"When NOT to Use\"") {
		t.Errorf("empty section should fail the required-section check, got %v", fs)
	}
}

func TestCheckDefects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		skill   skill.Skill
		wantSub string
	}{
		{
			name: "disallowed key",
			skill: skill.Skill{
				Dir:             "/x/s",
				Name:            "s",
				Description:     "d",
				FrontmatterKeys: []string{"source_book"},
			},
			wantSub: "disallowed key",
		},
		{
			name:    "name mismatch",
			skill:   skill.Skill{Dir: "/x/folder", Name: "different", Description: "d"},
			wantSub: "!= folder",
		},
		{
			name:    "empty description",
			skill:   skill.Skill{Dir: "/x/s", Name: "s", Description: ""},
			wantSub: "description is empty",
		},
		{
			name:    "angle brackets in description",
			skill:   skill.Skill{Dir: "/x/s", Name: "s", Description: "use <this> thing"},
			wantSub: "angle brackets",
		},
		{
			name: "parent-escaping body link",
			skill: skill.Skill{
				Dir:         "/x/s",
				Name:        "s",
				Description: "d",
				Body:        "see [x](../other/SKILL.md)",
			},
			wantSub: "parent-escaping link",
		},
		{
			name: "candidates path in body",
			skill: skill.Skill{
				Dir:         "/x/s",
				Name:        "s",
				Description: "d",
				Body:        "from candidates/pool.md",
			},
			wantSub: "candidates/",
		},
		{
			name: "runtime-bound wording",
			skill: skill.Skill{
				Dir:         "/x/s",
				Name:        "s",
				Description: "d",
				Raw:         "This is a Claude Code skill.",
			},
			wantSub: "runtime-bound",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fs := lint.Check(&tc.skill, lint.Options{})
			if !hasFinding(fs, tc.wantSub) {
				t.Errorf("Check = %v, want a finding containing %q", fs, tc.wantSub)
			}
		})
	}
}

func TestCheckIgnoresLinksInsideCodeFences(t *testing.T) {
	t.Parallel()
	s := skill.Skill{
		Dir:         "/x/s",
		Name:        "s",
		Description: "d",
		Body:        "```\nsee [x](../escape.md) inside a fence\n```\nprose is clean\n",
	}
	if fs := lint.Check(&s, lint.Options{}); len(fs) != 0 {
		t.Errorf("links inside fences must be ignored, got %v", fs)
	}
}

func hasFinding(fs []finding.Diagnostic, sub string) bool {
	for _, f := range fs {
		if strings.Contains(f.Message, sub) {
			return true
		}
	}
	return false
}
