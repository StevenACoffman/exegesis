package skilllint_test

import (
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint"
)

func TestSlugifyHeading(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Hello World":        "hello-world",
		"`code` and text":    "code-and-text",
		"A — B":              "a--b", // em dash removed leaves two spaces -> two hyphens
		"[link](x) here":     "link-here",
		"Punctuation! (yes)": "punctuation-yes",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if got := skilllint.SlugifyHeading(in); got != want {
				t.Errorf("SlugifyHeading(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestExtractHeadingsDuplicates(t *testing.T) {
	t.Parallel()
	md := "# Intro\n\ntext\n\n## Intro\n\nmore\n\n## Intro\n"
	got := skilllint.ExtractHeadings(md)
	for _, want := range []string{"intro", "intro-1", "intro-2"} {
		if !got[want] {
			t.Errorf("missing heading slug %q in %v", want, got)
		}
	}
}

func TestExtractHeadingsIgnoresCodeBlocks(t *testing.T) {
	t.Parallel()
	md := "# Real\n\n```\n# Fake heading in code\n```\n"
	got := skilllint.ExtractHeadings(md)
	if got["fake-heading-in-code"] {
		t.Error("heading inside a code block should be ignored")
	}
	if !got["real"] {
		t.Error("real heading missing")
	}
}

func TestExtractHeadingsSetextDisambiguation(t *testing.T) {
	t.Parallel()
	// A genuine Setext H2 (paragraph line immediately over dashes) is a heading.
	// A list item over a line of dashes is NOT: the dashes are a thematic break,
	// which the old regex mistook for a Setext underline, minting a phantom
	// "#item-one" anchor that made broken fragment links falsely pass.
	md := "Real Title\n---\n\n- item one\n----\n\n## Actual\n"
	got := skilllint.ExtractHeadings(md)
	if !got["real-title"] {
		t.Errorf("genuine Setext H2 missing; got %v", got)
	}
	if !got["actual"] {
		t.Errorf("ATX heading missing; got %v", got)
	}
	if got["item-one"] {
		t.Errorf("list item over dashes must not be a heading; got %v", got)
	}
}

func TestExtractLinksNestedBrackets(t *testing.T) {
	t.Parallel()
	// A link label containing brackets — the old `\[([^\]]*)\]` regex stops at the
	// first "]" and mis-parses the destination.
	md := "See [a [b] c](references/g.md) for details.\n"
	targets := skilllint.ExtractLocalLinkTargets(md)
	if len(targets) != 1 || targets[0] != "references/g.md" {
		t.Errorf("nested-bracket link target = %v, want [references/g.md]", targets)
	}
}

func TestExtractLinks(t *testing.T) {
	t.Parallel()
	md := "See [guide](references/guide.md) and [ext](https://x) and [frag](#section) " +
		"and [both](references/g.md#part).\n"
	targets := skilllint.ExtractLocalLinkTargets(md)
	if len(targets) != 2 || targets[0] != "references/guide.md" || targets[1] != "references/g.md" {
		t.Errorf("local targets = %v, want [references/guide.md references/g.md]", targets)
	}
	frags := skilllint.ExtractFragmentLinks(md)
	if len(frags) != 2 {
		t.Fatalf("fragment links = %v, want 2", frags)
	}
	if frags[0].Path != "" || frags[0].Fragment != "section" {
		t.Errorf("frag[0] = %+v, want {Path: '', Fragment: section}", frags[0])
	}
	if frags[1].Path != "references/g.md" || frags[1].Fragment != "part" {
		t.Errorf("frag[1] = %+v", frags[1])
	}
}

func TestFindUnclosedFence(t *testing.T) {
	t.Parallel()
	closed := "text\n```\ncode\n```\nmore\n"
	if _, open := skilllint.FindUnclosedFence(closed); open {
		t.Error("balanced fences reported as unclosed")
	}
	unclosed := "text\n```go\ncode never closed\n"
	line, open := skilllint.FindUnclosedFence(unclosed)
	if !open || line != 2 {
		t.Errorf("FindUnclosedFence = (%d, %v), want (2, true)", line, open)
	}
}
