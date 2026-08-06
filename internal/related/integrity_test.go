package related_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/related"
)

func TestDanglingEdges(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		nodes []related.Node
		want  []related.DanglingEdge
	}{
		"no nodes": {},
		"no edges": {
			nodes: []related.Node{{Slug: "a"}, {Slug: "b"}},
		},
		"every target known": {
			nodes: []related.Node{
				{Slug: "a", Edges: []related.Edge{dep("b")}},
				{Slug: "b"},
			},
		},
		"unknown target is reported": {
			nodes: []related.Node{{Slug: "a", Edges: []related.Edge{dep("ghost")}}},
			want: []related.DanglingEdge{
				{Source: "a", Edge: dep("ghost")},
			},
		},
		"report is sorted by source then kind then target": {
			nodes: []related.Node{
				{Slug: "b", Edges: []related.Edge{dep("z"), dep("y")}},
				{Slug: "a", Edges: []related.Edge{
					{Kind: related.DependsOn, Target: "x", Rationale: "r"},
					{Kind: related.ComposesWith, Target: "w", Rationale: "r"},
				}},
			},
			want: []related.DanglingEdge{
				{Source: "a", Edge: related.Edge{
					Kind: related.ComposesWith, Target: "w", Rationale: "r",
				}},
				{Source: "a", Edge: dep("x")},
				{Source: "b", Edge: dep("y")},
				{Source: "b", Edge: dep("z")},
			},
		},
		"non-depends-on kinds dangle too": {
			// Mermaid drops contrasts-with edges to unknown targets just the same,
			// so the report must not be limited to the kinds that order the path.
			nodes: []related.Node{{Slug: "a", Edges: []related.Edge{
				{Kind: related.ContrastsWith, Target: "ghost", Rationale: "r"},
			}}},
			want: []related.DanglingEdge{
				{Source: "a", Edge: related.Edge{
					Kind: related.ContrastsWith, Target: "ghost", Rationale: "r",
				}},
			},
		},
		"self edge is known": {
			nodes: []related.Node{{Slug: "a", Edges: []related.Edge{dep("a")}}},
		},
		"duplicate node slugs still resolve": {
			nodes: []related.Node{
				{Slug: "a", Edges: []related.Edge{dep("b")}},
				{Slug: "b"},
				{Slug: "b"},
			},
		},
		"empty target is unknown": {
			nodes: []related.Node{{Slug: "a", Edges: []related.Edge{dep("")}}},
			want: []related.DanglingEdge{
				{Source: "a", Edge: dep("")},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := related.DanglingEdges(tc.nodes)
			if len(got) != len(tc.want) {
				t.Fatalf("DanglingEdges = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("edge %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestUnknownSlugs(t *testing.T) {
	t.Parallel()
	nodes := []related.Node{{Slug: "a"}, {Slug: "b"}}
	cases := map[string]struct {
		want []string
		out  []string
	}{
		"nothing wanted": {},
		"all known":      {want: []string{"a", "b"}},
		"one unknown":    {want: []string{"a", "ghost"}, out: []string{"ghost"}},
		"duplicates collapse": {
			want: []string{"ghost", "ghost", "a"},
			out:  []string{"ghost"},
		},
		"several unknowns are sorted": {
			want: []string{"zeta", "alpha", "b"},
			out:  []string{"alpha", "zeta"},
		},
		"empty slug is unknown": {want: []string{""}, out: []string{""}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := related.UnknownSlugs(nodes, tc.want)
			if len(got) != len(tc.out) {
				t.Fatalf("UnknownSlugs(%q) = %q, want %q", tc.want, got, tc.out)
			}
			for i := range got {
				if got[i] != tc.out[i] {
					t.Errorf("slug %d = %q, want %q", i, got[i], tc.out[i])
				}
			}
		})
	}
}

func TestDanglingEdgesEmptyForRenderableGraph(t *testing.T) {
	t.Parallel()
	// The Ensures contract: no dangling edges iff Mermaid renders every edge.
	nodes := []related.Node{
		{Slug: "a", Title: "A", Edges: []related.Edge{dep("b"), dep("ghost")}},
		{Slug: "b", Title: "B"},
	}
	if got := related.DanglingEdges(nodes); len(got) != 1 {
		t.Fatalf("expected exactly the ghost edge, got %+v", got)
	}
	mermaid := related.Mermaid(nodes)
	if want := "  a -->|depends-on| b\n"; !strings.Contains(mermaid, want) {
		t.Errorf("Mermaid missing the known edge %q:\n%s", want, mermaid)
	}
	if strings.Contains(mermaid, "ghost") {
		t.Errorf("Mermaid rendered the dangling edge; the report would be wrong:\n%s", mermaid)
	}
}
