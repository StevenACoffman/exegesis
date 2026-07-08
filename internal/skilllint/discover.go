package skilllint

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

// Discover walks root and returns skill directories: any directory containing a
// SKILL.md, plus any immediate child of a directory named "skills" (so a missing
// SKILL.md can still be reported). Results are deduplicated by resolved path and
// sorted stably by basename, matching skillscheck.
func Discover(root string) ([]string, error) {
	var dirs []string
	seen := make(map[string]bool)

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if fileExists(filepath.Join(path, "SKILL.md")) {
			addUnique(&dirs, seen, path)
		}
		if d.Name() == "skills" {
			for _, child := range childDirs(path) {
				addUnique(&dirs, seen, child)
			}
		}
		return nil
	})
	if walkErr != nil {
		return nil, &book2skill.Error{Op: "skilllint.Discover", Err: walkErr}
	}

	sort.SliceStable(dirs, func(i, j int) bool {
		return filepath.Base(dirs[i]) < filepath.Base(dirs[j])
	})
	return dirs, nil
}

func childDirs(path string) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, filepath.Join(path, e.Name()))
		}
	}
	return out
}

func addUnique(dirs *[]string, seen map[string]bool, path string) {
	key := resolvePath(path)
	if seen[key] {
		return
	}
	seen[key] = true
	*dirs = append(*dirs, path)
}

func resolvePath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
