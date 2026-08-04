package related_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/related"
)

var update = flag.Bool("update", false, "update golden files")

// sampleNodes is a small tree exercising all three edge kinds and a two-level
// dependency chain, used by the golden render test.
func sampleNodes() []related.Node {
	return []related.Node{
		{Slug: "foundations", Title: "Foundations", Description: "the base ideas"},
		{
			Slug: "applying", Title: "Applying", Description: "put it to work",
			Edges: []related.Edge{
				{Kind: related.DependsOn, Target: "foundations", Rationale: "learn the base first"},
				{Kind: related.ComposesWith, Target: "measuring", Rationale: "use them together"},
			},
		},
		{
			Slug:        "measuring",
			Title:       "Measuring",
			Description: "check the results",
			Edges: []related.Edge{
				{Kind: related.ContrastsWith, Target: "applying", Rationale: "different lens"},
			},
		},
	}
}

func TestRenderGolden(t *testing.T) {
	t.Parallel()
	got := related.Render(
		related.Header{Title: "Demo Book", Author: "A. Author"},
		sampleNodes(),
		"",
	)
	golden := filepath.Join("testdata", "render.golden")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run: go test -update): %v", err)
	}
	if got != string(want) {
		t.Errorf("render mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderDeterministic(t *testing.T) {
	t.Parallel()
	h := related.Header{Title: "T"}
	first := related.Render(h, sampleNodes(), "")
	second := related.Render(h, sampleNodes(), "")
	if first != second {
		t.Error("Render is not deterministic")
	}
}

func TestRenderPreservesTail(t *testing.T) {
	t.Parallel()
	tail := "## Notes\n\nhand-written, keep me\n"
	out := related.Render(related.Header{Title: "T"}, sampleNodes(), tail)
	if !strings.HasSuffix(out, tail) {
		t.Errorf("preserved tail missing from output:\n%s", out)
	}
	// A regeneration that reads the tail back out must round-trip it.
	if got := related.Split(out); got != tail {
		t.Errorf("Split = %q, want %q", got, tail)
	}
}

func TestSplit(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   string
		want string
	}{
		"no markers is a fresh gen": {in: "# Hand written\n\nstuff\n", want: ""},
		"tail after end marker": {
			in:   "<!-- exegesis:index:start -->\nx\n<!-- exegesis:index:end -->\n\n## Notes\nkeep\n",
			want: "## Notes\nkeep\n",
		},
		"empty tail": {
			in:   "<!-- exegesis:index:start -->\nx\n<!-- exegesis:index:end -->\n",
			want: "",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := related.Split(tc.in); got != tc.want {
				t.Errorf("Split = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderCycleWarning(t *testing.T) {
	t.Parallel()
	nodes := []related.Node{
		{Slug: "a", Edges: []related.Edge{{Kind: related.DependsOn, Target: "b"}}},
		{Slug: "b", Edges: []related.Edge{{Kind: related.DependsOn, Target: "a"}}},
	}
	out := related.Render(related.Header{Title: "T"}, nodes, "")
	if !strings.Contains(out, "cycle among: a, b") {
		t.Errorf("expected a cycle warning in the learning path:\n%s", out)
	}
}
