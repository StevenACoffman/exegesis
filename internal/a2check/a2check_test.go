package a2check_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/a2check"
)

// body wraps signals in the A2 segment of a minimal skill body.
func body(signals ...string) string {
	var b strings.Builder
	b.WriteString("## R — Reading\n\n> a quotation that is not a signal at all\n\n")
	b.WriteString("## A2 — Trigger Scenario\n\n### Language Signals\n\n")
	for _, s := range signals {
		b.WriteString("- \"" + s + "\"\n")
	}
	b.WriteString("\n## E — Execution\n\n\"a phrase outside A2 that must not count\"\n")
	return b.String()
}

func TestSignals(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		body string
		want []string
	}{
		"reads the quoted phrases in A2": {
			body: body("we need five nines of availability", "how much reliability is enough"),
			want: []string{"we need five nines of availability", "how much reliability is enough"},
		},
		"ignores quotations outside A2": {
			body: body("we need five nines of availability"),
			want: []string{"we need five nines of availability"},
		},
		"drops a run too short to be a phrase": {
			body: body("we need five nines of availability", "merged"),
			want: []string{"we need five nines of availability"},
		},
		"deduplicates a signal stated twice": {
			body: body("we need five nines of availability", "we need five nines of availability"),
			want: []string{"we need five nines of availability"},
		},
		"a skill with no A2 states nothing": {
			body: "## R — Reading\n\n\"a quotation in the wrong segment entirely\"\n",
			want: nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := a2check.Signals(tc.body)
			if len(got) != len(tc.want) {
				t.Fatalf("Signals = %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("signal %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSignalsReadsABoldLabelledListToo(t *testing.T) {
	t.Parallel()
	// Only 10 of the 27 real merged skills use a "### Language Signals" heading; the
	// rest write a bold label or nothing at all, which is why the reader matches the
	// quotation rather than the subsection.
	in := "## A2 — Trigger\n\n**Language signals:**\n\n" +
		"- \"we should just ship it and clean it up later\"\n"
	got := a2check.Signals(in)
	if len(got) != 1 || got[0] != "we should just ship it and clean it up later" {
		t.Errorf("Signals = %q, want the bold-labelled signal", got)
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	sourceA := a2check.Signals(body("we need five nines of availability"))
	sourceB := a2check.Signals(body("how much reliability is enough for us"))
	cases := map[string]struct {
		merged string
		want   int
	}{
		"repeating both sources adds nothing": {
			merged: "we need five nines of availability|how much reliability is enough for us",
			want:   0,
		},
		"one new signal is not enough on its own": {
			merged: "we need five nines of availability|the budget is exhausted again",
			want:   1,
		},
		"two new signals clear the bar": {
			merged: "the budget is exhausted again|nobody agreed to this freeze policy",
			want:   2,
		},
		"a difference of case and punctuation is not novelty": {
			merged: "We need five nines of availability.|How much reliability is enough for us?",
			want:   0,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			merged := a2check.Signals(body(strings.Split(tc.merged, "|")...))
			got := a2check.New(merged, [][]string{sourceA, sourceB})
			if len(got) != tc.want {
				t.Errorf("New = %q (%d), want %d new", got, len(got), tc.want)
			}
		})
	}
}
