package quotecheck_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/quotecheck"
)

// Fixtures are deliberately wordy: a passage shorter than MinPassageWords is dropped
// before matching, so a terse fixture would exercise nothing at all.

func TestSegment(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{
		"intro prose",
		"## R — Original Text",
		"r content",
		"### a subsection",
		"still r content",
		"## I — Insight",
		"i content",
		"## A1 — Application",
		"a1 content",
	}, "\n")
	cases := map[string]struct{ want, wantNot string }{
		"R": {want: "r content", wantNot: "i content"},
		"I": {want: "i content", wantNot: "r content"},
	}
	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			got := quotecheck.Segment(body, label)
			if !strings.Contains(got, tc.want) {
				t.Errorf("Segment(%q) = %q, missing %q", label, got, tc.want)
			}
			if strings.Contains(got, tc.wantNot) {
				t.Errorf("Segment(%q) = %q, leaked %q from another segment", label, got, tc.wantNot)
			}
		})
	}
	// A "###" heading yields no label, so it is content rather than a boundary. If it
	// ended the segment, half of R would silently go unchecked.
	if got := quotecheck.Segment(body, "R"); !strings.Contains(got, "still r content") {
		t.Errorf("a subsection ended the segment early: %q", got)
	}
	if got := quotecheck.Segment(body, "B"); got != "" {
		t.Errorf("Segment for an absent label = %q, want empty", got)
	}
}

func TestPassages(t *testing.T) {
	t.Parallel()
	got := quotecheck.Passages(
		"The first sentence is plainly long enough. Too short. " +
			"The third sentence is also comfortably long enough to keep.")
	if len(got) != 2 {
		t.Fatalf("want 2 passages with the short one dropped, got %d: %q", len(got), got)
	}
	for _, p := range got {
		if n := len(strings.Fields(p)); n < quotecheck.MinPassageWords {
			t.Errorf("passage %q has %d words, under the %d minimum",
				p, n, quotecheck.MinPassageWords)
		}
	}
	// Over-splitting on an abbreviation is safe: the fragments fall under the minimum
	// and the substantive text is still checked.
	abbrev := quotecheck.Passages("Consider e.g. the case where the reader is misled badly")
	if len(abbrev) != 1 {
		t.Errorf("want the substantive remainder kept, got %q", abbrev)
	}
}

func TestCheckFindsAndMisses(t *testing.T) {
	t.Parallel()
	body := "## R\n\n> the unexamined life is not worth living for a human being.\n" +
		"> a line the author never once wrote down anywhere.\n"
	sources := []quotecheck.Source{
		{
			Name: "a.txt",
			Text: "and so the unexamined life is not worth living for a human being, he said",
		},
	}
	got := quotecheck.Check(body, "R", sources)
	if len(got) != 2 {
		t.Fatalf("want 2 findings, got %d: %+v", len(got), got)
	}
	if got[0].Missing() || got[0].FoundIn != "a.txt" {
		t.Errorf("a real passage was not located: %+v", got[0])
	}
	if !got[1].Missing() {
		t.Errorf("a fabricated passage was reported as found: %+v", got[1])
	}
}

func TestCheckToleratesTheDifferencesThatAreNotFabrication(t *testing.T) {
	t.Parallel()
	// A quotation is line-wrapped in Markdown and its source is not, and books use
	// typographic characters a plain-text extraction may not preserve. Without this
	// latitude every quotation would be reported missing and nobody would run the guard.
	cases := map[string]struct{ quote, source string }{
		"wrapped across lines": {
			quote:  "> the unexamined life is not\n> worth living for a human being",
			source: "the unexamined life is not worth living for a human being",
		},
		"curly apostrophe in the source": {
			quote:  "> it isn't the critic who counts in this arena",
			source: "it isn’t the critic who counts in this arena",
		},
		"curly apostrophe in the quotation": {
			quote:  "> it isn’t the critic who counts in this arena",
			source: "it isn't the critic who counts in this arena",
		},
		"em dash against a hyphen": {
			quote:  "> the two things — taken together at last here",
			source: "the two things - taken together at last here",
		},
		"non-breaking space": {
			quote:  "> a quoted phrase that is long enough here",
			source: "a quoted phrase that is long enough here",
		},
		"ellipsis character": {
			quote:  "> and so on… it continues for a while yet",
			source: "and so on... it continues for a while yet",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := quotecheck.Check("## R\n\n"+tc.quote+"\n", "R",
				[]quotecheck.Source{{Name: "s", Text: tc.source}})
			if len(got) != 1 {
				t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
			}
			if got[0].Missing() {
				t.Errorf("reported missing despite being present: %+v", got[0])
			}
		})
	}
}

