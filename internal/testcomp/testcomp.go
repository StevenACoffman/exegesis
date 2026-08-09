// Package testcomp defines exegesis's test-prompts compositions: the standard book2skill
// mix and the merged-skill mix. Both `tests` and `verify` read them, so a merged tree is
// gated under the same rules whichever command runs — closing the drift that let `verify`
// reject `prefer_merged_over_source` (validating against the standard mix) while
// `tests --merge` accepted it. It is the composition profile in its smallest honest form;
// it earns promotion to skillet only on a consumer outside exegesis.
package testcomp

import "github.com/StevenACoffman/skillet/testprompts"

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

// For returns the case mix a tree is gated against: the merged-skill mix when merged is
// true, otherwise skillet's standard one. It is the one definition both `tests` and
// `verify` call, so they cannot disagree about what a merged tree requires.
func For(merged bool) testprompts.Composition {
	want := testprompts.Standard()
	if merged {
		want[testprompts.TypeEdgeCase] = mergedEdgeMinimum
		want[PreferMerged] = preferMergedMinimum
	}
	return want
}
