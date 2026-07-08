package skilllint

import (
	"path/filepath"
	"strconv"
	"strings"
)

const maxReferenceTokens = 10000

// CheckDisclosure runs the progressive-disclosure checks (4b, 4c) on one skill.
// exclude prunes files/dirs by base-name glob; an optional TokenCounter overrides
// the default approximation for 4b.
func CheckDisclosure(s *Skill, exclude []string, count ...TokenCounter) []Diagnostic {
	if s.ParseError != "" {
		return nil
	}
	var diags []Diagnostic
	diags = append(diags, checkReferenceSizing(s, exclude, resolveCounter(count))...)
	diags = append(diags, checkNesting(s)...)
	return diags
}

// checkReferenceSizing warns about reference files over the token budget.
//
// skillscheck iterates these files in incidental (unsorted) rglob order; this
// port sorts them for deterministic output, which changes only diagnostic order,
// not which diagnostics fire.
func checkReferenceSizing(s *Skill, exclude []string, count TokenCounter) []Diagnostic {
	var diags []Diagnostic
	for _, sub := range disclosureDirNames() {
		for _, fpath := range filesUnder(filepath.Join(s.DirPath, sub), exclude) {
			if !isProbablyText(fpath) {
				continue // token counts are meaningless for binary/media files
			}
			tokens := count(readFile(fpath))
			if tokens > maxReferenceTokens {
				rel, _ := filepath.Rel(s.DirPath, fpath)
				diags = append(diags, Diagnostic{
					Level: LevelWarning,
					Check: "4b.reference.large",
					Message: "reference file '" + rel + "' is ~" + strconv.Itoa(
						tokens,
					) + " tokens (consider splitting)",
					Path:      fpath,
					SourceURL: SpecURL,
				})
			}
		}
	}
	return diags
}

// checkNesting flags a reference (a .md linked from SKILL.md) that itself links
// to other local files, since references should stay one level deep.
func checkNesting(s *Skill) []Diagnostic {
	var diags []Diagnostic
	for _, target := range ExtractLocalLinkTargets(s.Body) {
		ref := filepath.Join(s.DirPath, target)
		if !strings.EqualFold(filepath.Ext(ref), ".md") || !fileExists(ref) {
			continue
		}
		if nested := ExtractLocalLinkTargets(readFile(ref)); len(nested) > 0 {
			rel, _ := filepath.Rel(s.DirPath, ref)
			diags = append(diags, Diagnostic{
				Level: LevelInfo,
				Check: "4c.nesting",
				Message: "reference '" + rel + "' contains " + strconv.Itoa(
					len(nested),
				) + " link(s) to other local files — consider keeping references one level deep",
				Path:      ref,
				SourceURL: SpecURL,
			})
		}
	}
	return diags
}
