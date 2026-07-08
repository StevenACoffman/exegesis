package mergedoc_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/mergedoc"
)

func TestParseNoSection(t *testing.T) {
	t.Parallel()
	got, err := mergedoc.Parse("# Skill\n\nbody with no merge status\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil entries, got %+v", got)
	}
}

func TestParseVerification(t *testing.T) {
	t.Parallel()
	md := "---\npair: inv-vs-control\ncheck: r-quote-accuracy\nsources:\n" +
		"  - book: munger\n    skill: inversion\n    status: accurate\n" +
		"  - book: aurelius\n    skill: control\n    status: drifted-minor\n    corrected: true\n" +
		"---\n\nnarrative for humans\n"
	sv, ok, err := mergedoc.ParseVerification(md)
	if err != nil || !ok {
		t.Fatalf("ParseVerification ok=%v err=%v", ok, err)
	}
	if sv.Pair != "inv-vs-control" || sv.Check != book2skill.CheckRQuoteAccuracy {
		t.Errorf("header = %+v", sv)
	}
	if len(sv.Sources) != 2 || !sv.Sources[1].Corrected ||
		sv.Sources[1].Status != book2skill.StatusDriftedMinor {
		t.Errorf("sources = %+v", sv.Sources)
	}
	if problems := sv.Validate(); len(problems) != 0 {
		t.Errorf("well-formed header should validate, got %v", problems)
	}

	// No frontmatter -> ok=false, no error.
	if _, ok, err := mergedoc.ParseVerification("# just a body\n"); ok || err != nil {
		t.Errorf("expected ok=false,nil for a headerless doc; got ok=%v err=%v", ok, err)
	}
}

func TestAppendCreatesSectionThenPreserves(t *testing.T) {
	t.Parallel()
	md := "---\nname: x\n---\n\n# Skill X\n\n## Provenance\n\n- **Source:** book\n"

	first := &book2skill.MergeStatusEntry{
		Run: "run-1", State: book2skill.StateMerged, Into: "merged-a",
	}
	out, err := mergedoc.Append(md, first)
	if err != nil {
		t.Fatalf("first Append: %v", err)
	}
	if !strings.Contains(out, "## Merge Status") || !strings.Contains(out, "```yaml") {
		t.Fatalf("section/fence not created:\n%s", out)
	}

	// A second append must preserve the first entry (append-only).
	second := &book2skill.MergeStatusEntry{
		Run: "run-2", State: book2skill.StateRejected,
		Pair: "p2", Reason: book2skill.ReasonV1Failed,
	}
	out, err = mergedoc.Append(out, second)
	if err != nil {
		t.Fatalf("second Append: %v", err)
	}

	entries, err := mergedoc.Parse(out)
	if err != nil {
		t.Fatalf("Parse round-trip: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0] != *first || entries[1] != *second {
		t.Errorf("entries not preserved: %+v", entries)
	}
	// The original body content must survive.
	if !strings.Contains(out, "## Provenance") {
		t.Errorf("original content lost:\n%s", out)
	}
}
