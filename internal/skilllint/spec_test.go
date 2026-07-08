package skilllint_test

import (
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint"
)

func TestCheckSpec(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		frontmatter string
		body        string
		wantIDs     []string
		absentIDs   []string
	}{
		"valid minimal": {
			frontmatter: "name: good-skill\ndescription: Use when you need a demonstrably valid skill body here.",
			body:        "# Heading\n\ninstructions",
			absentIDs:   []string{"1b.name.missing", "1b.description.missing", "1b.name.format"},
		},
		"missing name": {
			frontmatter: "description: Use when you need something.",
			wantIDs:     []string{"1b.name.missing"},
		},
		"uppercase name (fixable)": {
			frontmatter: "name: Good-Skill\ndescription: Use when you need something here now.",
			wantIDs:     []string{"1b.name.format", "1b.name.dir-match"},
		},
		"consecutive hyphens": {
			frontmatter: "name: bad--name\ndescription: Use when you need something here now.",
			wantIDs:     []string{"1b.name.consecutive-hyphens", "1b.name.dir-match"},
		},
		"unknown field": {
			frontmatter: "name: s\ndescription: Use when you need something here now.\nbogus: 1",
			wantIDs:     []string{"1d.unknown-field"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), dirNameFor(name))
			body := tc.body
			if body == "" {
				body = "# H\n\ntext"
			}
			writeSkill(t, dir, "---\n"+tc.frontmatter+"\n---\n"+body+"\n")
			ids := checkIDs(skilllint.CheckSpec(skilllint.Parse(dir), nil))
			assertIDs(t, ids, tc.wantIDs, tc.absentIDs)
		})
	}
}

func assertIDs(t *testing.T, ids map[string]bool, want, absent []string) {
	t.Helper()
	for _, id := range want {
		if !ids[id] {
			t.Errorf("want %q to fire; got %v", id, keysOf(ids))
		}
	}
	for _, id := range absent {
		if ids[id] {
			t.Errorf("did not expect %q to fire", id)
		}
	}
}

func TestCheckSpecParseErrors(t *testing.T) {
	t.Parallel()
	missing := skilllint.Parse(filepath.Join(t.TempDir(), "nope"))
	if !checkIDs(skilllint.CheckSpec(missing, nil))["1a.presence"] {
		t.Error("missing SKILL.md should yield 1a.presence")
	}
}

func TestCheckCrossSkillDuplicateName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	writeSkill(t, a, "---\nname: dup\ndescription: Use when A.\n---\n# H\ntext\n")
	writeSkill(t, b, "---\nname: dup\ndescription: Use when B.\n---\n# H\ntext\n")
	diags := skilllint.CheckCrossSkill([]*skilllint.Skill{skilllint.Parse(a), skilllint.Parse(b)})
	if !checkIDs(diags)["1g.duplicate-name"] {
		t.Errorf("expected 1g.duplicate-name, got %v", diags)
	}
}

// dirNameFor keeps a valid-slug directory so dir-match noise is predictable.
func dirNameFor(testName string) string {
	switch testName {
	case "valid minimal":
		return "good-skill"
	default:
		return "s"
	}
}
