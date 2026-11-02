package lint

import (
	"path/filepath"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/skill"
	"github.com/StevenACoffman/skillet/speclint"
)

// DefaultMaxLines is the line gate's default: speclint.SkillMaxLines, agentskills.io's
// recommended SKILL.md ceiling. The body is loaded whole when a skill activates, so detail
// belongs in reference files the body names.
//
// It is enforced rather than advisory because both harnesses the family supports pay for
// an oversized skill: Claude Code re-attaches only the first 5,000 tokens of each skill
// after compaction, and a 32k local context cannot hold one and still do the work. The
// number and the counting are skillet's, so exegesis and skillsaw cannot disagree.
const DefaultMaxLines = speclint.SkillMaxLines

// checkSize applies the line gate to s, honouring the ratchet of recorded ceilings.
//
// A skill named in ceilings is a known exception (D7): it may stay over limit but must not
// grow past the ceiling recorded for it, and is reminded to lower the ceiling when it has
// shrunk, so the exception list can only get shorter. The ceiling is keyed by the skill's
// folder, which is the slug verify reports it under.
//
// Requires: limit >= 0; 0 disables the gate.
// Ensures:  at most one diagnostic; an error iff s exceeds limit and either has no ceiling
//
//	or exceeds its ceiling; it is pure.
func checkSize(s *skill.Skill, limit int, ceilings map[string]int) []finding.Diagnostic {
	if limit <= 0 {
		return nil
	}
	n := speclint.Lines(s.Raw)
	ceiling, excepted := ceilings[filepath.Base(s.Dir)]
	switch {
	case excepted && n <= limit:
		return []finding.Diagnostic{
			warnf(
				"size: SKILL.md is %d lines, within the %d-line limit; remove its line_ceilings entry",
				n,
				limit,
			),
		}
	case excepted && n > ceiling:
		return []finding.Diagnostic{
			diagf(
				"size: SKILL.md is %d lines, past its recorded ceiling of %d; an excepted skill may only shrink",
				n,
				ceiling,
			),
		}
	case excepted && n < ceiling:
		return []finding.Diagnostic{warnf(
			"size: SKILL.md is %d lines > %d (known exception); lower its ceiling from %d to %d",
			n, limit, ceiling, n)}
	case excepted:
		return []finding.Diagnostic{
			warnf(
				"size: SKILL.md is %d lines > %d (known exception at its ceiling; split it into reference files)",
				n,
				limit,
			),
		}
	case n > limit:
		return []finding.Diagnostic{
			diagf(
				"size: SKILL.md is %d lines > %d (agentskills.io: keep SKILL.md under %d lines; move detail into reference files)",
				n,
				limit,
				limit,
			),
		}
	}
	return nil
}

func warnf(format string, a ...any) finding.Diagnostic {
	d := diagf(format, a...)
	d.Severity = finding.SeverityWarning
	return d
}
