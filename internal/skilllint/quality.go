package skilllint

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	minDescLen       = 20
	maxQualFileBytes = 100 * 1024
	minQuotedStrings = 5
	minStuffSegments = 8
	shortSegWords    = 3
	stuffPercent     = 60
)

// CheckQuality runs the quality checks (2a–2d) on one skill. exclude holds
// base-name globs whose files/dirs are pruned from the filesystem walks.
func CheckQuality(s *Skill, exclude []string) []Diagnostic {
	if s.ParseError != "" {
		return nil
	}
	var diags []Diagnostic
	diags = append(diags, checkDescriptionQuality(s)...)
	diags = append(diags, checkFileHygiene(s, exclude)...)
	diags = append(diags, checkExtraneousFiles(s, exclude)...)
	diags = append(diags, checkBodyLinks(s)...)
	diags = append(diags, checkUnclosedFences(s, exclude)...)
	diags = append(diags, checkReferenceLinks(s, exclude)...)
	diags = append(diags, checkOrphanFiles(s, exclude)...)
	return diags
}

func checkDescriptionQuality(s *Skill) []Diagnostic {
	desc, ok := stringField(s.Frontmatter, "description")
	if !ok {
		return nil
	}
	var diags []Diagnostic
	if utf8.RuneCountInString(desc) < minDescLen {
		diags = append(diags, diag(LevelWarning, "2a.description.short",
			"description is only "+strconv.Itoa(utf8.RuneCountInString(desc))+
				" chars — probably insufficient for agent matching", s.SkillMDPath))
	}
	if !reUseWhen().MatchString(desc) {
		diags = append(diags, diag(LevelWarning, "2a.description.no-when",
			"description doesn't indicate when to use the skill "+
				"(spec recommends describing both what and when)", s.SkillMDPath))
	}
	if reUserCentric().MatchString(desc) {
		diags = append(diags, Diagnostic{
			Level: LevelWarning, Check: "2a.description.user-centric",
			Message: "description uses user-centric trigger — prefer agent-directed phrasing",
			Path:    s.SkillMDPath,
		})
	}
	diags = append(diags, checkKeywordStuffing(desc, s.SkillMDPath)...)
	return diags
}

func checkKeywordStuffing(desc, path string) []Diagnostic {
	quotes := reQuoted().FindAllString(desc, -1)
	if len(quotes) >= minQuotedStrings {
		prose := reQuoted().ReplaceAllString(desc, "")
		if proseWordCount(prose) < len(quotes) {
			return []Diagnostic{diag(LevelInfo, "2a.description.keyword-stuffing",
				"description contains "+strconv.Itoa(len(quotes))+" quoted strings with little "+
					"surrounding prose — consider also explaining what the skill does", path)}
		}
	}
	noQuotes := reQuoted().ReplaceAllString(desc, "")
	for _, sentence := range splitSentences(noQuotes) {
		segs := commaSegments(sentence)
		if len(segs) >= minStuffSegments && shortSegmentShare(segs) >= stuffPercent {
			return []Diagnostic{diag(LevelInfo, "2a.description.keyword-stuffing",
				"description has "+strconv.Itoa(len(segs))+" comma-separated segments, most very "+
					"short — consider also explaining what the skill does", path)}
		}
	}
	return nil
}

func checkFileHygiene(s *Skill, exclude []string) []Diagnostic {
	var diags []Diagnostic
	root := s.DirPath
	walkSkillFiles(root, exclude, func(path string) {
		rel, _ := filepath.Rel(root, path)
		diags = append(diags, fileHygieneDiags(path, rel)...)
	})
	return diags
}

func fileHygieneDiags(path, rel string) []Diagnostic {
	var diags []Diagnostic
	base := filepath.Base(path)
	if isSecretFilename(base) {
		diags = append(diags, Diagnostic{
			Level: LevelWarning, Check: "2b.secrets.filename",
			Message: "file '" + rel + "' matches a known secret filename pattern", Path: path,
		})
	}
	if isBinaryExt(filepath.Ext(base)) {
		diags = append(diags, Diagnostic{
			Level: LevelWarning, Check: "2b.binary",
			Message: "binary file '" + rel + "' found in skill directory", Path: path,
		})
	}
	if info, err := os.Stat(path); err == nil && info.Size() > maxQualFileBytes {
		diags = append(diags, Diagnostic{
			Level: LevelWarning, Check: "2b.large-file",
			Message: "file '" + rel + "' is " + strconv.FormatInt(info.Size()/1024, 10) +
				"KB (> 100KB)", Path: path,
		})
	}
	if isScannableExt(filepath.Ext(base)) && fileHasSecret(path) {
		diags = append(diags, Diagnostic{
			Level: LevelWarning, Check: "2b.secrets.content",
			Message: "file '" + rel + "' may contain a secret token", Path: path,
		})
	}
	return diags
}

