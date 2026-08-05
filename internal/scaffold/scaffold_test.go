package scaffold_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/scaffold"
	"github.com/StevenACoffman/skillet/testprompts"
)

func TestRenderSkillHasFrontmatterAndSixSegments(t *testing.T) {
	t.Parallel()
	got, err := scaffold.RenderSkill(&scaffold.Skill{
		Slug:        "Widget Maker",
		Description: "Use this when the user asks to build a widget.",
		Related: []scaffold.Edge{
			{Kind: "depends-on", Target: "widget-parts", Rationale: "needs parts"},
		},
	})
	if err != nil {
		t.Fatalf("RenderSkill: %v", err)
	}
	if !strings.HasPrefix(got, "---\nname: widget-maker\n") {
		t.Errorf("frontmatter name not slugified:\n%s", got)
	}
	if !strings.Contains(got, `description: "Use this when the user asks to build a widget."`) {
		t.Error("description missing from frontmatter")
	}
	for _, seg := range []string{"## R", "## I", "## A1", "## A2", "## E", "## B"} {
		if !strings.Contains(got, seg+"\n") {
			t.Errorf("missing RIA segment heading %q", seg)
		}
	}
	if !strings.Contains(got, "## Related skills") || !strings.Contains(got, "widget-parts") {
		t.Error("related-skills section missing")
	}
}

func TestRenderSkillRejectsUnknownKind(t *testing.T) {
	t.Parallel()
	_, err := scaffold.RenderSkill(&scaffold.Skill{
		Slug:        "s",
		Description: "d",
		Related:     []scaffold.Edge{{Kind: "bogus", Target: "x"}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown related kind") {
		t.Errorf("got %v, want an unknown-kind error", err)
	}
}

func TestBuildTestsScaffoldsWhenNoneSupplied(t *testing.T) {
	t.Parallel()
	f := scaffold.BuildTests(&scaffold.Skill{Slug: "demo"})
	if len(f.Tests) < 6 {
		t.Fatalf("Scaffold stub should have >=6 cases, got %d", len(f.Tests))
	}
	if problems := f.Validate(); len(problems) != 0 {
		t.Errorf("scaffold stub should pass validation, got %v", problems)
	}
}

func TestBuildTestsSeedsChecksFromSupplied(t *testing.T) {
	t.Parallel()
	expected := `the output must contain a "Risks" section`
	f := scaffold.BuildTests(&scaffold.Skill{
		Slug: "demo",
		TestPrompts: []scaffold.Prompt{
			{Type: "should_trigger", Prompt: "do it", Expected: expected},
		},
	})
	if len(f.Tests) != 1 || f.Tests[0].ID != 1 || f.Tests[0].Type != "should_trigger" {
		t.Fatalf("unexpected cases: %+v", f.Tests)
	}
	if !reflect.DeepEqual(f.Tests[0].Checks, testprompts.DeriveChecks(expected)) {
		t.Errorf("checks not seeded via DeriveChecks: %+v", f.Tests[0].Checks)
	}
}
