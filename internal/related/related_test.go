package related_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/related"
)

func TestBulletExactFormat(t *testing.T) {
	t.Parallel()
	got := related.Bullet(
		related.Edge{Kind: related.DependsOn, Target: "other-skill", Rationale: "needs it first"},
	)
	want := "- depends-on: `other-skill` — needs it first"
	if got != want {
		t.Fatalf("Bullet = %q, want %q", got, want)
	}
}

func TestKindValid(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		kind related.Kind
		want bool
	}{
		"depends-on": {related.DependsOn, true},
		"contrasts":  {related.ContrastsWith, true},
		"composes":   {related.ComposesWith, true},
		"unknown":    {related.Kind("relates-to"), false},
		"empty":      {related.Kind(""), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := tc.kind.Valid(); got != tc.want {
				t.Errorf("Kind(%q).Valid() = %v, want %v", tc.kind, got, tc.want)
			}
		})
	}
}

func TestParseSection(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		md   string
		want []related.Edge
	}{
		"no section": {
			md:   "# Title\n\nbody only\n",
			want: nil,
		},
		"one edge": {
			md:   "# T\n\n## Related skills\n\n- depends-on: `a` — because\n",
			want: []related.Edge{{Kind: related.DependsOn, Target: "a", Rationale: "because"}},
		},
		"file order, all kinds": {
			md: "## Related skills\n\n" +
				"- depends-on: `a` — one\n" +
				"- contrasts-with: `b` — two\n" +
				"- composes-with: `c` — three\n",
			want: []related.Edge{
				{Kind: related.DependsOn, Target: "a", Rationale: "one"},
				{Kind: related.ContrastsWith, Target: "b", Rationale: "two"},
				{Kind: related.ComposesWith, Target: "c", Rationale: "three"},
			},
		},
		"skips unknown kind": {
			md:   "## Related skills\n\n- relates-to: `a` — nope\n- depends-on: `b` — yes\n",
			want: []related.Edge{{Kind: related.DependsOn, Target: "b", Rationale: "yes"}},
		},
		"stops at next heading": {
			md:   "## Related skills\n\n- depends-on: `a` — one\n\n## Notes\n\n- depends-on: `z` — ignored\n",
			want: []related.Edge{{Kind: related.DependsOn, Target: "a", Rationale: "one"}},
		},
		"ignores bullets in a fence": {
			md:   "## Related skills\n\n```\n- depends-on: `x` — fenced\n```\n- depends-on: `a` — real\n",
			want: []related.Edge{{Kind: related.DependsOn, Target: "a", Rationale: "real"}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := related.ParseSection(tc.md)
			if !equalEdges(got, tc.want) {
				t.Errorf("ParseSection = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestUpsertCreatesSection(t *testing.T) {
	t.Parallel()
	md := "---\nname: s\n---\n# Body\n\ntext\n"
	out, changed := related.Upsert(
		md,
		related.Edge{Kind: related.ComposesWith, Target: "t", Rationale: "why"},
	)
	if !changed {
		t.Fatal("expected changed=true when creating the section")
	}
	if !strings.Contains(out, "## Related skills") {
		t.Errorf("section heading missing:\n%s", out)
	}
	if !strings.HasPrefix(out, md[:len(md)-1]) { // original preserved (minus its trailing newline)
		t.Errorf("original content not preserved:\n%s", out)
	}
	got := related.ParseSection(out)
	if len(got) != 1 || got[0].Target != "t" {
		t.Errorf("round-trip failed: %#v", got)
	}
}

func TestUpsertIdempotent(t *testing.T) {
	t.Parallel()
	md := "## Related skills\n\n- depends-on: `a` — first\n"
	e := related.Edge{Kind: related.DependsOn, Target: "a", Rationale: "first"}
	out1, changed1 := related.Upsert(md, e)
	if changed1 {
		t.Fatalf("identical edge should be a no-op, got changed=true:\n%s", out1)
	}
	out2, changed2 := related.Upsert(out1, e)
	if changed2 || out2 != out1 {
		t.Errorf("Upsert is not idempotent: changed=%v", changed2)
	}
}

func TestUpsertUpdatesRationaleInPlace(t *testing.T) {
	t.Parallel()
	md := "## Related skills\n\n- depends-on: `a` — old reason\n"
	out, changed := related.Upsert(
		md,
		related.Edge{Kind: related.DependsOn, Target: "a", Rationale: "new reason"},
	)
	if !changed {
		t.Fatal("expected changed=true when the rationale differs")
	}
	if strings.Contains(out, "old reason") {
		t.Errorf("old rationale should be replaced:\n%s", out)
	}
	if strings.Count(out, "`a`") != 1 {
		t.Errorf("edge should be updated in place, not duplicated:\n%s", out)
	}
}

func TestUpsertAppendsToExistingSection(t *testing.T) {
	t.Parallel()
	md := "## Related skills\n\n- depends-on: `a` — one\n"
	out, changed := related.Upsert(
		md,
		related.Edge{Kind: related.ComposesWith, Target: "b", Rationale: "two"},
	)
	if !changed {
		t.Fatal("expected changed=true when appending a new edge")
	}
	got := related.ParseSection(out)
	if len(got) != 2 || got[1].Target != "b" {
		t.Errorf("append failed, edges = %#v", got)
	}
}

func equalEdges(a, b []related.Edge) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
