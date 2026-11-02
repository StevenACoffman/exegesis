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

func TestCheckBundleUnreferencedFiles(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"SKILL.md":                   {Data: []byte("x")},
		"references/linked.md":       {Data: []byte("See `references/deeper.md`.\n")},
		"references/deeper.md":       {Data: []byte("leaf\n")},
		"references/orphan.md":       {Data: []byte("nobody links me\n")},
		"methodology/00-overview.md": {Data: []byte("then [next](01-next.md)\n")},
		"methodology/01-next.md":     {Data: []byte("relative to its own folder\n")},
		"scripts/current-state.sh":   {Data: []byte("#!/bin/sh\n")},
		"templates/a.md":             {Data: []byte("x\n")},
		"test-prompts.json":          {Data: []byte("{}")},
		"test-results.md":            {Data: []byte("report\n")},
	}
	body := "Read [linked](references/linked.md), then `methodology/00-overview.md`.\n" +
		"Run `sh scripts/current-state.sh` from the repository root. Templates live in `templates/`.\n"
	got := findingsContaining(
		lint.CheckBundle(fsys, &skill.Skill{Dir: "/x/s", Name: "s", Body: body}),
		"bundle:",
	)
	if len(got) != 1 || !strings.Contains(got[0].Message, "references/orphan.md is not linked") ||
		got[0].Severity.Blocking() {
		t.Errorf("want one warning for references/orphan.md, got %v", got)
	}
}

func TestCheckBundleCrossSkillPaths(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		body    string
		wantSub string // "" = no cross-skill finding
	}{
		"home path into another skill": {
			body:    "Exceptions: `~/Documents/agent-orange/.../skills/webapp/RULES.md`\n",
			wantSub: `a path into skill "webapp"'s directory`,
		},
		"absolute path into another skill": {
			body:    "cat /opt/skills/other/notes.md\n\n`cat /opt/skills/other/notes.md`\n",
			wantSub: `skill "other"`,
		},
		"relative path into a sibling": {
			body:    "`../sibling/SKILL.md`\n",
			wantSub: `skill "sibling"`,
		},
		"own directory by absolute path": {body: "`~/x/skills/s/references/a.md`\n"},
		"unrelated absolute path":        {body: "`/etc/hosts`\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{"SKILL.md": {Data: []byte("x")}}
			got := findingsContaining(
				lint.CheckBundle(
					fsys,
					&skill.Skill{Dir: "/x/s", Name: "s", Body: tc.body},
				),
				"a path into skill",
			)
			if tc.wantSub == "" {
				if len(got) != 0 {
					t.Errorf("want none, got %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0].Message, tc.wantSub) ||
				got[0].Severity.Blocking() {
				t.Errorf("want one warning containing %q, got %v", tc.wantSub, got)
			}
		})
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

func TestCheckBundleResolvesFromTheReferencingFile(t *testing.T) {
	t.Parallel()
	// A reference file linking back to its own skill's SKILL.md climbs one level from
	// references/, which is still this skill. Resolving from the skill root instead read
	// it as a path into a skill named "SKILL.md#quick-start".
	fsys := fstest.MapFS{
		"SKILL.md": {
			Data: []byte("See [a](references/a.md) and [b](references/b.md).\n"),
		},
		"references/a.md": {
			Data: []byte("# A\n\nBack to [quick start](../SKILL.md#quick-start).\n"),
		},
		"references/b.md": {
			Data: []byte("# B\n\nSee [sibling](../../sibling/SKILL.md#usage).\n"),
		},
		"references/c.md":      {Data: []byte("# C\n\nNot [linked](../SKILL.md) anywhere else.\n")},
		"references/unused.md": {Data: []byte("# U\n")},
	}
	got := findingsContaining(
		lint.CheckBundle(fsys, &skill.Skill{
			Dir: "/x/s", Name: "s", Body: "See [a](references/a.md) and [b](references/b.md).\n",
		}),
		"a path into skill",
	)
	if len(got) != 1 || !strings.Contains(got[0].Message, `skill "sibling"`) {
		t.Errorf("want one warning naming sibling, got %v", got)
	}
}
