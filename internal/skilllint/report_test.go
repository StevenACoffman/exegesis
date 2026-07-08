package skilllint_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/skilllint"
)

func sampleResult() *skilllint.Result {
	r := skilllint.NewResult()
	r.Add("my-skill", skilllint.CategorySpec, skilllint.Diagnostic{
		Level:   skilllint.LevelError,
		Check:   "1b.name.missing",
		Message: "required field 'name' is missing",
		Path:    "my-skill/SKILL.md",
	})
	r.Add("my-skill", skilllint.CategoryRedlines, skilllint.Diagnostic{
		Level:   skilllint.LevelWarning,
		Check:   "rl.test-prompts.present",
		Message: "no test-prompts.json",
	})
	r.Add(skilllint.CrossSkillKey, skilllint.CategorySpec, skilllint.Diagnostic{
		Level: skilllint.LevelInfo, Check: "1g.duplicate-name", Message: "dup",
	})
	return r
}

func TestCountsAndExitCode(t *testing.T) {
	t.Parallel()
	r := sampleResult()
	c := r.Counts()
	if c.Skills != 1 { // _cross-skill excluded
		t.Errorf("Skills = %d, want 1", c.Skills)
	}
	if c.Errors != 1 || c.Warnings != 1 || c.Info != 1 {
		t.Errorf("counts = %+v, want 1/1/1", c)
	}
	if r.ExitCode(false) != 1 {
		t.Error("ExitCode(false) should be 1 with an error present")
	}

	clean := skilllint.NewResult()
	clean.Add(
		"s",
		skilllint.CategoryQuality,
		skilllint.Diagnostic{Level: skilllint.LevelWarning, Check: "x", Message: "m"},
	)
	if clean.ExitCode(false) != 0 {
		t.Error("ExitCode(false) should be 0 with only warnings")
	}
	if clean.ExitCode(true) != 1 {
		t.Error("ExitCode(true) should be 1 with warnings under strict")
	}
}

func TestWriteJSONShape(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := skilllint.WriteJSON(
		&buf,
		sampleResult(),
		[]string{"lowercased name 'X' to 'x'"},
	); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var doc struct {
		Skills  map[string]map[string][]map[string]any `json:"skills"`
		Agents  map[string]any                         `json:"agents"`
		Summary map[string]int                         `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	spec := doc.Skills["my-skill"]["spec"]
	if len(spec) != 1 || spec[0]["check"] != "1b.name.missing" {
		t.Errorf("spec category missing expected diagnostic: %v", spec)
	}
	// Empty optional fields must be omitted (line/source_url/fixable absent).
	if _, ok := spec[0]["line"]; ok {
		t.Error("empty line should be omitted from JSON")
	}
	if doc.Summary["errors"] != 1 {
		t.Errorf("summary.errors = %d, want 1", doc.Summary["errors"])
	}
}

func TestWriteJSONFixes(t *testing.T) {
	t.Parallel()
	var withFixes bytes.Buffer
	if err := skilllint.WriteJSON(&withFixes, sampleResult(), []string{"renamed dir"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if !strings.Contains(withFixes.String(), `"fixes"`) {
		t.Error("expected a fixes array when fixes are present")
	}

	var noFixes bytes.Buffer
	if err := skilllint.WriteJSON(&noFixes, sampleResult(), nil); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if strings.Contains(noFixes.String(), `"fixes"`) {
		t.Error("fixes array must be omitted when empty")
	}
}

func TestWriteText(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	skilllint.WriteText(&buf, sampleResult())
	out := buf.String()
	for _, want := range []string{"skills/my-skill", "cross-skill", "1b.name.missing", "summary: 1 skills, 1 errors"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q\n%s", want, out)
		}
	}
}
