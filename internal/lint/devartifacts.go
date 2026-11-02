package lint

import (
	"path"
	"strings"
)

// IsDevArtifact reports whether p (a slash path inside a skill directory) is one of the
// family's development files rather than skill content: the reports book2skill and
// merge-skills write, a maintainer backlog or repository note, or a skill-creator
// evaluation workspace.
//
// D8 decided these move out of shipped plugins to a parked dev/<skill>/ tree, because a
// plugin install copies the whole skill directory. The closure check warns when one is
// found inside a skill so they do not drift back.
func IsDevArtifact(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if strings.HasSuffix(seg, "-workspace") {
			return true
		}
	}
	switch path.Base(p) {
	case "test-results.md", "merge-audit.md", "TODO.md", "GITHUB_REPO.md":
		return true
	}
	return false
}