func checkExtraneousFiles(s *Skill, exclude []string) []Diagnostic {
	entries, err := os.ReadDir(s.DirPath)
	if err != nil {
		return nil
	}
	var diags []Diagnostic
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == "SKILL.md" || strings.HasPrefix(name, ".") ||
			matchesAnyGlob(name, exclude) {
			continue
		}
		if isExtraneousFile(strings.ToLower(name)) {
			diags = append(diags, Diagnostic{
				Level: LevelInfo,
				Check: "2b.extraneous-file",
				Message: "'" + name + "' is not needed in a skill — agents may load it into their " +
					"context window, wasting space",
				Path: filepath.Join(s.DirPath, name),
			})
		}
	}
	return diags
}

func checkBodyLinks(s *Skill) []Diagnostic {
	if s.Body == "" {
		return nil
	}
	var diags []Diagnostic
	for _, target := range ExtractLocalLinkTargets(s.Body) {
		if !pathExists(filepath.Join(s.DirPath, target)) {
			diags = append(diags, Diagnostic{
				Level: LevelWarning, Check: "2c.broken-link",
				Message: "link target '" + target + "' does not exist", Path: s.SkillMDPath,
			})
		}
	}
	selfHeadings := ExtractHeadings(s.Body)
	for _, link := range ExtractFragmentLinks(s.Body) {
		diags = append(
			diags,
			fragmentDiag(s.DirPath, s.SkillMDPath, "SKILL.md", link, selfHeadings)...)
	}
	return diags
}

func fragmentDiag(
	dir, reportPath, selfLabel string,
	link LinkFragment,
	selfHeadings map[string]bool,
) []Diagnostic {
	if link.Path == "" {
		if selfHeadings[link.Fragment] {
			return nil
		}
		return []Diagnostic{{
			Level: LevelWarning, Check: "2c.broken-link.fragment",
			Message: "fragment '#" + link.Fragment + "' does not match any heading in " + selfLabel,
			Path:    reportPath,
		}}
	}
	target := filepath.Join(dir, link.Path)
	if !pathExists(target) {
		return nil // file-level broken link already reported elsewhere
	}
	if ExtractHeadings(readFile(target))[link.Fragment] {
		return nil
	}
	return []Diagnostic{
		{
			Level:   LevelWarning,
			Check:   "2c.broken-link.fragment",
			Message: "fragment '#" + link.Fragment + "' does not match any heading in '" + link.Path + "'",
			Path:    reportPath,
		},
	}
}

func checkUnclosedFences(s *Skill, exclude []string) []Diagnostic {
	var diags []Diagnostic
	if line, open := FindUnclosedFence(s.Body); open {
		diags = append(diags, Diagnostic{
			Level: LevelError, Check: "2d.unclosed-fence",
			Message: "SKILL.md has an unclosed code fence starting at line " +
				strconv.Itoa(
					s.BodyLineOffset+line,
				), Path: s.SkillMDPath, Line: s.BodyLineOffset + line,
		})
	}
	for _, ref := range disclosureMarkdown(s.DirPath, exclude) {
		if line, open := FindUnclosedFence(readFile(ref)); open {
			rel, _ := filepath.Rel(s.DirPath, ref)
			diags = append(diags, Diagnostic{
				Level: LevelError,
				Check: "2d.unclosed-fence",
				Message: "'" + rel + "' has an unclosed code fence starting at line " + strconv.Itoa(
					line,
				),
				Path: ref,
				Line: line,
			})
		}
	}
	return diags
}

func checkReferenceLinks(s *Skill, exclude []string) []Diagnostic {
	var diags []Diagnostic
	skillRoot := resolvePath(s.DirPath)
	for _, ref := range disclosureMarkdown(s.DirPath, exclude) {
		content := readFile(ref)
		baseDir := filepath.Dir(ref)
		rel, _ := filepath.Rel(s.DirPath, ref)
		for _, target := range ExtractLocalLinkTargets(content) {
			resolved, escapes, local := resolveLocalLink(baseDir, target, skillRoot)
			switch {
			case escapes:
				diags = append(diags, Diagnostic{
					Level: LevelInfo, Check: "2c.escapes-skill",
					Message: "'" + rel + "': link target '" + target +
						"' resolves outside the skill directory", Path: ref,
				})
			case local && !pathExists(resolved):
				diags = append(diags, Diagnostic{
					Level:   LevelWarning,
					Check:   "2c.broken-link",
					Message: "'" + rel + "': link target '" + target + "' does not exist",
					Path:    ref,
				})
			}
		}
	}
	return diags
}

func checkOrphanFiles(s *Skill, exclude []string) []Diagnostic {
	if s.Body == "" {
		return nil
	}
	var diags []Diagnostic
	for _, sub := range disclosureDirNames() {
		subdir := filepath.Join(s.DirPath, sub)
		for _, fpath := range filesUnder(subdir, exclude) {
			if filepath.Base(fpath) == "__init__.py" {
				continue
			}
			rel, _ := filepath.Rel(s.DirPath, fpath)
			if !strings.Contains(s.Body, rel) && !strings.Contains(s.Body, filepath.Base(fpath)) {
				diags = append(diags, Diagnostic{
					Level:   LevelInfo,
					Check:   "2b.orphan",
					Message: "'" + rel + "' is not referenced from SKILL.md — agents may not discover it",
					Path:    fpath,
				})
			}
		}
	}
	return diags
}
