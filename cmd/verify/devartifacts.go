package verify

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	lintlib "github.com/StevenACoffman/exegesis/internal/lint"
	"github.com/StevenACoffman/exegesis/internal/mergedfrom"
)

// mergeAudit is the file merge-skills writes for each merged skill, under the parked
// dev/<slug>/ tree rather than inside the skill.
const mergeAudit = "merge-audit.md"

// checkDevArtifacts reports development files inside the tree's skills: book2skill's and
// merge-skills' reports, maintainer backlogs, and evaluation workspaces.
//
// It is the half of the closure check that was deferred on 2026-08-23 because the corpus
// held 130+ such files and reporting them would have buried the one real finding. The
// market has since moved them to a parked dev/<skill>/ tree outside the shipped plugins
// (D8 in stbenjam_improvements.md), so what is left is the regression this guards: a
// plugin install copies the whole skill directory, and a file found here ships with it.
//
// Reported, not failed, like the rest of the closure check: a tree under construction
// legitimately holds them until they are parked.
//
// Ensures: one line per artifact, a workspace directory reported once rather than per
// file inside it, in report then walk order.
func checkDevArtifacts(reports []skillReport) []string {
	var lines []string
	for i := range reports {
		r := &reports[i]
		found, err := devArtifacts(os.DirFS(r.dir))
		if err != nil {
			lines = append(
				lines,
				fmt.Sprintf("closure: %s: dev artifacts not checked: %v", r.slug, err),
			)
			continue
		}
		for _, p := range found {
			lines = append(lines, fmt.Sprintf(
				"closure: %s/%s: a development file inside the skill, so installers ship it "+
					"(reported, not failed: move it out of the shipped tree)", r.slug, p))
		}
	}
	return lines
}

// devArtifacts returns the development files and workspaces under fsys, in walk order.
func devArtifacts(fsys fs.FS) ([]string, error) {
	var found []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." || !lintlib.IsDevArtifact(p) {
			return nil
		}
		found = append(found, p)
		if d.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}
	return found, nil
}

// checkMergeAudits reports each skill declaring metadata.merged-from that has no merge
// audit at dev/<slug>/merge-audit.md.
//
// The dev/ tree is found, not configured: it is the nearest dev/ directory at or above
// tree, stopping at the repository root. That covers both layouts that exist --
// merge-skills writes <output>/dev/<slug>/ beside the skills it merged, and the market
// parks every skill's files in <repo>/dev/<slug>/ above plugins/<plugin>/skills/ -- and
// the repository boundary keeps an unrelated dev/ further up from being mistaken for it.
//
// Reported, not failed, like the rest of the closure check. An installed or exported
// tree has no dev/ at all, and the audit is evidence about how a skill was made, not
// part of what ships. The audit's contents are not compared with the declared sources:
// the four audits in the market record them in three different shapes.
//
// Ensures: nil when no skill declares a merge; otherwise one line per merged skill
// without an audit, or a single line saying no dev/ directory was found.
func checkMergeAudits(tree string, reports []skillReport) []string {
	var merged []string
	for i := range reports {
		if reports[i].merged {
			merged = append(merged, reports[i].slug)
		}
	}
	if len(merged) == 0 {
		return nil
	}
	dev, ok := findDevDir(tree)
	if !ok {
		return []string{fmt.Sprintf(
			"dev: no dev/ directory at or above %s, so the merge audits of %d merged "+
				"skill(s) were not checked (reported, not failed)", tree, len(merged))}
	}
	var lines []string
	for _, slug := range merged {
		audit := filepath.Join(dev, slug, mergeAudit)
		if _, err := os.Stat(audit); err != nil {
			lines = append(lines, fmt.Sprintf(
				"dev: %s: declares %s but has no %s (reported, not failed)",
				slug, mergedfrom.Path, audit))
		}
	}
	return lines
}

// findDevDir returns the nearest dev/ directory at or above tree, looking no higher than
// the repository root: the first ancestor holding .git. Outside a repository only tree
// itself is looked in, so a walk can never reach the system's own /dev. A dev/ that holds
// a SKILL.md is a skill named dev, not the parked tree, and is passed over.
func findDevDir(tree string) (string, bool) {
	start, err := filepath.Abs(tree)
	if err != nil {
		return "", false
	}
	top := start
	for dir := start; ; dir = filepath.Dir(dir) {
		if exists(filepath.Join(dir, ".git")) {
			top = dir
			break
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	for dir := start; ; dir = filepath.Dir(dir) {
		dev := filepath.Join(dir, "dev")
		if isDir(dev) && !exists(filepath.Join(dev, "SKILL.md")) {
			return dev, true
		}
		if dir == top {
			return "", false
		}
	}
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
