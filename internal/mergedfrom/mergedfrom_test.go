package mergedfrom_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/mergedfrom"
	"github.com/StevenACoffman/skillet/related"
)

func TestParse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		frontmatter string
		want        mergedfrom.Declaration
	}{
		{"absent", "name: m\n", mergedfrom.Declaration{}},
		{"other metadata only", "name: m\nmetadata:\n  lineage: book-derived\n", mergedfrom.Declaration{}},
		{
			"two sources", "name: m\nmetadata:\n  merged-from: a  b\n",
			mergedfrom.Declaration{Declared: true, Sources: []string{"a", "b"}},
		},
		{
			// speclint rejects the list; it still declares a merge, so the skill is not
			// silently gated as an ordinary one while the author fixes the value.
			"a list is declared but names nothing", "name: m\nmetadata:\n  merged-from: [a, b]\n",
			mergedfrom.Declaration{Declared: true},
		},
		{"empty value", "name: m\nmetadata:\n  merged-from: \"\"\n", mergedfrom.Declaration{Declared: true}},
		{"unparseable", "name: [\n", mergedfrom.Declaration{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mergedfrom.Parse(tc.frontmatter)
			if got.Declared != tc.want.Declared ||
				!reflect.DeepEqual(got.Sources, tc.want.Sources) {
				t.Errorf("Parse = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestProblems(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		sources []string
		want    string // substring of the one problem expected, "" for none
	}{
		{"valid", []string{"a", "b-c"}, ""},
		{"empty", nil, "names no source skills"},
		{"invalid name", []string{"a", "Bad_Name"}, `"Bad_Name" is not a valid skill name`},
		{"itself", []string{"m", "a"}, "names the skill itself"},
		{"twice", []string{"a", "a"}, `"a" is listed twice`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mergedfrom.Declaration{Declared: true, Sources: tc.sources}.Problems("m")
			switch {
			case tc.want == "" && len(got) != 0:
				t.Errorf("want no problems, got %q", got)
			case tc.want != "" && (len(got) != 1 || !strings.Contains(got[0], tc.want)):
				t.Errorf("want one problem containing %q, got %q", tc.want, got)
			}
		})
	}
	if got := (mergedfrom.Declaration{}).Problems("m"); got != nil {
		t.Errorf("an undeclared skill has no problems, got %q", got)
	}
}

func TestResolveRefusesToGuess(t *testing.T) {
	t.Parallel()
	subs := mergedfrom.Subsumers(map[string]mergedfrom.Declaration{
		"m1": {Declared: true, Sources: []string{"a", "shared"}},
		"m2": {Declared: true, Sources: []string{"b", "shared"}},
	})
	if to, ok := mergedfrom.Resolve(subs, "a"); !ok || to != "m1" {
		t.Errorf("Resolve(a) = %q, %t; want m1, true", to, ok)
	}
	if to, ok := mergedfrom.Resolve(subs, "shared"); ok {
		t.Errorf("Resolve(shared) = %q; two claimants must not resolve", to)
	}
	if !reflect.DeepEqual(subs["shared"], []string{"m1", "m2"}) {
		t.Errorf("subsumers[shared] = %q, want [m1 m2]", subs["shared"])
	}
	if _, ok := mergedfrom.Resolve(subs, "nobody"); ok {
		t.Error("an undeclared target must not resolve")
	}
}

func TestRedirect(t *testing.T) {
	t.Parallel()
	subs := map[string][]string{"gone": {"merged"}, "kept": {"merged"}, "src": {"merged"}}
	nodes := []related.Node{
		{Slug: "merged", Edges: []related.Edge{
			{Kind: related.DependsOn, Target: "src"}, // the merge itself: dropped
		}},
		{Slug: "kept"},
		{Slug: "user", Edges: []related.Edge{
			{Kind: related.DependsOn, Target: "gone", Rationale: "first"},
			{Kind: related.DependsOn, Target: "merged", Rationale: "duplicate once redirected"},
			{Kind: related.DependsOn, Target: "kept"}, // present, so it means itself
			{Kind: related.DependsOn, Target: "ghost"},
		}},
	}
	got := mergedfrom.Redirect(nodes, subs)
	if len(got[0].Edges) != 0 {
		t.Errorf("a merged skill's edge to its own source must be dropped, got %+v", got[0].Edges)
	}
	want := []related.Edge{
		{Kind: related.DependsOn, Target: "merged", Rationale: "first"},
		{Kind: related.DependsOn, Target: "kept"},
		{Kind: related.DependsOn, Target: "ghost"},
	}
	if !reflect.DeepEqual(got[2].Edges, want) {
		t.Errorf("Redirect edges = %+v\nwant %+v", got[2].Edges, want)
	}
	if nodes[2].Edges[0].Target != "gone" {
		t.Error("Redirect must not mutate its input")
	}
}