func TestCheckStillCatchesRealFabrication(t *testing.T) {
	t.Parallel()
	// The latitude above must not collapse into "anything matches".
	got := quotecheck.Check("## R\n\n> a sentence the author never once wrote down\n", "R",
		[]quotecheck.Source{{Name: "s", Text: "a sentence the author did once write down"}})
	if len(got) != 1 || !got[0].Missing() {
		t.Errorf("normalization swallowed a real difference: %+v", got)
	}
}

func TestCheckCatchesOneFabricationInsideAFaithfulQuotation(t *testing.T) {
	t.Parallel()
	// The case per-passage matching exists for. Whole-quotation matching would condemn
	// the entire segment and say nothing about which sentence was invented.
	body := "## R\n\n> The first sentence is genuinely from the book. " +
		"An invented sentence slipped in right here. " +
		"The third sentence is genuinely from the book.\n"
	source := "The first sentence is genuinely from the book. " +
		"The third sentence is genuinely from the book."
	got := quotecheck.Check(body, "R", []quotecheck.Source{{Name: "s", Text: source}})
	if len(got) != 3 {
		t.Fatalf("want 3 passages, got %d: %+v", len(got), got)
	}
	missing := 0
	for _, f := range got {
		if f.Missing() {
			missing++
			if !strings.Contains(f.Passage, "invented") {
				t.Errorf("named the wrong passage as missing: %q", f.Passage)
			}
		}
	}
	if missing != 1 {
		t.Errorf("want exactly the invented passage missing, got %d of 3", missing)
	}
}

func TestCheckSearchesEverySource(t *testing.T) {
	t.Parallel()
	got := quotecheck.Check("## R\n\n> a passage taken from the second book here\n", "R",
		[]quotecheck.Source{
			{Name: "a.txt", Text: "unrelated"},
			{Name: "b.txt", Text: "a line: a passage taken from the second book here, yes"},
		})
	if len(got) != 1 || got[0].FoundIn != "b.txt" {
		t.Errorf("did not search past the first source: %+v", got)
	}
}

func TestCheckIgnoresQuotationsOutsideTheSegment(t *testing.T) {
	t.Parallel()
	// An illustrative blockquote in another segment is not a claim about a book, so
	// reporting it would train the reader to ignore the output.
	body := "## R\n\n> a real quote of sufficient length to check\n" +
		"\n## E — Example\n\n> an invented illustration of some length\n"
	got := quotecheck.Check(body, "R",
		[]quotecheck.Source{{Name: "s", Text: "a real quote of sufficient length to check"}})
	if len(got) != 1 {
		t.Fatalf("want only the R quotation, got %d: %+v", len(got), got)
	}
	if got[0].Missing() {
		t.Errorf("checked the wrong segment: %+v", got[0])
	}
}

func TestCheckWithNothingToCheck(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"no R segment":                "## I\n\n> a quote of sufficient length to check\n",
		"R segment with no quotation": "## R\n\nplain prose only\n",
		"quotation under the minimum": "## R\n\n> too short\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := quotecheck.Check(body, "R",
				[]quotecheck.Source{{Name: "s", Text: "x"}}); got != nil {
				t.Errorf("want no findings, got %+v", got)
			}
		})
	}
}

func TestCheckIgnoresQuotationsInsideCodeFences(t *testing.T) {
	t.Parallel()
	// redlines.Quotes strips fences, so a shell transcript is not a quotation. If it
	// were, every skill showing "> " prompt output would report a fabrication.
	body := "## R\n\n```\n> not a quotation but a shell transcript line\n```\n" +
		"\n> a real quote of sufficient length to check\n"
	got := quotecheck.Check(body, "R",
		[]quotecheck.Source{{Name: "s", Text: "a real quote of sufficient length to check"}})
	if len(got) != 1 || got[0].Missing() {
		t.Errorf("fenced content was treated as a quotation: %+v", got)
	}
}
