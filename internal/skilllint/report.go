package skilllint

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

type jsonDiag struct {
	Level     string `json:"level"`
	Check     string `json:"check"`
	Message   string `json:"message"`
	Path      string `json:"path,omitempty"`
	Line      int    `json:"line,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
	Fixable   bool   `json:"fixable,omitempty"`
}

type jsonCounts struct {
	Skills   int `json:"skills"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
}

type jsonReport struct {
	Skills  map[string]map[string][]jsonDiag `json:"skills"`
	Agents  map[string][]jsonDiag            `json:"agents"`
	Fixes   []string                         `json:"fixes,omitempty"`
	Summary jsonCounts                       `json:"summary"`
}

// levelSymbol maps a level to its text-report glyph.
func levelSymbol(l Level) string {
	switch l {
	case LevelError:
		return "✗" // ✗
	case LevelWarning:
		return "⚠" // ⚠
	case LevelInfo:
		return "ℹ" // ℹ
	default:
		return "?"
	}
}

// WriteText renders r as a human-readable report to w.
func WriteText(w io.Writer, r *Result) {
	for _, name := range sortedKeys(r.skills) {
		if name == CrossSkillKey {
			continue
		}
		writef(w, "\nskills/%s\n", name)
		writeDiags(w, r.skills[name].All())
	}
	if cross := r.skills[CrossSkillKey]; cross != nil && len(cross.All()) > 0 {
		writef(w, "\ncross-skill\n")
		writeDiags(w, cross.All())
	}
	for _, name := range sortedAgentKeys(r.agents) {
		writef(w, "\nagents/%s\n", name)
		writeDiags(w, r.agents[name])
	}
	writeSummary(w, r.Counts())
}

// WriteJSON renders r as indented JSON to w. fixes, when non-empty, is emitted
// as a "fixes" array (the descriptions of applied --fix changes).
func WriteJSON(w io.Writer, r *Result, fixes []string) error {
	report := jsonReport{
		Skills:  make(map[string]map[string][]jsonDiag),
		Agents:  make(map[string][]jsonDiag),
		Fixes:   fixes,
		Summary: jsonCounts(r.Counts()),
	}
	for name, sd := range r.skills {
		cats := make(map[string][]jsonDiag)
		addCat(cats, string(CategoryRedlines), sd.Redlines)
		addCat(cats, string(CategorySpec), sd.Spec)
		addCat(cats, string(CategoryQuality), sd.Quality)
		addCat(cats, string(CategoryDisclosure), sd.Disclosure)
		report.Skills[name] = cats
	}
	for name, diags := range r.agents {
		report.Agents[name] = toJSONDiags(diags)
	}

	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return &book2skill.Error{Op: "skilllint.WriteJSON", Err: err}
	}
	writef(w, "%s\n", b)
	return nil
}

func addCat(cats map[string][]jsonDiag, name string, diags []Diagnostic) {
	if len(diags) > 0 {
		cats[name] = toJSONDiags(diags)
	}
}

func toJSONDiags(diags []Diagnostic) []jsonDiag {
	out := make([]jsonDiag, len(diags))
	for i, d := range diags {
		out[i] = jsonDiag{
			Level:     string(d.Level),
			Check:     d.Check,
			Message:   d.Message,
			Path:      d.Path,
			Line:      d.Line,
			SourceURL: d.SourceURL,
			Fixable:   d.Fixable,
		}
	}
	return out
}

func writeDiags(w io.Writer, diags []Diagnostic) {
	for _, d := range diags {
		writef(w, "  %s [%s] %s\n", levelSymbol(d.Level), d.Check, d.Message)
	}
}

func writeSummary(w io.Writer, c Counts) {
	writef(w, "\nsummary: %d skills, %d errors, %d warnings", c.Skills, c.Errors, c.Warnings)
	if c.Info > 0 {
		writef(w, ", %d info", c.Info)
	}
	writef(w, "\n")
}

func sortedKeys(m map[string]*SkillDiagnostics) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedAgentKeys(m map[string][]Diagnostic) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
