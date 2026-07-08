package skilllint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint"
)

const validRIASkill = `---
name: inversion-thinking
description: Invoke when a user is stuck on a decision and keeps listing reasons for.
tags: [decision]
---

# Inversion Thinking

## R — Original text (Reading)

> Invert, always invert.
>
> — Jacobi

## I — Interpretation

Ask what would guarantee failure, then avoid it.

## A1 — Past application

### Case

- Problem: risk

## A2 — Trigger

1. stuck on a decision

## E — Execution

1. list failure modes

## B — Boundary

### Do not use when

- pure lookup
`

func checkIDs(diags []skilllint.Diagnostic) map[string]bool {
	ids := make(map[string]bool)
	for _, d := range diags {
		ids[d.Check] = true
	}
	return ids
}

func TestRedlinesValidSkillPasses(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "inversion-thinking")
	writeSkill(t, dir, validRIASkill)
	// A valid RIA++ skill needs its test-prompts.json to satisfy the last red line.
	if err := os.WriteFile(
		filepath.Join(dir, "test-prompts.json"),
		[]byte("[]"),
		0o600,
	); err != nil {
		t.Fatalf("write test-prompts: %v", err)
	}
	diags := skilllint.CheckRedlines(skilllint.Parse(dir))
	if len(diags) != 0 {
		t.Errorf("expected no red-line violations, got %v", diags)
	}
}

func TestRedlinesViolations(t *testing.T) {
	t.Parallel()
	// name mismatches dir, unknown key, XML in description, missing segments,
	// a ../SKILL.md related link, and (no test-prompts.json written).
	content := `---
name: other-name
description: A <b>bold</b> description.
bogus: nope
---

# Body

## R — Original text (Reading)

> a quote

## Related skills

- see [that](../other/SKILL.md)
`
	dir := filepath.Join(t.TempDir(), "my-skill")
	writeSkill(t, dir, content)

	ids := checkIDs(skilllint.CheckRedlines(skilllint.Parse(dir)))
	want := []string{
		"rl.frontmatter.allowed-keys",
		"rl.name.dir-match",
		"rl.description.plaintext",
		"rl.segments.present",
		"rl.related.slug-form",
		"rl.test-prompts.present",
	}
	for _, id := range want {
		if !ids[id] {
			t.Errorf("expected red-line %q to fire; got %v", id, keysOf(ids))
		}
	}
}

func keysOf(m map[string]bool) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return strings.Join(ks, ", ")
}
