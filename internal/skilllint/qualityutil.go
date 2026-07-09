package skilllint

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	sentPlaceholder = "∯"
	textSniffBytes  = 512
)

func disclosureDirNames() []string { return []string{"references", "scripts", "assets"} }

// proseWordCount counts whitespace-separated words containing an alphanumeric.
func proseWordCount(prose string) int {
	n := 0
	for _, w := range strings.Fields(prose) {
		if strings.IndexFunc(w, isAlnum) >= 0 {
			n++
		}
	}
	return n
}

func isAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// splitSentences splits text on sentence boundaries, protecting abbreviations and
// decimal numbers from being treated as boundaries (skillscheck heuristic).
func splitSentences(text string) []string {
	if text == "" {
		return nil
	}
	protected := reAbbrev().ReplaceAllStringFunc(text, func(m string) string {
		return strings.ReplaceAll(m, ".", sentPlaceholder)
	})
	protected = reDecimal().ReplaceAllString(protected, "${1}"+sentPlaceholder+"${2}")

	var parts []string
	start := 0
	for _, loc := range reSentenceBoundary().FindAllStringSubmatchIndex(protected, -1) {
		capStart := loc[2]
		if seg := restoreSentence(protected[start:capStart]); seg != "" {
			parts = append(parts, seg)
		}
		start = capStart
	}
	if seg := restoreSentence(protected[start:]); seg != "" {
		parts = append(parts, seg)
	}
	return parts
}

func restoreSentence(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), sentPlaceholder, ".")
}

func commaSegments(sentence string) []string {
	var segs []string
	for _, p := range strings.Split(sentence, ",") {
		if t := strings.TrimSpace(p); t != "" {
			segs = append(segs, t)
		}
	}
	return segs
}

func shortSegmentShare(segs []string) int {
	if len(segs) == 0 {
		return 0
	}
	short := 0
	for _, s := range segs {
		if len(strings.Fields(s)) <= shortSegWords {
			short++
		}
	}
	return short * 100 / len(segs)
}

func isSecretFilename(name string) bool {
	set := secretNames()
	lower := strings.ToLower(name)
	if set[lower] || set[strings.ToLower(filepath.Ext(name))] {
		return true
	}
	return reUnderscoreSecret().MatchString(lower)
}

func isBinaryExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".exe", ".dll", ".so", ".dylib", ".bin", ".o", ".a", ".pyc", ".class", ".wasm":
		return true
	default:
		return false
	}
}

func isScannableExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".md", ".txt", ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

func isExtraneousFile(lowerName string) bool {
	switch lowerName {
	case "readme.md", "readme", "readme.txt", "readme.rst",
		"changelog.md", "changelog", "changelog.txt",
		"license", "license.md", "license.txt",
		"contributing.md", "code_of_conduct.md", "makefile", "agents.md":
		return true
	default:
		return false
	}
}

func fileHasSecret(path string) bool {
	content := readFile(path)
	for _, re := range secretContentREs() {
		if re.MatchString(content) {
			return true
		}
	}
	return false
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// disclosureMarkdown returns, sorted, every .md file under references/, scripts/,
// and assets/ of the skill directory.
func disclosureMarkdown(skillDir string, exclude []string) []string {
	var out []string
	for _, sub := range disclosureDirNames() {
		for _, f := range filesUnder(filepath.Join(skillDir, sub), exclude) {
			if strings.EqualFold(filepath.Ext(f), ".md") {
				out = append(out, f)
			}
		}
	}
	sort.Strings(out)
	return out
}

// filesUnder returns, sorted, every regular file beneath dir (recursively),
// excluding version-control directories and any path segment matching an
// exclude glob.
func filesUnder(dir string, exclude []string) []string {
	var out []string
	walkSkillFiles(dir, exclude, func(path string) { out = append(out, path) })
	sort.Strings(out)
	return out
}

// walkSkillFiles invokes fn for each regular file beneath root, pruning hidden
// directories (any whose base name starts with ".", covering .git/.hg/.svn and
// tool caches like .rumdl_cache), directories and files whose base name matches
// an exclude glob, and silently skipping unreadable entries. Hidden FILES are
// not pruned, so secret files such as .env still reach fn.
func walkSkillFiles(root string, exclude []string, fn func(path string)) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable entries are skipped, not fatal
		}
		if d.IsDir() {
			if path != root && (isHiddenDir(d.Name()) || matchesAnyGlob(d.Name(), exclude)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !matchesAnyGlob(d.Name(), exclude) {
			fn(path)
		}
		return nil
	})
}

