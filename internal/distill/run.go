package distill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenACoffman/exegesis/internal/overview"
	"github.com/StevenACoffman/skillet/atomicfile"
)

// cacheDirName is the per-tree directory holding the content-addressed responses.
const cacheDirName = ".exegesis"

// overviewFile is the Stage-0 artifact.
const overviewFile = "BOOK_OVERVIEW.md"

// Run advances the pipeline one round for the book at bookPath, writing artifacts
// under tree, and returns the Outcome to print. resume is the exact command the
// agent should re-run after answering the prompts. It performs the round's file
// I/O and delegates all computation to the pure builders/renderers.
//
// Phase 1 handles Stage 0 only: it returns StatusComplete once a gate-passing
// BOOK_OVERVIEW.md exists, and StatusNeedsPrompts (the overview prompt) until then.
func Run(tree, bookPath, resume string) (Outcome, error) {
	if md, err := os.ReadFile(filepath.Join(tree, overviewFile)); err == nil &&
		len(overview.Check(string(md))) == 0 {
		return Outcome{Status: StatusComplete, Summary: overviewFile + " gate passed"}, nil
	}
	book, err := os.ReadFile(bookPath)
	if err != nil {
		return Outcome{}, fmt.Errorf("read book %s: %w", bookPath, err)
	}
	cache := NewCache(filepath.Join(tree, cacheDirName))
	if err := os.MkdirAll(cache.dir, 0o755); err != nil {
		return Outcome{}, fmt.Errorf("create cache dir: %w", err)
	}
	return stage0(tree, string(book), cache, resume)
}

// stage0 walks the Stage-0 correction chain from the cache: it builds the
// overview prompt for the accumulated feedback history, emits it when the agent
// has not answered yet, and otherwise renders and gates the answer — completing
// on a pass or extending the history and advancing on a fail. The history grows
// every failed round, so each prompt has a distinct content-address and the walk
// always terminates (at an unanswered prompt or a passing one).
func stage0(tree, book string, cache *Cache, resume string) (Outcome, error) {
	var history []string
	for attempt := 1; ; attempt++ {
		p := stage0Prompt(book, history)
		p.ResponsePath = cache.Path(p.ID)
		if !cache.Has(p.ID) {
			return Outcome{
				Status:  StatusNeedsPrompts,
				Stage:   stageOverview,
				Prompts: []PromptRequest{p},
				Resume:  resume,
			}, nil
		}
		r, err := decodeStage0(cache, p.ID)
		if err != nil {
			return Outcome{}, err
		}
		md := renderOverview(&r)
		problems := overview.Check(md)
		if len(problems) == 0 {
			if err := writeOverview(tree, md); err != nil {
				return Outcome{}, err
			}
			return Outcome{
				Status:  StatusComplete,
				Summary: "wrote " + filepath.Join(tree, overviewFile),
			}, nil
		}
		history = append(
			history,
			fmt.Sprintf("attempt %d: %s", attempt, strings.Join(problems, "; ")),
		)
	}
}

// decodeStage0 reads and parses the agent's cached overview reply.
func decodeStage0(cache *Cache, id string) (Stage0Response, error) {
	b, err := cache.Read(id)
	if err != nil {
		return Stage0Response{}, err
	}
	var r Stage0Response
	if err := json.Unmarshal(b, &r); err != nil {
		return Stage0Response{}, fmt.Errorf("parse overview response %s: %w", id, err)
	}
	return r, nil
}

// writeOverview atomically writes the rendered overview into tree.
func writeOverview(tree, md string) error {
	path := filepath.Join(tree, overviewFile)
	if err := atomicfile.WriteFile(path, []byte(md), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
