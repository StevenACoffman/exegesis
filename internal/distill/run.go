package distill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/skillet/atomicfile"
)

// cacheDirName is the per-tree directory holding the content-addressed responses.
const cacheDirName = ".exegesis"

// pipeline is the per-round shell state the stages read and write.
type pipeline struct {
	tree  string
	book  string
	cache *Cache
}

// stageStep advances one stage: it returns the prompts the agent must still
// answer (which stops the round), or nil when the stage is already satisfied
// (so the pipeline continues to the next stage). A step performs its own file
// I/O; all computation stays in the pure builders and renderers.
type stageStep func(pc *pipeline) ([]PromptRequest, error)

// namedStage pairs a stage's Outcome name with its step.
type namedStage struct {
	name string
	step stageStep
}

// stages returns the ordered pipeline. Run walks it until a stage emits prompts
// or every stage is satisfied.
func stages() []namedStage {
	return []namedStage{
		{stageOverview, stage0Step},
		{stageExtract, stage1Step},
		{stageConstruct, stage2Step},
	}
}

// Run advances the pipeline one round for the book at bookPath, writing
// artifacts under tree, and returns the Outcome to print. resume is the exact
// command the agent re-runs after answering the prompts. It walks the stages in
// order: the first stage that still needs prompts stops the round; when every
// stage is satisfied the pipeline is complete.
func Run(tree, bookPath, resume string) (Outcome, error) {
	book, err := os.ReadFile(bookPath)
	if err != nil {
		return Outcome{}, fmt.Errorf("read book %s: %w", bookPath, err)
	}
	pc := &pipeline{
		tree:  tree,
		book:  string(book),
		cache: NewCache(filepath.Join(tree, cacheDirName)),
	}
	if err := os.MkdirAll(pc.cache.dir, 0o755); err != nil {
		return Outcome{}, fmt.Errorf("create cache dir: %w", err)
	}
	for _, st := range stages() {
		emit, err := st.step(pc)
		if err != nil {
			return Outcome{}, err
		}
		if len(emit) > 0 {
			return Outcome{
				Status:  StatusNeedsPrompts,
				Stage:   st.name,
				Prompts: emit,
				Resume:  resume,
			}, nil
		}
	}
	return Outcome{Status: StatusComplete, Summary: "skill tree complete"}, nil
}

// decode reads prompt id's cached answer and parses it into T. what names the
// stage for the error message.
func decode[T any](cache *Cache, id, what string) (T, error) {
	var v T
	b, err := cache.Read(id)
	if err != nil {
		return v, err
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return v, fmt.Errorf("parse %s response %s: %w", what, id, err)
	}
	return v, nil
}

// writeArtifact atomically writes content to path, creating parent directories.
func writeArtifact(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	if err := atomicfile.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
