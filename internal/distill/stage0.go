package distill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/exegesis/internal/overview"
	"github.com/StevenACoffman/skillet/identity"
)

// stageOverview is the stage name reported in the Outcome and mixed into the
// Stage-0 prompt's content address.
const stageOverview = "overview"

// overviewFile is the Stage-0 artifact written under the tree.
const overviewFile = "BOOK_OVERVIEW.md"

// stage0System instructs the model to distil a book's overview following Adler's
// steps and to satisfy the deterministic Stage-0 gate.
const stage0System = "You distil a book into a structured overview using Mortimer Adler's " +
	"analytical-reading steps (structure, interpretation, critique). Reply ONLY with JSON matching " +
	"the given schema. The skeleton must have 3-7 primary arguments; key_terms at least 5 terms in " +
	"the author's own usage; and era_limitations, author_blind_spots and unproven_assumptions must " +
	"total at least 3 critique items. The summary must be exactly one sentence."

// stage0Schema is the JSON Schema the agent's overview reply must satisfy.
const stage0Schema = `{
  "type": "object",
  "required": ["title","summary","skeleton","key_terms","era_limitations","author_blind_spots","unproven_assumptions"],
  "properties": {
    "title": {"type": "string"},
    "author": {"type": "string"},
    "year": {"type": "string"},
    "genre": {"type": "string"},
    "summary": {"type": "string", "description": "exactly one sentence"},
    "skeleton": {"type": "array", "items": {"type": "string"}, "minItems": 3, "maxItems": 7},
    "key_terms": {"type": "array", "items": {"type": "string"}, "minItems": 5},
    "propositions": {"type": "array", "items": {"type": "string"}},
    "era_limitations": {"type": "array", "items": {"type": "string"}},
    "author_blind_spots": {"type": "array", "items": {"type": "string"}},
    "unproven_assumptions": {"type": "array", "items": {"type": "string"}}
  }
}`

// Stage0Response is the JSON the agent returns for the overview prompt; its
// fields become the BOOK_OVERVIEW.md sections.
type Stage0Response struct {
	Title          string   `json:"title"`
	Author         string   `json:"author"`
	Year           string   `json:"year"`
	Genre          string   `json:"genre"`
	Summary        string   `json:"summary"`
	Skeleton       []string `json:"skeleton"`
	KeyTerms       []string `json:"key_terms"`
	Propositions   []string `json:"propositions"`
	EraLimitations []string `json:"era_limitations"`
	BlindSpots     []string `json:"author_blind_spots"`
	Assumptions    []string `json:"unproven_assumptions"`
}

// stage0Prompt builds the overview PromptRequest for a book. feedback (gate
// problems from a prior attempt) is folded into both the user message and the
// content-addressed ID, so a correction round is a distinct cache entry.
// ResponsePath is left empty for the shell to fill from the cache.
//
// Ensures: the ID is stable for a given (bookText, feedback) and changes when
// either changes.
func stage0Prompt(bookText string, feedback []string) PromptRequest {
	user := "Distil this book into the overview JSON.\n\n<book>\n" + bookText + "\n</book>"
	if len(feedback) > 0 {
		user += "\n\nA previous attempt failed these gate checks; fix them:\n- " +
			strings.Join(feedback, "\n- ")
	}
	id := identity.Hash(stageOverview + "\x00" + bookText + "\x00" + strings.Join(feedback, "\x00"))
	return PromptRequest{
		ID: id,
		Messages: []Message{
			{Role: "system", Content: stage0System},
			{Role: "user", Content: user},
		},
		Schema: json.RawMessage(stage0Schema),
	}
}

// renderOverview renders r into BOOK_OVERVIEW.md text whose "##" headings the
// overview gate parses. Pure; r is not mutated.
func renderOverview(r *Stage0Response) string {
	var b strings.Builder
	title := r.Title
	if title == "" {
		title = "Book"
	}
	fmt.Fprintf(&b, "# %s — Book Overview\n\n", title)
	fmt.Fprintf(
		&b,
		"- **Author:** %s\n- **Year:** %s\n- **Genre:** %s\n\n",
		r.Author,
		r.Year,
		r.Genre,
	)
	fmt.Fprintf(&b, "## One-sentence summary\n\n%s\n\n", r.Summary)
	bulletSection(&b, "Skeleton", r.Skeleton)
	bulletSection(&b, "Key terms", r.KeyTerms)
	bulletSection(&b, "Core propositions", r.Propositions)
	bulletSection(&b, "Era limitations", r.EraLimitations)
	bulletSection(&b, "Author blind spots", r.BlindSpots)
	bulletSection(&b, "Unproven assumptions", r.Assumptions)
	return b.String()
}

// bulletSection writes a "## <title>" heading followed by one "- " bullet per
// item (or an empty section when items is empty).
func bulletSection(b *strings.Builder, title string, items []string) {
	fmt.Fprintf(b, "## %s\n\n", title)
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", it)
	}
	b.WriteString("\n")
}

// stage0Step is the Stage-0 step: it returns nil once a gate-passing
// BOOK_OVERVIEW.md exists, and the overview prompt until then. It walks the
// correction chain from the cache — building the prompt for the accumulated
// feedback history, emitting it when unanswered, and otherwise rendering and
// gating the answer. The history grows every failed round, so each prompt has a
// distinct content-address and the walk always terminates.
func stage0Step(pc *pipeline) ([]PromptRequest, error) {
	if md, err := os.ReadFile(filepath.Join(pc.tree, overviewFile)); err == nil &&
		len(overview.Check(string(md))) == 0 {
		return nil, nil
	}
	var history []string
	for attempt := 1; ; attempt++ {
		p := stage0Prompt(pc.book, history)
		p.ResponsePath = pc.cache.Path(p.ID)
		if !pc.cache.Has(p.ID) {
			return []PromptRequest{p}, nil
		}
		r, err := decode[Stage0Response](pc.cache, p.ID, stageOverview)
		if err != nil {
			return nil, err
		}
		md := renderOverview(&r)
		problems := overview.Check(md)
		if len(problems) == 0 {
			if err := writeArtifact(filepath.Join(pc.tree, overviewFile), md); err != nil {
				return nil, err
			}
			return nil, nil
		}
		history = append(
			history,
			fmt.Sprintf("attempt %d: %s", attempt, strings.Join(problems, "; ")),
		)
	}
}
