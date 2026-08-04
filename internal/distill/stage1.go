package distill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/skillet/identity"
)

// stageExtract is the stage name for the parallel-extraction round.
const stageExtract = "extract"

// candidatesDir is the per-tree directory holding the extractors' audit output.
const candidatesDir = "candidates"

// extractBase is the shared instruction prefix for every extractor lens.
const extractBase = "You are one of five parallel extractors distilling a book into candidate units. " +
	"Reply ONLY with JSON matching the schema. Each unit needs a short title, its type, a source_chapter, " +
	"a source_quote (the original text, at most 150 characters), and a body in your own words. When a " +
	"unit's ownership is ambiguous, extract it anyway — a later stage dedups. Stay within your lens."

// stage1Schema is the JSON Schema each extractor reply must satisfy.
const stage1Schema = `{
  "type": "object",
  "required": ["units"],
  "properties": {
    "units": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["title", "type", "body"],
        "properties": {
          "id": {"type": "string"},
          "title": {"type": "string"},
          "type": {"type": "string"},
          "source_chapter": {"type": "string"},
          "source_quote": {"type": "string", "description": "original quote, at most 150 chars"},
          "body": {"type": "string"}
        }
      }
    }
  }
}`

// extractor is one of the five parallel extraction lenses: its candidate type
// and the focus phrase spliced into the prompt.
type extractor struct {
	typ  string
	role string
}

// CandidateUnit is one extracted unit (the methodology's minimum fields).
type CandidateUnit struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Type          string `json:"type"`
	SourceChapter string `json:"source_chapter"`
	SourceQuote   string `json:"source_quote"`
	Body          string `json:"body"`
}

// Stage1Response is one extractor's reply.
type Stage1Response struct {
	Units []CandidateUnit `json:"units"`
}

// extractors returns the five lenses. It is a function rather than a package
// variable so there is no mutable global state.
func extractors() []extractor {
	return []extractor{
		{"frameworks", "mental models, decision frameworks, and reasoning methods"},
		{"principles", "principles, rules, checklists, and assertions"},
		{"cases", "concrete examples and case studies the author personally uses"},
		{"counter-examples", "failure patterns, counterexamples, traps, and warnings"},
		{"glossary", "key terms and concepts in the author's own usage, with definitions"},
	}
}

// extractPrompt builds the PromptRequest for one extractor over the book and the
// Stage-0 overview. The content-address folds in the lens, overview, and book,
// so extraction re-runs if the overview changes. ResponsePath is left for the
// shell to fill from the cache.
func extractPrompt(e extractor, book, overviewText string) PromptRequest {
	user := "Overview:\n\n" + overviewText + "\n\nExtract " + e.role +
		" from this book.\n\n<book>\n" + book + "\n</book>"
	id := identity.Hash(stageExtract + "\x00" + e.typ + "\x00" + overviewText + "\x00" + book)
	return PromptRequest{
		ID: id,
		Messages: []Message{
			{Role: "system", Content: extractBase + " Your lens: " + e.role + "."},
			{Role: "user", Content: user},
		},
		Schema: json.RawMessage(stage1Schema),
	}
}

// renderCandidates renders units into the candidates/<type>.md audit file. Pure.
func renderCandidates(typ string, units []CandidateUnit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s candidates\n\n", typ)
	if len(units) == 0 {
		b.WriteString("_No candidates extracted._\n")
		return b.String()
	}
	for i := range units {
		u := &units[i]
		heading := u.Title
		if u.ID != "" {
			heading = u.ID + " — " + u.Title
		}
		fmt.Fprintf(&b, "## %s\n\n", heading)
		if u.SourceChapter != "" {
			fmt.Fprintf(&b, "- **source:** %s\n\n", u.SourceChapter)
		}
		if u.SourceQuote != "" {
			fmt.Fprintf(&b, "> %s\n\n", u.SourceQuote)
		}
		fmt.Fprintf(&b, "%s\n\n", u.Body)
	}
	return b.String()
}

// stage1Step is the extract step: it emits the extractor prompts whose
// candidates/<type>.md is not yet written, and writes that file when the prompt
// is answered. It returns nil once all five candidate files exist.
func stage1Step(pc *pipeline) ([]PromptRequest, error) {
	overviewText, err := os.ReadFile(filepath.Join(pc.tree, overviewFile))
	if err != nil {
		return nil, fmt.Errorf("read overview: %w", err)
	}
	var pending []PromptRequest
	for _, e := range extractors() {
		path := filepath.Join(pc.tree, candidatesDir, e.typ+".md")
		if _, statErr := os.Stat(path); statErr == nil {
			continue
		}
		p := extractPrompt(e, pc.book, string(overviewText))
		p.ResponsePath = pc.cache.Path(p.ID)
		if !pc.cache.Has(p.ID) {
			pending = append(pending, p)
			continue
		}
		r, decErr := decode[Stage1Response](pc.cache, p.ID, stageExtract)
		if decErr != nil {
			return nil, decErr
		}
		if err := writeArtifact(path, renderCandidates(e.typ, r.Units)); err != nil {
			return nil, err
		}
	}
	return pending, nil
}
