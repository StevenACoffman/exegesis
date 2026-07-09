package book2skill

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// rawCase is one decoded test case before normalization: its arbitrary JSON
// fields, plus the type implied by a category-grouped array it came from ("" if
// it came from a flat list, where the type is read from the fields instead).
type rawCase struct {
	fields    map[string]any
	groupType TestType
}

// MigrateTestPrompts adapts a foreign test-prompts.json into the canonical
// darwin-shaped cases that EncodeTestPrompts produces and DecodeTestPrompts
// re-parses. It unwraps the common wrapper shapes (a top-level array, or an
// object keyed by test_cases/prompts/test_prompts/tests, or category-grouped
// should_trigger/should_not_trigger/edge_cases arrays), maps type/category
// synonyms to the canonical set, adopts the expected value from "expected" or
// any "expected_*" variant, renumbers ids sequentially, and preserves every
// other field in notes so nothing is dropped.
//
// It never fabricates a positive test's expected behavior: when a case has no
// expected field it is defaulted only for should_not_trigger (where "skill
// should not activate" is always correct) and otherwise left empty with a
// warning. Every gap that needs an author's attention — a missing prompt or
// expected, an unrecognized type — is returned as a warning rather than guessed.
func MigrateTestPrompts(raw []byte) ([]TestCase, []string, error) {
	var top any
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, nil, &Error{Code: EINVALID, Message: "test-prompts.json is not valid JSON"}
	}
	rawCases := locateCases(top)
	if len(rawCases) == 0 {
		return nil, nil, &Error{
			Code:    EINVALID,
			Message: "could not locate a test-case array in test-prompts.json",
		}
	}
	cases := make([]TestCase, 0, len(rawCases))
	var warnings []string
	for i := range rawCases {
		testCase, warns := normalizeCase(&rawCases[i], i+1)
		cases = append(cases, testCase)
		warnings = append(warnings, warns...)
	}
	return cases, warnings, nil
}

// locateCases finds the array of cases within any of the observed layouts.
func locateCases(top any) []rawCase {
	switch v := top.(type) {
	case []any:
		return wrapCases(v, "")
	case map[string]any:
		for _, key := range []string{"test_cases", "prompts", "test_prompts", "tests"} {
			switch inner := v[key].(type) {
			case []any:
				return wrapCases(inner, "")
			case map[string]any:
				return groupedCases(inner)
			}
		}
		return groupedCases(v)
	default:
		return nil
	}
}

// groupedCases collects cases from the category-grouped layout, tagging each
// with the type its array name implies.
func groupedCases(m map[string]any) []rawCase {
	groups := []struct {
		key string
		typ TestType
	}{
		{"should_trigger", ShouldTrigger},
		{"should_not_trigger", ShouldNotTrigger},
		{"edge_cases", EdgeCase},
		{"edge_case", EdgeCase},
	}
	var out []rawCase
	for _, g := range groups {
		if arr, ok := m[g.key].([]any); ok {
			out = append(out, wrapCases(arr, g.typ)...)
		}
	}
	return out
}

// wrapCases keeps only the object elements of arr, tagging each with group.
func wrapCases(arr []any, group TestType) []rawCase {
	out := make([]rawCase, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			out = append(out, rawCase{fields: m, groupType: group})
		}
	}
	return out
}

// normalizeCase maps one raw case to a canonical TestCase with the given id,
// returning any gaps that need an author's attention.
func normalizeCase(rc *rawCase, id int) (TestCase, []string) {
	prefix := "case " + strconv.Itoa(id) + ": "
	var warnings []string
	prompt := jsonString(rc.fields, "prompt")
	if prompt == "" {
		warnings = append(warnings, prefix+"missing prompt")
	}
	typ, typeWarn := deriveType(rc, prefix)
	if typeWarn != "" {
		warnings = append(warnings, typeWarn)
	}
	expected, expWarn := deriveExpected(rc.fields, typ, prefix)
	if expWarn != "" {
		warnings = append(warnings, expWarn)
	}
	return TestCase{
		ID:       id,
		Type:     typ,
		Prompt:   prompt,
		Expected: expected,
		Notes:    deriveNotes(rc.fields),
	}, warnings
}

