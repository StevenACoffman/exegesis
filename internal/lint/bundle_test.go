package lint_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/StevenACoffman/exegesis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/skill"
)

func TestCheckBundleBrokenLinks(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"SKILL.md":            {Data: []byte("x")},
		"references/guide.md": {Data: []byte("x")},
	}
	cases := map[string]struct {
		body    string
		wantSub string // "" = no finding
	}{
		"resolving link": {body: "See [g](references/guide.md)."},
		"broken link": {
			body:    "See [g](references/gone.md).",
			wantSub: "broken link 'references/gone.md'",
		},
		"external link":    {body: "See [g](https://example.com/gone.md)."},
		"link inside code": {body: "`[g](gone.md)`"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ds := blocking(
				lint.CheckBundle(fsys, &skill.Skill{Dir: "/x/s", Name: "s", Body: tc.body}),
			)
			if tc.wantSub == "" {
				if len(ds) != 0 {
					t.Errorf("want none, got %v", ds)
				}
				return
			}
			if len(ds) != 1 || !strings.Contains(ds[0].Message, tc.wantSub) ||
				!ds[0].Severity.Blocking() {
				t.Errorf("want one error containing %q, got %v", tc.wantSub, ds)
			}
		})
	}
}

func TestCheckBundleScansBundledFilesForHiddenCharacters(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"SKILL.md":                   {Data: []byte("a\u200Bb")}, // Check's, not CheckBundle's
		"methodology/00-overview.md": {Data: []byte("line one\nx\u200By\n")},
		"references/clean.md":        {Data: []byte("nothing hidden\n")},
		"assets/logo.png":            {Data: []byte{0x89, 'P', 'N', 'G', 0, 0}},
		".rumdl_cache/index.md":      {Data: []byte("x\u202Ey")},
	}
	ds := blocking(lint.CheckBundle(fsys, &skill.Skill{Dir: "/x/s", Name: "s"}))
	if len(ds) != 1 {
		t.Fatalf("want exactly the methodology hit, got %v", ds)
	}
	want := "hidden zero-width character at methodology/00-overview.md:2 (U+200B)"
	if !strings.Contains(ds[0].Message, want) || !ds[0].Severity.Blocking() {
		t.Errorf("got %q, want an error containing %q", ds[0].Message, want)
	}
}

// blocking keeps the error-severity diagnostics.
func blocking(ds []finding.Diagnostic) []finding.Diagnostic {
	var out []finding.Diagnostic
	for _, d := range ds {
		if d.Severity.Blocking() {
			out = append(out, d)
		}
	}
	return out
}
