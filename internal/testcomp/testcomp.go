// Package testcomp defines exegesis's test-prompts compositions: the standard book2skill
// mix and the merged-skill mix. Both `tests` and `verify` read them, so a merged tree is
// gated under the same rules whichever command runs — closing the drift that let `verify`
// reject `prefer_merged_over_source` (validating against the standard mix) while
// `tests --merge` accepted it. It is the composition profile in its smallest honest form;
// it earns promotion to skillet only on a consumer outside exegesis.
package testcomp

import (
	"fmt"

	"github.com/StevenACoffman/exegesis/internal/mergedfrom"
	"github.com/StevenACoffman/skillet/testprompts"
)

// PreferMerged is the case category unique to a merged skill: a prompt where the merged
// skill must be chosen over either source it came from. skillet does not know it exists;
// exegesis owns this policy and passes it in.
const PreferMerged = "prefer_merged_over_source"

const (
	// mergedEdgeMinimum is the edge_case floor for a merged skill: two, against the
	// standard one, because a merged skill inherits both parents' boundaries and one
	// edge case cannot cover where they meet.
	mergedEdgeMinimum = 2
	// preferMergedMinimum is how many prefer_merged_over_source cases a merged skill
	// must carry.
	preferMergedMinimum = 2
)

// For returns the case mix a skill is gated against: the merged-skill mix when merged is
// true, otherwise skillet's standard one. A caller passes true for every skill under
// --merge, and otherwise for a skill that declares metadata.merged-from. It is the one definition
// both `tests` and
// `verify` call, so they cannot disagree about what a merged tree requires.
func For(merged bool) testprompts.Composition {
	want := testprompts.Standard()
	if merged {
		want[testprompts.TypeEdgeCase] = mergedEdgeMinimum
		want[PreferMerged] = preferMergedMinimum
	}
	return want
}

// Undeclared explains a prefer_merged_over_source case in a skill gated under the
// standard mix. Without it the only report is skillet's "unknown type", which is true
// and points nowhere: the repair is a frontmatter declaration, not a test-prompts edit.
//
// Ensures: nil when merged is true or the file holds no prefer_merged_over_source case.
func Undeclared(f *testprompts.File, merged bool) []string {
	if merged || f.CountOf(PreferMerged) == 0 {
		return nil
	}
	return []string{fmt.Sprintf(
		"%s cases need the merged composition: declare %s with the skills this one "+
			"was merged from, or run with --merge on a tree that is merged throughout",
		PreferMerged, mergedfrom.Path)}
}
