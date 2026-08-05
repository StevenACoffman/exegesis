package distill

import (
	"fmt"

	"github.com/StevenACoffman/exegesis/internal/indexgen"
)

// stageIndex is the deterministic index stage.
const stageIndex = "index"

// stage3Step regenerates tree/INDEX.md from the constructed skills' `## Related
// skills` sections. It is deterministic — it emits no prompts and always returns
// nil, so the pipeline proceeds straight to complete once the skills exist.
func stage3Step(pc *pipeline) ([]PromptRequest, error) {
	out, err := indexgen.Generate(pc.tree, "", "")
	if err != nil {
		return nil, fmt.Errorf("index: %w", err)
	}
	if err := writeArtifact(indexgen.Path(pc.tree), out); err != nil {
		return nil, err
	}
	return nil, nil
}
