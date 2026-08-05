package relate_test

import (
	"testing"

	"github.com/StevenACoffman/exegesis/internal/relate"
	"github.com/StevenACoffman/exegesis/internal/related"
)

func TestParseGroupsBySourceSorted(t *testing.T) {
	t.Parallel()
	data := []byte(`{"edges":[
	  {"from":"Widget Maker","kind":"depends-on","to":"Widget Parts","rationale":"needs parts"},
	  {"from":"apples","kind":"contrasts-with","to":"oranges","rationale":"different"},
	  {"from":"Widget Maker","kind":"composes-with","to":"paint","rationale":"then paint"}
	]}`)
	groups, err := relate.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}
	// Sorted by slug: "apples" before "widget-maker".
	if groups[0].Slug != "apples" || groups[1].Slug != "widget-maker" {
		t.Fatalf("groups not sorted by slug: %q, %q", groups[0].Slug, groups[1].Slug)
	}
	wm := groups[1]
	if len(wm.Edges) != 2 {
		t.Fatalf("widget-maker should have 2 edges, got %d", len(wm.Edges))
	}
	if wm.Edges[0].Kind != related.DependsOn || wm.Edges[0].Target != "widget-parts" {
		t.Errorf("edge not parsed/normalized: %+v", wm.Edges[0])
	}
}

func TestParseRejectsBadRows(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"unknown kind":    `{"edges":[{"from":"a","kind":"bogus","to":"b","rationale":"x"}]}`,
		"empty to":        `{"edges":[{"from":"a","kind":"depends-on","to":"","rationale":"x"}]}`,
		"empty rationale": `{"edges":[{"from":"a","kind":"depends-on","to":"b","rationale":""}]}`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := relate.Parse([]byte(data)); err == nil {
				t.Errorf("expected an error for %s", name)
			}
		})
	}
}
