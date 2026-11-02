package lint

import (
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/markdown"
	"github.com/StevenACoffman/skillet/skill"
)

// isFamilyFile reports whether base names a file the family's own tools read beside
// SKILL.md, so nothing in the skill needs to reference it: test-prompts.json is the test
// contract exegesis and skillsaw read, and the two reports are written by book2skill and
// merge-skills.
func isFamilyFile(base string) bool {
	switch base {
	case "test-prompts.json", "test-results.md", "merge-audit.md":
		return true
	}
	return false
}

// unreferencedFindings warns about each bundled file nothing points an agent at.
//
// A file is referenced when SKILL.md, or a markdown file SKILL.md references (followed
// transitively), names it in a link or a code span. A reference to a directory covers
// everything under it. Paths are resolved both against the naming file's directory and
// against the skill root, because skills write both. Only links and code spans count:
// prose that mentions a file by name is too loose to follow mechanically, so a file named
// only in prose is reported, and the remedy is to link it.
//
// It is advisory: an unreferenced file is shipped but an agent will not know to read it,
// which is a cost rather than a defect. Dev artifacts are left to the closure check.
//
// Requires: files holds every bundled text file other than SKILL.md.
// Ensures:  one warning per unreferenced file, sorted by path; it is pure.
func unreferencedFindings(body string, files []bundledFile) []finding.Diagnostic {
	texts := make(map[string]string, len(files))
	for _, f := range files {
		texts[f.path] = f.text
	}
	reached := reachable(body, texts)
	var paths []string
	for _, f := range files {
		base := path.Base(f.path)
		if isFamilyFile(base) || IsDevArtifact(f.path) || covered(f.path, reached) {
			continue
		}
		paths = append(paths, f.path)
	}
	sort.Strings(paths)
	ds := make([]finding.Diagnostic, 0, len(paths))
	for _, p := range paths {
		ds = append(ds, warnf(
			"bundle: %s is not linked or named in a code span from SKILL.md or a file it references, "+
				"so an agent will not know to read it",
			p,
		))
	}
	return ds
}

// reachable returns every path referenced from SKILL.md's body, following references
// into the markdown files they name.
func reachable(body string, texts map[string]string) map[string]bool {
	reached := map[string]bool{}
	queue := []string{skill.FileName}
	visited := map[string]bool{skill.FileName: true}
	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		text := body
		if from != skill.FileName {
			text = texts[from]
		}
		for _, target := range referencedPaths(from, text) {
			reached[target] = true
			if _, isFile := texts[target]; isFile && !visited[target] &&
				strings.HasSuffix(target, ".md") {
				visited[target] = true
				queue = append(queue, target)
			}
		}
	}
	return reached
}

// referencedPaths returns the in-skill paths text (the file at from) names in links and
// code spans, each resolved against from's directory and against the skill root.
func referencedPaths(from, text string) []string {
	doc := markdown.Parse(text)
	var out []string
	for _, ref := range append(doc.LinkDestinations, codeSpanTokens(doc.CodeSpans)...) {
		p, ok := inSkillPath(ref)
		if !ok {
			continue
		}
		out = append(out, p, path.Clean(path.Join(path.Dir(from), p)))
	}
	return out
}

// codeSpanTokens splits code spans into whitespace-separated tokens, since a span may hold
// a whole command ("cat references/guide.md") rather than a bare path.
func codeSpanTokens(spans []string) []string {
	var out []string
	for _, s := range spans {
		out = append(out, strings.Fields(s)...)
	}
	return out
}

// inSkillPath normalizes a reference to a relative path inside the skill, or reports
// false for a URL, an absolute or home path, or one that leaves the skill.
func inSkillPath(ref string) (string, bool) {
	ref = strings.Trim(ref, "\"'`,;:()")
	if i := strings.IndexAny(ref, "#?"); i >= 0 {
		ref = ref[:i]
	}
	if outsideByForm(ref) {
		return "", false
	}
	if unescaped, err := url.PathUnescape(ref); err == nil {
		ref = unescaped
	}
	clean := path.Clean(ref)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

// outsideByForm reports whether ref names no in-skill path by its form alone: empty,
// absolute, home-relative, or carrying a URL scheme.
func outsideByForm(ref string) bool {
	if ref == "" || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "~") {
		return true
	}
	u, err := url.Parse(ref)
	return err == nil && u.Scheme != ""
}

// covered reports whether p, or a directory containing it, was referenced.
func covered(p string, reached map[string]bool) bool {
	for q := p; q != "." && q != "/"; q = path.Dir(q) {
		if reached[q] {
			return true
		}
	}
	return false
}

// crossSkillFindings warns about each path in a skill that points into another skill's
// directory: an absolute or home path through a "skills/<name>/" segment, or a relative
// path that climbs out of this skill into a sibling. An install copies one skill
// directory, so such a path names a location the installed skill does not have; the
// remedy is to name the other skill and let the agent find it (F30).
//
// Requires: self is this skill's folder name; texts maps file name to content.
// Ensures:  one warning per (file, reference), in file then document order; it is pure.
func crossSkillFindings(self string, texts []bundledFile) []finding.Diagnostic {
	var ds []finding.Diagnostic
	for _, f := range texts {
		if !strings.HasSuffix(f.path, ".md") {
			continue
		}
		doc := markdown.Parse(f.text)
		seen := map[string]bool{}
		for _, ref := range append(doc.LinkDestinations, codeSpanTokens(doc.CodeSpans)...) {
			other, ok := otherSkill(self, f.path, ref)
			if !ok || seen[ref] {
				continue
			}
			seen[ref] = true
			ds = append(ds, warnf(
				"bundle: %s refers to %q, a path into skill %q's directory; "+
					"an install may not put it there, so name the skill instead", f.path, ref, other))
		}
	}
	return ds
}

// otherSkill returns the skill a path points into when that skill is not self. A relative
// ref is resolved from the directory of from, the file it appears in (a slash path within
// the skill), so references/x.md linking "../SKILL.md" stays inside the skill. A fragment
// or query is not part of the path.
func otherSkill(self, from, ref string) (string, bool) {
	ref = strings.Trim(ref, "\"'`,;:()")
	if i := strings.IndexAny(ref, "#?"); i >= 0 {
		ref = ref[:i]
	}
	switch {
	case strings.HasPrefix(ref, "/"), strings.HasPrefix(ref, "~/"):
		return skillInAbsolute(self, ref)
	case strings.HasPrefix(ref, "../"):
		return skillInRelative(self, from, ref)
	}
	return "", false
}

// skillInAbsolute finds a "skills/<name>/" segment in an absolute or home path.
func skillInAbsolute(self, ref string) (string, bool) {
	segs := strings.Split(path.Clean(ref), "/")
	for i := 0; i+2 < len(segs); i++ {
		if segs[i] == "skills" && segs[i+1] != self && segs[i+1] != "..." {
			return segs[i+1], true
		}
	}
	return "", false
}

// skillInRelative resolves ref from from's directory and names the sibling skill it
// lands in, if it leaves this skill at all.
func skillInRelative(self, from, ref string) (string, bool) {
	resolved := path.Clean(path.Join(path.Dir(from), ref))
	if !strings.HasPrefix(resolved, "../") {
		return "", false // climbs only as far as this skill's own directory
	}
	for _, s := range strings.Split(resolved, "/") {
		if s != ".." && s != "" {
			return s, s != self
		}
	}
	return "", false
}
