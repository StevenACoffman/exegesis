package skilllint

// Diagnostic severity levels.
const (
	// LevelError is a spec violation; its presence makes the run fail.
	LevelError Level = "error"
	// LevelWarning is a likely problem; it fails the run only under --strict.
	LevelWarning Level = "warning"
	// LevelInfo is advisory and never fails the run.
	LevelInfo Level = "info"
)

// Level is the severity of a Diagnostic.
type Level string

// Diagnostic is one finding about a skill or an agent configuration. It mirrors
// skillscheck's diagnostic record so check IDs and severities stay comparable.
type Diagnostic struct {
	Level     Level
	Check     string // stable check ID, e.g. "1b.name.dir-match" or "rl.segments.present"
	Message   string
	Path      string // file or directory the finding concerns; empty means unset
	Line      int    // 1-based; 0 means unset
	SourceURL string
	Fixable   bool
}

// Valid reports whether l is one of the known severity levels.
func (l Level) Valid() bool {
	switch l {
	case LevelError, LevelWarning, LevelInfo:
		return true
	default:
		return false
	}
}
