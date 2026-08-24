package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/lint"
	"github.com/StevenACoffman/skillet/skill"
)

// hiddenSkill builds a skill whose Raw carries body, so Check sees it the way it
// sees a file read from disk. Raw is what the hidden scan reads, since the threat is
// the whole document rather than the parsed body.
func hiddenSkill(body string) *skill.Skill {
	return &skill.Skill{
		Dir:             "/tmp/hid",
		Name:            "hid",
		Description:     "Invoke when the user needs a hidden-character case exercised.",
		FrontmatterKeys: []string{"description", "name"},
		Body:            "# Body\nplain.\n",
		Raw:             body,
	}
}

func TestCheckFlagsEachHiddenClass(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, raw, want string
	}{
		{"zero width space", "a\u200Bb", "hidden zero-width character at SKILL.md:1 (U+200B)"},
		{"zero width joiner", "a\u200Db", "hidden zero-width character at SKILL.md:1 (U+200D)"},
		{"word joiner", "a\u2060b", "hidden zero-width character at SKILL.md:1 (U+2060)"},
		{"bom", "a\uFEFFb", "hidden zero-width character at SKILL.md:1 (U+FEFF)"},
		{"bidi override", "a\u202Eb", "hidden bidirectional override at SKILL.md:1 (U+202E)"},
		{"bidi isolate", "a\u2066b", "hidden bidirectional override at SKILL.md:1 (U+2066)"},
		{"tag character", "a\U000E0041b", "hidden Unicode tag character at SKILL.md:1 (U+E0041)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ds := lint.Check(hiddenSkill(tc.raw), lint.Options{})
			var found bool
			for _, d := range ds {
				if strings.Contains(d.Message, tc.want) {
					found = true
				}
			}
			if !found {
				t.Errorf("expected a diagnostic containing %q, got %v", tc.want, ds)
			}
		})
	}
}

// TestCheckAcceptsTheEscapedForm pins the remedy the message names. If a literal and
// its escape were both flagged the advice would be impossible to follow, which is a
// worse failure than missing the character: the author would have nowhere to go.
func TestCheckAcceptsTheEscapedForm(t *testing.T) {
	t.Parallel()
	raw := "This skill documents U+200B, written as \\u200B so it survives a copy-paste.\n"
	for _, d := range lint.Check(hiddenSkill(raw), lint.Options{}) {
		if strings.Contains(d.Message, "hidden") {
			t.Errorf("an escape sequence must not be flagged, got %q", d.Message)
		}
	}
}

// TestCheckReportsHiddenInsideFences pins the deliberate choice to scan the whole
// document. A fence does not stop an agent reading what is inside it, so exempting
// fenced content would leave the vector open in the one place a payload is easiest
// to disguise.
func TestCheckReportsHiddenInsideFences(t *testing.T) {
	t.Parallel()
	raw := "text\n\n```sh\necho a\u200Bb\n```\n"
	var found bool
	for _, d := range lint.Check(hiddenSkill(raw), lint.Options{}) {
		if strings.Contains(d.Message, "hidden zero-width character at SKILL.md:4") {
			found = true
		}
	}
	if !found {
		t.Error("expected a hidden character inside a fence to be reported, with its line")
	}
}

// TestCheckReportsOneHitPerClassPerLine keeps a tampered file from burying every
// other diagnostic: a payload is many characters, and one line of output per class
// is enough to act on.
func TestCheckReportsOneHitPerClassPerLine(t *testing.T) {
	t.Parallel()
	raw := "a\u200Bb\u200Bc\u200Bd\u202Ee\n"
	var zw, bidi int
	for _, d := range lint.Check(hiddenSkill(raw), lint.Options{}) {
		switch {
		case strings.Contains(d.Message, "hidden zero-width"):
			zw++
		case strings.Contains(d.Message, "hidden bidirectional"):
			bidi++
		}
	}
	if zw != 1 || bidi != 1 {
		t.Errorf("expected 1 zero-width and 1 bidi diagnostic, got %d and %d", zw, bidi)
	}
}
