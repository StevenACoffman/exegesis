// Package mergestatus reads and appends a source skill's merge ledger: the
// `## Merge Status` section recording what each merge run decided about that skill.
//
// The ledger is an audit trail, so appending is the only write. Prior entries are never
// re-rendered, let alone edited: a new entry is spliced in as text ahead of the closing
// fence, which makes "append-only" a property of the code rather than a rule a caller is
// asked to respect.
//
// It lives in the body rather than the frontmatter because `merge_status` is not a
// spec-allowed frontmatter key and would fail `exegesis lint` on the source skill.
//
// Everything here is pure; the command reads and writes the files.
package mergestatus

import (
	"fmt"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// Heading is the section the ledger lives in, in the form this package writes.
//
// Title case because `rumdl` enforces it (MD063) and would otherwise rewrite the
// heading on the next format pass, leaving the tool and the formatter fighting over one
// line. Readers match it case-insensitively, so a ledger written either way is found.
const Heading = "## Merge Status"

// fenceLine opens and closes the ledger's YAML block.
const fenceLine = "```yaml"

// Entry is one merge run's verdict on one source skill.
type Entry struct {
	Run      string `yaml:"run"`
	State    string `yaml:"state"`
	Pair     string `yaml:"pair,omitempty"`
	Into     string `yaml:"into,omitempty"`
	Reason   string `yaml:"reason,omitempty"`
	Excluded string `yaml:"excluded,omitempty"`
}

// States are the fates a merge run can assign, and the fields each one requires.
//
// The map is the vocabulary and the required-field list at once, so the two cannot
// disagree — the same reason skillet's testprompts.Composition holds its case types and
// their minimums in one value.
func States() map[string][]string {
	return map[string][]string{
		"no-candidate":        {},
		"surface-resemblance": {"pair"},
		"complementary":       {"pair"},
		"rejected":            {"pair", "reason"},
		"partial":             {"into", "excluded"},
		"merged":              {"into"},
	}
}

// Reasons are the codes valid with state "rejected".
func Reasons() map[string]bool {
	return map[string]bool{
		"source-text-unavailable":      true,
		"source-verification-failed":   true,
		"v1-failed":                    true,
		"v2-failed":                    true,
		"v3-failed":                    true,
		"v4-failed-merge-not-additive": true,
	}
}

// Validate reports every way e departs from the ledger schema, empty when it conforms.
//
// A field is reported when it is required by the state and missing, and equally when it
// is present and the state has no use for it. Both directions matter for an audit trail:
// a missing field loses the reason a fate was assigned, and a stray one records a claim
// the state cannot support — a "rejected" entry naming what it merged into is not a
// harmless extra, it is two contradictory accounts of the same decision.
//
// Ensures: the result is deterministically ordered; it is pure.
func (e *Entry) Validate() []string {
	var problems []string
	if strings.TrimSpace(e.Run) == "" {
		problems = append(problems, "run is required")
	}
	required, known := States()[e.State]
	if !known {
		return append(problems, fmt.Sprintf("unknown state %q (known: %s)",
			e.State, strings.Join(stateNames(), ", ")))
	}
	need := make(map[string]bool, len(required))
	for _, f := range required {
		need[f] = true
	}
	problems = append(problems, e.fieldProblems(need)...)
	if e.Reason != "" && need["reason"] && !Reasons()[e.Reason] {
		problems = append(problems, fmt.Sprintf("unknown reason %q (known: %s)",
			e.Reason, strings.Join(reasonNames(), ", ")))
	}
	return problems
}

// Render returns e as one YAML list item, indented for the ledger block.
//
// Marshalled rather than formatted by hand: "excluded" is free text that can carry a
// colon or a quote, and hand-built YAML would emit a document that no longer parses.
func (e *Entry) Render() (string, error) {
	b, err := yaml.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("mergestatus: render entry: %w", err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	for i, line := range lines {
		if i == 0 {
			lines[i] = "- " + line
			continue
		}
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n"), nil
}

// fieldProblems reports each optional field that the state requires and e lacks, or
// that e carries and the state has no use for.
func (e *Entry) fieldProblems(need map[string]bool) []string {
	var problems []string
	for _, f := range []string{"pair", "into", "reason", "excluded"} {
		value := strings.TrimSpace(e.field(f))
		switch {
		case need[f] && value == "":
			problems = append(problems, fmt.Sprintf("state %q requires %s", e.State, f))
		case !need[f] && value != "":
			problems = append(problems, fmt.Sprintf("state %q does not take %s", e.State, f))
		}
	}
	return problems
}

// field returns the named optional field's value, or "" for a name that is not one.
func (e *Entry) field(name string) string {
	switch name {
	case "pair":
		return e.Pair
	case "into":
		return e.Into
	case "reason":
		return e.Reason
	case "excluded":
		return e.Excluded
	}
	return ""
}

// Parse returns the entries recorded in md's ledger. A skill with no ledger has never
// been evaluated in any merge run, which is not an error and yields no entries.
//
// Ensures: it is pure.
func Parse(md string) ([]Entry, error) {
	block, ok := blockOf(strings.Split(md, "\n"))
	if !ok || strings.TrimSpace(block) == "" {
		return nil, nil
	}
	var entries []Entry
	if err := yaml.Unmarshal([]byte(block), &entries); err != nil {
		return nil, fmt.Errorf("mergestatus: parse ledger: %w", err)
	}
	return entries, nil
}

// Append returns md with e added to its ledger, creating the section when absent.
//
// Existing entries are copied through byte for byte -- the new entry is spliced in ahead
// of the closing fence rather than the block being re-rendered -- so an append cannot
// reformat, reorder or lose a prior run's record even if this package's rendering later
// changes.
//
// Ensures: the result contains every byte of md's existing ledger entries; it is pure.
func Append(md string, e *Entry) (string, error) {
	rendered, err := e.Render()
	if err != nil {
		return "", err
	}
	lines := strings.Split(md, "\n")
	closeAt, ok := closingFenceOf(lines)
	if !ok {
		return appendSection(md, rendered), nil
	}
	out := make([]string, 0, len(lines)+2)
	out = append(out, lines[:closeAt]...)
	out = append(out, strings.Split(rendered, "\n")...)
	out = append(out, lines[closeAt:]...)
	return strings.Join(out, "\n"), nil
}

// appendSection returns md with a fresh ledger section holding one entry.
func appendSection(md, rendered string) string {
	return strings.TrimRight(md, "\n") + "\n\n" + Heading + "\n\n" +
		fenceLine + "\n" + rendered + "\n```\n"
}

// stateNames lists the state vocabulary in a stable order for a message.
func stateNames() []string {
	names := make([]string, 0, len(States()))
	for name := range States() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// reasonNames lists the reason vocabulary in a stable order for a message.
func reasonNames() []string {
	names := make([]string, 0, len(Reasons()))
	for name := range Reasons() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// sectionRange returns the [head, end) line range of the ledger section.
//
// The heading is matched case-insensitively: `rumdl` rewrites headings to title case, a
// hand-written ledger may use either, and a reader that saw only one spelling would
// report a populated ledger as absent -- then happily append a second one below it.
func sectionRange(lines []string) (head, end int, found bool) {
	head = -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if head < 0 {
			if strings.EqualFold(trimmed, Heading) {
				head = i
			}
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			return head, i, true
		}
	}
	if head < 0 {
		return -1, -1, false
	}
	return head, len(lines), true
}

// blockOf returns the YAML text inside the ledger section's fenced block.
func blockOf(lines []string) (string, bool) {
	head, end, ok := sectionRange(lines)
	if !ok {
		return "", false
	}
	open := -1
	for i := head; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if open < 0 {
			if strings.HasPrefix(trimmed, "```") {
				open = i
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			return strings.Join(lines[open+1:i], "\n"), true
		}
	}
	return "", false
}

// closingFenceOf returns the line index of the ledger block's closing fence.
func closingFenceOf(lines []string) (int, bool) {
	head, end, ok := sectionRange(lines)
	if !ok {
		return 0, false
	}
	open := -1
	for i := head; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if open < 0 {
			if strings.HasPrefix(trimmed, "```") {
				open = i
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			return i, true
		}
	}
	return 0, false
}
