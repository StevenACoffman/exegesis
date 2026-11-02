package verify

import (
	"fmt"
	"io/fs"
	"os"

	lintlib "github.com/StevenACoffman/exegesis/internal/lint"
)

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
