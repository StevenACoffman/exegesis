package skilllint_test

import (
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint"
)

// TestSkillscheckParity runs the checks over skillscheck's own fixtures and
// asserts the same check IDs fire, locking this port to the reference tool.
func TestSkillscheckParity(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		quality   bool // run CheckQuality instead of CheckSpec
		want      []string
		wantClean bool // no error-level diagnostics
	}{
		"bad-name-mismatch":  {want: []string{"1b.name.dir-match"}},
		"bad-name-uppercase": {want: []string{"1b.name.format", "1b.name.dir-match"}},
		"bad-name-consecutive": {
			want: []string{"1b.name.consecutive-hyphens", "1b.name.dir-match"},
		},
		"missing-description": {want: []string{"1b.description.missing"}},
		"unknown-fields":      {want: []string{"1d.unknown-field"}},
		"allowed-tools-list":  {want: []string{"1c.allowed-tools.list-form"}},
		"bad-frontmatter":     {want: []string{"1a.frontmatter"}},
		"valid-minimal":       {wantClean: true},
		"valid-full":          {wantClean: true},
		"broken-link":         {quality: true, want: []string{"2c.broken-link"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			skill := skilllint.Parse(filepath.Join("testdata", "skillscheck", name))
			diags := skilllint.CheckSpec(skill, nil)
			if tc.quality {
				diags = skilllint.CheckQuality(skill, nil)
			}
			assertIDs(t, checkIDs(diags), tc.want, nil)
			if tc.wantClean && countErrors(diags) != 0 {
				t.Errorf("%s: expected no spec errors, got %v", name, keysOf(checkIDs(diags)))
			}
		})
	}
}

func countErrors(diags []skilllint.Diagnostic) int {
	n := 0
	for _, d := range diags {
		if d.Level == skilllint.LevelError {
			n++
		}
	}
	return n
}
