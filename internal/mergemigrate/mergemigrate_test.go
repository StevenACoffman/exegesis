package mergemigrate_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/mergemigrate"
)

// merged is the frontmatter shape every skill in books/merged/all-books-v1 has: five
// keys the spec calls unknown fields, and no `name`.
const merged = `---
id: some-merged-skill
title: Some Merged Skill — With a Subtitle
description: Use this skill when the user needs the merged thing done in a particular way.
type: merged-skill
source_skills:
  - slug: book-a/source-one
    book: "Book A"
    author: Author A
  - slug: book-b/source-two
    book: "Book B"
    author: Author B
related_skills:
  - slug: book-a/source-one
    relation: supersedes
    note: adds the thing the source lacks
  - slug: book-b/other-skill
    relation: composes-with
    note: used together
tags: [merging, testing]
---

# Some Merged Skill — With a Subtitle

## R — Original Text

body text

## Audit

- **V1** ok
`

func TestMigrate(t *testing.T) {
	t.Parallel()
	got, changed, err := mergemigrate.Migrate(merged, "some-merged-skill")
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !changed {
		t.Fatal("expected a merged skill's frontmatter to change")
	}

	frontmatter, _, _ := strings.Cut(strings.TrimPrefix(got, "---\n"), "\n---\n")

	cases := map[string]struct {
		want string
		in   bool // true = must be present, false = must be gone
	}{
		"name is set from the folder": {"name: some-merged-skill", true},
		"description survives verbatim": {
			"description: Use this skill when the user needs the merged thing done in a particular way.",
			true,
		},
		"tags survive verbatim": {"tags: [merging, testing]", true},
		"id is dropped":         {"id:", false},

		"source_skills leave frontmatter": {"---\nname: some-merged-skill\ndescription", true},
		"provenance section is written":   {"## Provenance", true},
		"prose names the first source": {
			"- `book-a/source-one` — *Book A* by Author A",
			true,
		},
		"machine block names both sources": {"- slug: book-b/source-two", true},
		"a supersession note is kept on its source": {
			"note: adds the thing the source lacks",
			true,
		},
		"composes-with becomes a body edge": {
			"- composes-with: `book-b/other-skill` — used together",
			true,
		},
		"supersedes is not restated as edge": {"superseded", false},
		"provenance precedes the audit":      {"## Provenance", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if strings.Contains(got, tc.want) != tc.in {
				t.Errorf("presence of %q = %v, want %v in:\n%s", tc.want, !tc.in, tc.in, got)
			}
		})
	}

	if at, audit := strings.Index(
		got,
		"## Provenance",
	), strings.Index(
		got,
		"## Audit",
	); at > audit {
		t.Errorf("Provenance must precede the audit section:\n%s", got)
	}
	// The moved keys must be gone from the frontmatter specifically: `type` and the
	// source slugs legitimately reappear in the body's machine-readable block, so
	// asserting on the whole file would be asserting the opposite of the intent.
	for _, key := range []string{"id:", "title:", "type:", "source_skills:", "related_skills:"} {
		if strings.Contains(frontmatter, key) {
			t.Errorf("%q survived in the frontmatter:\n%s", key, frontmatter)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	t.Parallel()
	once, _, err := mergemigrate.Migrate(merged, "some-merged-skill")
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	twice, changed, err := mergemigrate.Migrate(once, "some-merged-skill")
	if err != nil {
		t.Fatalf("Migrate again: %v", err)
	}
	if changed || twice != once {
		t.Errorf("a second migration must be a no-op, got changed=%v:\n%s", changed, twice)
	}
	if n := strings.Count(twice, "## Provenance"); n != 1 {
		t.Errorf("expected exactly one Provenance section, got %d", n)
	}
}

func TestMigrateRestoresATitleTheBodyDoesNotHave(t *testing.T) {
	t.Parallel()
	// 10 of the 27 real merged skills have no `# ` heading, so dropping the
	// frontmatter title would delete the file's only human-readable name. This is the
	// one key whose removal is not obviously lossless, so it gets its own test.
	in := strings.Replace(merged, "\n# Some Merged Skill — With a Subtitle\n", "", 1)
	got, _, err := mergemigrate.Migrate(in, "some-merged-skill")
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !strings.Contains(got, "# Some Merged Skill — With a Subtitle") {
		t.Errorf("the title was lost rather than restored as a heading:\n%s", got)
	}
	if n := strings.Count(got, "# Some Merged Skill"); n != 1 {
		t.Errorf("expected one title heading, got %d:\n%s", n, got)
	}
}

func TestMigrateLeavesAnOrdinarySkillAlone(t *testing.T) {
	t.Parallel()
	// A book skill carries none of the moved keys. Migrating a whole tree must not
	// rewrite the skills that were never the problem.
	in := "---\nname: plain-skill\ndescription: Use when the user needs a plain thing.\n" +
		"tags: [x]\n---\n\n# Plain Skill\n\nbody\n"
	got, changed, err := mergemigrate.Migrate(in, "plain-skill")
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if changed || got != in {
		t.Errorf("an ordinary skill must be untouched, got changed=%v:\n%s", changed, got)
	}
}
