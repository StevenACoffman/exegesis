package lint_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/lint"
	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/skill"
)

func TestSizeGate(t *testing.T) {
	t.Parallel()
	lines := func(n int) string { return strings.Repeat("x\n", n) }
	cases := map[string]struct {
		raw      string
		max      int
		ceilings map[string]int
		wantSev  finding.Severity // "" = no size finding
		wantSub  string
	}{
		"at the limit": {raw: lines(500), max: 500},
		"over the limit": {
			raw:     lines(501),
			max:     500,
			wantSev: finding.SeverityError,
			wantSub: "501 lines > 500",
		},
		"no trailing newline": {
			raw:     lines(500) + "x",
			max:     500,
			wantSev: finding.SeverityError,
			wantSub: "501 lines",
		},
		"gate off": {raw: lines(900), max: 0},
		"exception at ceiling": {
			raw:      lines(600),
			max:      500,
			ceilings: map[string]int{"s": 600},
			wantSev:  finding.SeverityWarning,
			wantSub:  "known exception at its ceiling",
		},
		"exception shrank": {
			raw:      lines(550),
			max:      500,
			ceilings: map[string]int{"s": 600},
			wantSev:  finding.SeverityWarning,
			wantSub:  "lower its ceiling from 600 to 550",
		},
		"exception grew": {
			raw:      lines(601),
			max:      500,
			ceilings: map[string]int{"s": 600},
			wantSev:  finding.SeverityError,
			wantSub:  "past its recorded ceiling of 600",
		},
		"exception now in limit": {
			raw:      lines(400),
			max:      500,
			ceilings: map[string]int{"s": 600},
			wantSev:  finding.SeverityWarning,
			wantSub:  "remove its line_ceilings entry",
		},
		"another skill's ceiling": {
			raw:      lines(501),
			max:      500,
			ceilings: map[string]int{"other": 900},
			wantSev:  finding.SeverityError,
			wantSub:  "501 lines > 500",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := &skill.Skill{Dir: "/x/s", Name: "s", Description: "d", Raw: tc.raw}
			got := findingsContaining(
				lint.Check(s, lint.Options{MaxLines: tc.max, LineCeilings: tc.ceilings}),
				"size:",
			)
			wantCount := 0
			if tc.wantSev != "" {
				wantCount = 1
			}
			if len(got) != wantCount {
				t.Fatalf("got %v, want severity %q containing %q", got, tc.wantSev, tc.wantSub)
			}
			if len(got) == 1 &&
				(got[0].Severity != tc.wantSev || !strings.Contains(got[0].Message, tc.wantSub)) {
				t.Errorf("got %v, want one %s containing %q", got, tc.wantSev, tc.wantSub)
			}
		})
	}
}