func isHiddenDir(name string) bool {
	return strings.HasPrefix(name, ".")
}

// matchesAnyGlob reports whether name matches any of the shell-style globs. An
// invalid glob simply does not match.
func matchesAnyGlob(name string, globs []string) bool {
	for _, g := range globs {
		if ok, _ := filepath.Match(g, name); ok {
			return true
		}
	}
	return false
}

// isProbablyText reports whether the file at path looks like text: its first
// bytes contain no NUL and are valid UTF-8.
func isProbablyText(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, textSniffBytes)
	n, _ := f.Read(buf)
	buf = buf[:n]
	return bytes.IndexByte(buf, 0) < 0 && utf8.Valid(buf)
}

// resolveLocalLink resolves target relative to baseDir, bounded by skillRoot. It
// reports the resolved path, whether it escapes the skill, and whether it is a
// local (non-remote-scheme) link at all.
func resolveLocalLink(baseDir, target, skillRoot string) (resolved string, escapes, local bool) {
	if target == "" {
		return "", false, false
	}
	lower := strings.ToLower(target)
	for _, scheme := range []string{"http://", "https://", "mailto:", "tel:", "ftp://"} {
		if strings.HasPrefix(lower, scheme) {
			return "", false, false
		}
	}
	if strings.HasPrefix(target, "//") {
		return "", false, false
	}
	if strings.HasPrefix(target, "/") {
		return "", true, false
	}
	full := resolvePath(filepath.Clean(filepath.Join(baseDir, target)))
	if full != skillRoot && !strings.HasPrefix(full, skillRoot+string(filepath.Separator)) {
		return full, true, false
	}
	return full, false, true
}

func secretNames() map[string]bool {
	return map[string]bool{
		".env": true, ".env.local": true, ".env.production": true, ".env.staging": true,
		".env.development": true, ".pem": true, ".key": true, "credentials.json": true,
		".pfx": true, ".p12": true,
	}
}

func secretContentREs() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		regexp.MustCompile(`gh[posr]_[A-Za-z0-9]{36}`),
		regexp.MustCompile(`-{5}BEGIN.*PRIVATE KEY-{5}`),
		regexp.MustCompile(`glpat-[A-Za-z0-9\-_]{20,}`),
		regexp.MustCompile(`xox[bp]-\d+-[0-9A-Za-z]+`),
		regexp.MustCompile(`xapp-\d+-[0-9A-Za-z]+`),
		regexp.MustCompile(`LS0tLS1CRUdJTi[A-Za-z0-9+/=]+`),
	}
}

// reUseWhen matches language indicating WHEN a skill applies. A bare "when" or
// "whenever" counts (it subsumes "use when", "when you need", "invoke … when",
// "when you encounter …"); "trigger" catches explicit "Trigger:" labels and
// "the trigger signal is …"; "after"/"before" catch temporal triggers like
// "use this skill after an incident" and "invoke after a gap is located". The
// remaining alternatives cover descriptions that state applicability without any
// of those words. This is an advisory warning, so erring toward a false pass (a
// topical "when"/"after") beats falsely flagging a clear trigger clause.
func reUseWhen() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(when|whenever|after|before|trigger|use for|use to|` +
		`use if|designed for|required for|needed for|applies to|for use in)\b`)
}

func reUserCentric() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(whenever the user|when the user|if the user|` +
		`user asks|user mentions|user requests|user wants)\b`)
}

func reQuoted() *regexp.Regexp           { return regexp.MustCompile(`"[^"]*"`) }
func reUnderscoreSecret() *regexp.Regexp { return regexp.MustCompile(`(?i)_secret`) }

func reAbbrev() *regexp.Regexp           { return regexp.MustCompile(`(?i)\b(e\.g|i\.e|vs|al|approx|etc)\.\s`) }
func reDecimal() *regexp.Regexp          { return regexp.MustCompile(`(\d)\.(\d)`) }
func reSentenceBoundary() *regexp.Regexp { return regexp.MustCompile(`[.!?]\s+([A-Z])`) }