// deriveType resolves the canonical type from the group hint or the type/
// category field, warning when it cannot be classified.
func deriveType(rc *rawCase, prefix string) (TestType, string) {
	if rc.groupType != "" {
		return rc.groupType, ""
	}
	raw := jsonString(rc.fields, "type")
	if raw == "" {
		raw = jsonString(rc.fields, "category")
	}
	typ := normalizeTestType(raw)
	switch {
	case typ.Valid():
		return typ, ""
	case raw == "":
		return typ, prefix + "no type/category — cannot classify"
	default:
		return typ, prefix + "unknown type " + strconv.Quote(raw) + " — left as-is"
	}
}

// normalizeTestType maps the type/category vocabularies seen in the wild onto
// the canonical TestType set. Unrecognized values are returned verbatim (and
// reported by the caller) rather than silently reclassified.
func normalizeTestType(raw string) TestType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "should_trigger", "should-trigger", "trigger", "should_invoke", "invoke":
		return ShouldTrigger
	case "should_not_trigger", "should-not-trigger", "should_not_invoke", "decoy":
		return ShouldNotTrigger
	case "edge_case", "edge-case", "edge", "blurred_boundary", "boundary":
		return EdgeCase
	case "prefer_merged_over_source", "prefer_merged":
		return PreferMergedOverSource
	default:
		return TestType(raw)
	}
}

// deriveExpected resolves the expected behavior, defaulting only the case where
// it is provably correct (should_not_trigger) and warning otherwise.
func deriveExpected(fields map[string]any, typ TestType, prefix string) (string, string) {
	if e := expectedFromFields(fields); e != "" {
		return e, ""
	}
	if typ == ShouldNotTrigger {
		return "skill should not activate", ""
	}
	return "", prefix + "no expected — author must supply"
}

// expectedFromFields returns the first non-empty expected value: the "expected"
// field, then any "expected_*" variant (expected_behavior, expected_skill_use,
// expected_answer_summary, …) in a deterministic alphabetical order. Skill sets
// in the wild carry the expectation under many such keys; reading them all keeps
// migration from discarding an expectation the author already wrote.
func expectedFromFields(fields map[string]any) string {
	if e := jsonText(fields, "expected"); e != "" {
		return e
	}
	variants := make([]string, 0)
	for k := range fields {
		if strings.HasPrefix(k, "expected_") {
			variants = append(variants, k)
		}
	}
	sort.Strings(variants)
	for _, k := range variants {
		if e := jsonText(fields, k); e != "" {
			return e
		}
	}
	return ""
}

// deriveNotes preserves every field not consumed into a canonical TestCase
// (id/type/category/prompt/expected*) as "key: value", plus the original id when
// it is a non-numeric label, in a deterministic order — so migration loses no
// authoring intent (rationale, must_include, segment, trigger, …) without
// passing any of it off as the expectation.
func deriveNotes(fields map[string]any) string {
	var parts []string
	if oid := jsonText(fields, "id"); oid != "" && !isIntString(oid) {
		parts = append(parts, "source id: "+oid)
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if !isConsumedField(k) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if v := jsonText(fields, k); v != "" {
			parts = append(parts, k+": "+v)
		}
	}
	return strings.Join(parts, "; ")
}

// isConsumedField reports whether key is mapped into a canonical TestCase field
// and so must not be duplicated into notes. The original id is preserved
// separately (only when non-numeric), so it counts as consumed here.
func isConsumedField(key string) bool {
	switch key {
	case "id", "type", "category", "prompt", "expected":
		return true
	default:
		return strings.HasPrefix(key, "expected_")
	}
}

// jsonString returns fields[key] when it is a string, else "".
func jsonString(fields map[string]any, key string) string {
	if v, ok := fields[key].(string); ok {
		return v
	}
	return ""
}

// jsonText renders fields[key] losslessly as text: a string as-is, a string
// array joined by ", ", and any other value (number, bool, nested array/object)
// as compact JSON. Missing keys render as "". This makes note preservation
// lossless for the arbitrary shapes migrated cases carry.
func jsonText(fields map[string]any, key string) string {
	v, ok := fields[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any:
		items := make([]string, 0, len(t))
		for _, it := range t {
			if s, isStr := it.(string); isStr {
				items = append(items, s)
			} else if b, err := json.Marshal(it); err == nil {
				items = append(items, string(b))
			}
		}
		return strings.Join(items, ", ")
	default:
		if b, err := json.Marshal(t); err == nil {
			return string(b)
		}
		return ""
	}
}

// isIntString reports whether s is a non-empty run of ASCII digits.
func isIntString(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
