package provenance_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/provenance"
	"github.com/StevenACoffman/skillet/related"
)

// bulletDoc renders a skill body whose related-skills section holds one bullet.
func bulletDoc(kind, target string) string {
	return "# Skill\n\n## Related Skills\n\n- " + kind + ": `" + target + "` — why\n"
}

func TestTargets(t *testing.T) {
	t.Parallel()
	const merged = "merged/all-books-v1/x"
	cases := map[string]struct {
		frontmatter string
		body        string
		want        []string
	}{
		"bullet only": {
			body: bulletDoc("superseded-by", merged),
			want: []string{merged},
		},
		"frontmatter only": {
			frontmatter: "related_skills:\n  - slug: " + merged + "\n    relation: superseded-by\n",
			want:        []string{merged},
		},
		"both spellings are one relationship": {
			frontmatter: "related_skills:\n  - slug: " + merged + "\n    relation: superseded-by\n",
			body:        bulletDoc("superseded-by", merged),
			want:        []string{merged},
		},
		"frontmatter entry with no relation is not a supersession": {
			frontmatter: "related_skills:\n  - slug: " + merged + "\n",
			want:        nil,
		},
		"another relation is not a supersession": {
			frontmatter: "related_skills:\n  - slug: " + merged + "\n    relation: relates-to\n",
			body:        bulletDoc("depends-on", merged),
			want:        nil,
		},
		"malformed frontmatter yields none rather than failing": {
			frontmatter: "related_skills:\n  - slug: [unclosed\n",
			want:        nil,
		},
		"an unqualified target is a sibling, not a merge": {
			body: bulletDoc("superseded-by", "some-other-skill"),
			want: nil,
		},
		"two supersessions are both reported, sorted": {
			frontmatter: "related_skills:\n" +
				"  - slug: merged/run/b\n    relation: superseded-by\n" +
				"  - slug: merged/run/a\n    relation: superseded-by\n",
			want: []string{"merged/run/a", "merged/run/b"},
		},
		"no related skills at all": {
			body: "# Skill\n\nnothing here\n",
			want: nil,
		},
		// The shape hashimoto/table-driven-named-cases really has. Decoding this list
		// into a slice of structs fails on the first bare scalar and yields nothing for
		// the whole block, which hid one of the 27 merged skills until it was measured.
		"a list mixing bare slugs with mappings still yields the supersession": {
			frontmatter: "related_skills:\n" +
				"  - golden-files-update-flag\n" +
				"  - test-helper-contract\n" +
				"  - slug: " + merged + "\n    relation: superseded-by\n",
			want: []string{merged},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := provenance.Targets(tc.frontmatter, tc.body)
			if len(got) != len(tc.want) {
				t.Fatalf("Targets = %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("target %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestRelationVocabularyMatchesTheBulletKind pins the one word this package spells that
// skillet also spells. The frontmatter `relation:` value and the bullet kind are the same
// vocabulary; if skillet ever renames the kind, this fails here rather than silently
// halving the corpus a merge gate can see.
func TestRelationVocabularyMatchesTheBulletKind(t *testing.T) {
	t.Parallel()
	const merged = "merged/all-books-v1/x"
	fm := "related_skills:\n  - slug: " + merged +
		"\n    relation: " + string(related.SupersededBy) + "\n"
	if got := provenance.Targets(fm, ""); len(got) != 1 {
		t.Errorf("Targets with relation %q = %q, want the target read",
			related.SupersededBy, got)
	}
}

// writeSkill creates a skill directory holding a SKILL.md with the given frontmatter
// and body.
func writeSkill(t *testing.T, tree, slug, frontmatter, body string) {
	t.Helper()
	dir := filepath.Join(tree, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	doc := "---\nname: " + slug + "\ndescription: d\n" + frontmatter + "---\n\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write %s: %v", dir, err)
	}
}

func TestCollect(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeSkill(t, tree, "src-bullet", "",
		bulletDoc("superseded-by", "merged/run/target"))
	writeSkill(t, tree, "src-frontmatter",
		"related_skills:\n  - slug: merged/run/target\n    relation: superseded-by\n", "")
	writeSkill(t, tree, "src-unrelated", "", bulletDoc("depends-on", "merged/run/target"))

	idx, _, err := provenance.Collect([]string{tree})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	sources := idx["target"]
	if len(sources) != 2 {
		t.Fatalf(
			"sources for %q = %+v, want the two that declared a supersession",
			"target",
			sources,
		)
	}
	if sources[0].Slug != "src-bullet" || sources[1].Slug != "src-frontmatter" {
		t.Errorf("sources = %+v, want them sorted by slug", sources)
	}
	if sources[0].Dir != filepath.Join(tree, "src-bullet") {
		t.Errorf("Dir = %q, want the directory that was read", sources[0].Dir)
	}
}

// TestCollectKeysBySlugNotQualifiedTarget is the decision the Index doc records: the
// tree on disk and the target's prefix need not agree, so only the last segment can be
// the key.
func TestCollectKeysBySlugNotQualifiedTarget(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeSkill(t, tree, "src", "", bulletDoc("superseded-by", "merged/some-other-run/target"))

	idx, _, err := provenance.Collect([]string{tree})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if _, ok := idx["target"]; !ok {
		t.Errorf("index keys = %v, want the merged slug regardless of the run prefix", keysOf(idx))
	}
}

// TestCollectOmitsAMergedSkillNobodyNames keeps "replaced nothing we can see" tellable
// from "replaced nothing": an unnamed merged skill is absent, not present and empty.
func TestCollectOmitsAMergedSkillNobodyNames(t *testing.T) {
	t.Parallel()
	tree := t.TempDir()
	writeSkill(t, tree, "src", "", "# Skill\n\nno related skills\n")

	idx, _, err := provenance.Collect([]string{tree})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(idx) != 0 {
		t.Errorf("index = %v, want empty", keysOf(idx))
	}
}

// TestCollectReportsATreeWithNoSkillsWithoutFailing pins the split the Collect doc
// argues: a book that nests its skills a level down and a book with none built yet look
// identical from here, so the fact is reported and the judgement left to the caller.
func TestCollectReportsATreeWithNoSkillsWithoutFailing(t *testing.T) {
	t.Parallel()
	populated, bare := t.TempDir(), t.TempDir()
	writeSkill(t, populated, "src", "", bulletDoc("superseded-by", "merged/run/target"))

	idx, empty, err := provenance.Collect([]string{populated, bare})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(empty) != 1 || empty[0] != bare {
		t.Errorf("empty = %q, want just the tree holding no skills", empty)
	}
	if len(idx["target"]) != 1 {
		t.Errorf("sources = %+v, want the populated tree still read", idx["target"])
	}
}

func TestCollectReportsAnUnreadableTree(t *testing.T) {
	t.Parallel()
	if _, _, err := provenance.Collect([]string{filepath.Join(t.TempDir(), "absent")}); err == nil {
		t.Error("Collect over a missing tree = nil error, want the tree reported")
	}
}

// keysOf returns an index's keys for failure messages.
func keysOf(idx provenance.Index) []string {
	out := make([]string, 0, len(idx))
	for k := range idx {
		out = append(out, k)
	}
	return out
}
