package skilllint

// Diagnostic categories attached to a skill.
const (
	// CategoryRedlines holds book2skill Quality Red Line findings.
	CategoryRedlines Category = "redlines"
	// CategorySpec holds agentskills.io spec-compliance findings.
	CategorySpec Category = "spec"
	// CategoryQuality holds practical quality findings.
	CategoryQuality Category = "quality"
	// CategoryDisclosure holds progressive-disclosure findings.
	CategoryDisclosure Category = "disclosure"

	// CrossSkillKey is the synthetic skill name holding cross-skill findings.
	CrossSkillKey = "_cross-skill"
)

// Category names a group of per-skill diagnostics.
type Category string

// Counts summarizes a Result.
type Counts struct {
	Skills   int
	Errors   int
	Warnings int
	Info     int
}

// SkillDiagnostics holds one skill's diagnostics grouped by category.
type SkillDiagnostics struct {
	Redlines   []Diagnostic
	Spec       []Diagnostic
	Quality    []Diagnostic
	Disclosure []Diagnostic
}

// Result accumulates all diagnostics from a lint run.
type Result struct {
	skills map[string]*SkillDiagnostics
	agents map[string][]Diagnostic
}

// NewResult returns an empty Result.
func NewResult() *Result {
	return &Result{
		skills: make(map[string]*SkillDiagnostics),
		agents: make(map[string][]Diagnostic),
	}
}

// All returns the skill's diagnostics across categories, in category order.
func (s *SkillDiagnostics) All() []Diagnostic {
	out := make([]Diagnostic, 0, len(s.Redlines)+len(s.Spec)+len(s.Quality)+len(s.Disclosure))
	out = append(out, s.Redlines...)
	out = append(out, s.Spec...)
	out = append(out, s.Quality...)
	out = append(out, s.Disclosure...)
	return out
}

// Skill returns the diagnostics bucket for name, creating it if needed.
func (r *Result) Skill(name string) *SkillDiagnostics {
	sd := r.skills[name]
	if sd == nil {
		sd = &SkillDiagnostics{}
		r.skills[name] = sd
	}
	return sd
}

// Add appends diags to the given category of the named skill.
func (r *Result) Add(name string, cat Category, diags ...Diagnostic) {
	if len(diags) == 0 {
		return
	}
	sd := r.Skill(name)
	switch cat {
	case CategoryRedlines:
		sd.Redlines = append(sd.Redlines, diags...)
	case CategorySpec:
		sd.Spec = append(sd.Spec, diags...)
	case CategoryQuality:
		sd.Quality = append(sd.Quality, diags...)
	case CategoryDisclosure:
		sd.Disclosure = append(sd.Disclosure, diags...)
	case CategoryAgents:
		// Agent findings are keyed by agent, not skill; use AddAgent instead.
	}
}

// AddAgent appends agent-compatibility diagnostics under an agent name.
func (r *Result) AddAgent(name string, diags ...Diagnostic) {
	r.agents[name] = append(r.agents[name], diags...)
}

// Counts tallies skills and diagnostics by level.
func (r *Result) Counts() Counts {
	var c Counts
	for name, sd := range r.skills {
		if name == "" || name[0] != '_' {
			c.Skills++
		}
		for _, d := range sd.All() {
			c.tally(d.Level)
		}
	}
	for _, diags := range r.agents {
		for _, d := range diags {
			c.tally(d.Level)
		}
	}
	return c
}

// ExitCode is 1 when errors exist, 1 when strict and warnings exist, else 0.
func (r *Result) ExitCode(strict bool) int {
	c := r.Counts()
	switch {
	case c.Errors > 0:
		return 1
	case strict && c.Warnings > 0:
		return 1
	default:
		return 0
	}
}

func (c *Counts) tally(level Level) {
	switch level {
	case LevelError:
		c.Errors++
	case LevelWarning:
		c.Warnings++
	case LevelInfo:
		c.Info++
	default:
	}
}
