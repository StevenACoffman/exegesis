package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/cmd/root"
)

// The two sentences are long enough to survive quotecheck's MinPassageWords filter,
// which is what makes them two passages rather than none.
const (
	firstPassage  = "The first sentence is genuinely taken from the book"
	secondPassage = "The second sentence is genuinely taken from the book too"
)

// writeQuotingSkill writes a skill whose R segment is one blockquote holding quote --
// the shape 95% of the real corpus has -- and returns its directory.
func writeQuotingSkill(t *testing.T, tree, name, quote string) string {
	t.Helper()
	dir := filepath.Join(tree, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "---\nname: " + name + "\n" +
		"description: Invoke when the user needs a demo thing done in a particular way.\n" +
		"---\n# Body\n\n## R — Original Text\n\n" + quote + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", dir, err)
	}
	return dir
}

// writeSourceText writes a plain-text source file and returns its path.
func writeSourceText(t *testing.T, tree, name, text string) string {
	t.Helper()
	path := filepath.Join(tree, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestQuotecheckMinSupport(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	source := writeSourceText(t, tree, "book.txt",
		firstPassage+". "+secondPassage+".")
	twoPassages := writeQuotingSkill(t, tree, "two-passages",
		"> "+firstPassage+". "+secondPassage+".")
	// A skill that quotes nothing: the case a threshold must fail rather than wave
	// through, since a skill with no quotations has no located evidence at all.
	noQuotations := writeQuotingSkill(t, tree, "no-quotations", "plain prose only.")

	cases := map[string]struct {
		args     []string
		wantFail bool
		wantMsg  string
	}{
		"threshold met": {
			args:    []string{"--min-support", "2", twoPassages},
			wantMsg: "2/2 passages located",
		},
		"threshold unmet": {
			args:     []string{"--min-support", "3", twoPassages},
			wantFail: true,
			wantMsg:  "SUPPORT 2 located, --min-support 3",
		},
		"no quotations at all": {
			args:     []string{"--min-support", "1", noQuotations},
			wantFail: true,
			wantMsg:  "SUPPORT 0 located, --min-support 1",
		},
		"no threshold passes a skill that quotes nothing": {
			args:    []string{noQuotations},
			wantMsg: "no quotations of at least",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"quotecheck", "--source-text", source}, tc.args...)
			out, err := run(t, args...)
			var exit root.ExitError
			switch {
			case tc.wantFail && !errors.As(err, &exit):
				t.Fatalf("expected a non-zero exit, got %v\n%s", err, out)
			case !tc.wantFail && err != nil:
				t.Fatalf("expected success, got %v\n%s", err, out)
			}
			if !strings.Contains(out, tc.wantMsg) {
				t.Errorf("expected %q in the report, got:\n%s", tc.wantMsg, out)
			}
		})
	}
}

func TestQuotecheckMinSupportCountsOnlyLocatedPassages(t *testing.T) {
	t.Parallel()
	// Support is the point of the flag: a skill can quote plenty and still support
	// nothing, so the threshold must count located passages, not checked ones.
	tree := t.TempDir()
	source := writeSourceText(t, tree, "book.txt", firstPassage+".")
	dir := writeQuotingSkill(t, tree, "half-invented",
		"> "+firstPassage+". An invented sentence that is nowhere in the book.")

	out, err := run(t, "quotecheck", "--source-text", source, "--min-support", "2", dir)
	var exit root.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected a non-zero exit, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "SUPPORT 1 located, --min-support 2") {
		t.Errorf("expected the located count to exclude the missing passage, got:\n%s", out)
	}
	if !strings.Contains(out, "MISS") {
		t.Errorf("expected the fabricated passage to still be named, got:\n%s", out)
	}
}

func TestQuotecheckRejectsANegativeMinSupport(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	source := writeSourceText(t, tree, "book.txt", firstPassage+".")
	dir := writeQuotingSkill(t, tree, "any-skill", "> "+firstPassage+".")

	_, err := run(t, "quotecheck", "--source-text", source, "--min-support", "-1", dir)
	if err == nil || !strings.Contains(err.Error(), "--min-support cannot be negative") {
		t.Errorf("expected a usage error for a negative threshold, got %v", err)
	}
}
