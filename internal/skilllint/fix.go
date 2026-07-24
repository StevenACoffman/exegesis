package skilllint

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

const (
	maxFixPasses = 5
	fixFilePerm  = 0o644
)

// Fix applies the mechanical fixes skillscheck supports (name lowercasing,
// consecutive-hyphen collapse, directory rename to match name), re-running after
// each pass until nothing changes or maxFixPasses is reached. It returns the
// final Result of running cats, plus a description of every fix applied.
func Fix(root string, cats map[Category]bool) (*Result, []string, error) {
	var applied []string
	for range maxFixPasses {
		fixes, err := applyFixPass(root)
		if err != nil {
			return nil, nil, err
		}
		if len(fixes) == 0 {
			break
		}
		applied = append(applied, fixes...)
	}

	remaining, err := hasFixable(root)
	if err != nil {
		return nil, nil, err
	}
	if remaining {
		applied = append(applied, "warning: fix loop hit "+strconv.Itoa(maxFixPasses)+
			"-pass limit; re-run --fix to continue")
	}

	result, err := Run(root, Options{Categories: cats})
	if err != nil {
		return nil, nil, err
	}
	return result, applied, nil
}

func applyFixPass(root string) ([]string, error) {
	dirs, err := Discover(root)
	if err != nil {
		return nil, err
	}
	var applied []string
	for _, dir := range dirs {
		s := Parse(dir)
		if s.ParseError != "" {
			continue
		}
		for _, d := range CheckSpec(s, nil) {
			if !d.Fixable {
				continue
			}
			desc, fixErr := applyOneFix(s, d.Check)
			if fixErr != nil {
				return nil, fixErr
			}
			if desc != "" {
				applied = append(applied, desc)
			}
		}
	}
	return applied, nil
}

func hasFixable(root string) (bool, error) {
	dirs, err := Discover(root)
	if err != nil {
		return false, err
	}
	for _, dir := range dirs {
		s := Parse(dir)
		if s.ParseError != "" {
			continue
		}
		for _, d := range CheckSpec(s, nil) {
			if d.Fixable {
				return true, nil
			}
		}
	}
	return false, nil
}

func applyOneFix(s *Skill, check string) (string, error) {
	switch check {
	case "1b.name.format":
		return fixNameLowercase(s)
	case "1b.name.consecutive-hyphens":
		return fixConsecutiveHyphens(s)
	case "1b.name.dir-match":
		return fixDirMatch(s)
	default:
		return "", nil
	}
}

func fixNameLowercase(s *Skill) (string, error) {
	name, _ := stringField(s.Frontmatter, "name")
	lower := strings.ToLower(name)
	if name == "" || name == lower {
		return "", nil
	}
	updated, err := updateFrontmatterName(s.SkillMDPath, lower)
	if err != nil || !updated {
		return "", err
	}
	s.Frontmatter["name"] = lower
	return "lowercased name '" + name + "' to '" + lower + "' in " + s.SkillMDPath, nil
}

func fixConsecutiveHyphens(s *Skill) (string, error) {
	name, _ := stringField(s.Frontmatter, "name")
	if !strings.Contains(name, "--") {
		return "", nil
	}
	fixed := reConsecutiveHyphens().ReplaceAllString(name, "-")
	updated, err := updateFrontmatterName(s.SkillMDPath, fixed)
	if err != nil || !updated {
		return "", err
	}
	s.Frontmatter["name"] = fixed
	return "fixed consecutive hyphens in name '" + name + "' to '" + fixed + "'", nil
}

func fixDirMatch(s *Skill) (string, error) {
	name, _ := stringField(s.Frontmatter, "name")
	if name == "" || name == s.DirName || !nameRE().MatchString(name) ||
		strings.Contains(name, "--") {
		return "", nil
	}
	newDir := filepath.Join(filepath.Dir(s.DirPath), name)
	if pathExists(newDir) {
		return "", nil
	}
	if err := os.Rename(s.DirPath, newDir); err != nil {
		return "", &book2skill.Error{Op: "skilllint.fixDirMatch", Err: err}
	}
	old := s.DirName
	s.DirName = name
	s.DirPath = newDir
	s.SkillMDPath = filepath.Join(newDir, "SKILL.md")
	return "renamed directory '" + old + "' to '" + name + "'", nil
}

// updateFrontmatterName rewrites the first "name:" line inside the frontmatter,
// preserving all other formatting. It reports whether the file was changed.
func updateFrontmatterName(skillMD, newName string) (bool, error) {
	data, err := os.ReadFile(skillMD)
	if err != nil {
		return false, &book2skill.Error{Op: "skilllint.updateFrontmatterName", Err: err}
	}
	lines := strings.Split(string(data), "\n")
	end, ok := frontmatterEnd(lines)
	if !ok {
		return false, nil
	}
	for i := 1; i < end; i++ {
		if reNameLine().MatchString(lines[i]) {
			lines[i] = "name: " + newName
			if writeErr := os.WriteFile(
				skillMD,
				[]byte(strings.Join(lines, "\n")),
				fixFilePerm,
			); writeErr != nil {
				return false, &book2skill.Error{
					Op:  "skilllint.updateFrontmatterName",
					Err: writeErr,
				}
			}
			return true, nil
		}
	}
	return false, nil
}

func reConsecutiveHyphens() *regexp.Regexp { return regexp.MustCompile(`-{2,}`) }
func reNameLine() *regexp.Regexp           { return regexp.MustCompile(`^name\s*:`) }
