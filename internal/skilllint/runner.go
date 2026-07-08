package skilllint

import "github.com/StevenACoffman/exegesis/internal/skilllint/agents"

// CategoryAgents selects the per-agent compatibility checks. Unlike the other
// categories its findings are stored under Result.agents, not per skill.
const CategoryAgents Category = "agents"

// Options configures a lint run.
type Options struct {
	// Categories selects which check groups run.
	Categories map[Category]bool
	// AgentNames selects agent adapters (nil/empty auto-detects; {"all"} forces all).
	AgentNames []string
	// Exclude holds base-name globs pruned from the filesystem walks.
	Exclude []string
}

// DefaultCategories is the general skill-linter default: the skillscheck-style
// checks, excluding the book2skill-specific Quality Red Lines.
func DefaultCategories() map[Category]bool {
	return map[Category]bool{
		CategorySpec:       true,
		CategoryQuality:    true,
		CategoryDisclosure: true,
		CategoryAgents:     true,
	}
}

// AllCategories is DefaultCategories plus the opt-in redlines category.
func AllCategories() map[Category]bool {
	cats := DefaultCategories()
	cats[CategoryRedlines] = true
	return cats
}

// Run discovers skills under root, parses them, and runs the selected checks.
func Run(root string, opts Options) (*Result, error) {
	dirs, err := Discover(root)
	if err != nil {
		return nil, err
	}
	skills := make([]*Skill, len(dirs))
	for i, dir := range dirs {
		skills[i] = Parse(dir)
	}

	var adapters []agents.Adapter
	extensionFields := map[string]bool{}
	if opts.Categories[CategoryAgents] {
		adapters = agents.Select(opts.AgentNames, root)
		extensionFields = agents.KnownFields(adapters)
	}

	count := newTokenCounter()
	r := NewResult()
	for _, s := range skills {
		runSkillChecks(r, s, &opts, extensionFields, count)
	}
	if opts.Categories[CategorySpec] {
		r.Add(CrossSkillKey, CategorySpec, CheckCrossSkill(skills)...)
	}
	if opts.Categories[CategoryAgents] {
		runAgentChecks(r, root, adapters, skills)
	}
	return r, nil
}

func runSkillChecks(
	r *Result,
	s *Skill,
	opts *Options,
	extensionFields map[string]bool,
	count TokenCounter,
) {
	cats := opts.Categories
	r.Skill(s.DirName) // register even when a skill has no diagnostics
	if cats[CategoryRedlines] {
		r.Add(s.DirName, CategoryRedlines, CheckRedlines(s)...)
	}
	if cats[CategorySpec] {
		r.Add(s.DirName, CategorySpec, CheckSpec(s, extensionFields, count)...)
	}
	if cats[CategoryQuality] {
		r.Add(s.DirName, CategoryQuality, CheckQuality(s, opts.Exclude)...)
	}
	if cats[CategoryDisclosure] {
		r.Add(s.DirName, CategoryDisclosure, CheckDisclosure(s, opts.Exclude, count)...)
	}
}

func runAgentChecks(r *Result, root string, adapters []agents.Adapter, skills []*Skill) {
	agentSkills := toAgentSkills(skills)
	for _, a := range adapters {
		for _, d := range a.Check(root, agentSkills) {
			r.AddAgent(a.Name(), fromAgentDiag(&d))
		}
	}
	for _, d := range agents.CrossAgent(root, adapters) {
		r.AddAgent("cross-agent", fromAgentDiag(&d))
	}
}

func toAgentSkills(skills []*Skill) []agents.Skill {
	out := make([]agents.Skill, len(skills))
	for i, s := range skills {
		out[i] = agents.Skill{
			DirName:     s.DirName,
			DirPath:     s.DirPath,
			SkillMDPath: s.SkillMDPath,
			Frontmatter: s.Frontmatter,
			Body:        s.Body,
		}
	}
	return out
}

func fromAgentDiag(d *agents.Diagnostic) Diagnostic {
	return Diagnostic{
		Level:     Level(d.Level),
		Check:     d.Check,
		Message:   d.Message,
		Path:      d.Path,
		SourceURL: d.SourceURL,
	}
}
