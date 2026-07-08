package book2skill

// Merge-status states, written to a source skill's `## Merge Status` ledger by a
// merge-skills run at the phase where the skill's fate is decided.
const (
	// StateNoCandidate: no overlap candidate found for this skill in this run.
	StateNoCandidate MergeState = "no-candidate"
	// StateSurfaceResemblance: a pair was evaluated and rejected as labels-only.
	StateSurfaceResemblance MergeState = "surface-resemblance"
	// StateComplementary: different domains; a Zettelkasten link was added instead.
	StateComplementary MergeState = "complementary"
	// StateRejected: passed Phase 0 but failed source verification or V1–V4.
	StateRejected MergeState = "rejected"
	// StatePartial: merged, but some of this skill's content was excluded.
	StatePartial MergeState = "partial"
	// StateMerged: merged, with all key content represented.
	StateMerged MergeState = "merged"
)

// Merge-status rejection reason codes, valid only with StateRejected.
const (
	// ReasonSourceTextUnavailable: the source EPUB/PDF was not available.
	ReasonSourceTextUnavailable MergeReason = "source-text-unavailable"
	// ReasonSourceVerificationFailed: a quote was not found, or material drift.
	ReasonSourceVerificationFailed MergeReason = "source-verification-failed"
	// ReasonV1Failed: convergence was not genuine.
	ReasonV1Failed MergeReason = "v1-failed"
	// ReasonV2Failed: the merged skill added no predictive power.
	ReasonV2Failed MergeReason = "v2-failed"
	// ReasonV3Failed: the synthesis was common wisdom.
	ReasonV3Failed MergeReason = "v3-failed"
	// ReasonV4FailedMergeNotAdditive: the merge added no capability over sources.
	ReasonV4FailedMergeNotAdditive MergeReason = "v4-failed-merge-not-additive"
)

// MergeState classifies a source skill's fate in one merge run.
type MergeState string

// MergeReason is the machine-readable cause recorded with a rejected state.
type MergeReason string

// MergeStatusEntry is one append-only line in a source skill's `## Merge Status`
// ledger. Its YAML form is the fenced block merge-skills maintains; the struct
// tags are inert here (marshaling lives in the internal/mergedoc adapter).
type MergeStatusEntry struct {
	Run      string      `yaml:"run"`
	State    MergeState  `yaml:"state"`
	Pair     string      `yaml:"pair,omitempty"`
	Into     string      `yaml:"into,omitempty"`
	Reason   MergeReason `yaml:"reason,omitempty"`
	Excluded string      `yaml:"excluded,omitempty"`
}

// Valid reports whether s is a known merge state.
func (s MergeState) Valid() bool {
	switch s {
	case StateNoCandidate, StateSurfaceResemblance, StateComplementary,
		StateRejected, StatePartial, StateMerged:
		return true
	default:
		return false
	}
}

// Valid reports whether r is a known rejection reason.
func (r MergeReason) Valid() bool {
	switch r {
	case ReasonSourceTextUnavailable, ReasonSourceVerificationFailed,
		ReasonV1Failed, ReasonV2Failed, ReasonV3Failed, ReasonV4FailedMergeNotAdditive:
		return true
	default:
		return false
	}
}

// Validate returns the reasons e is not a well-formed ledger entry; an empty
// slice means it is valid. It enforces the vocabulary and the per-state
// required-field rules from the merge-skills contract.
func (e *MergeStatusEntry) Validate() []string {
	var problems []string
	if e.Run == "" {
		problems = append(problems, "run is required")
	}
	if !e.State.Valid() {
		problems = append(problems, "unknown state "+string(e.State))
		return problems // remaining field rules depend on a known state
	}
	if requiresPair(e.State) && e.Pair == "" {
		problems = append(problems, "state "+string(e.State)+" requires pair")
	}
	if (e.State == StateMerged || e.State == StatePartial) && e.Into == "" {
		problems = append(problems, "state "+string(e.State)+" requires into")
	}
	if e.State == StatePartial && e.Excluded == "" {
		problems = append(problems, "state partial requires excluded")
	}
	problems = append(problems, validateReason(e)...)
	return problems
}

func requiresPair(s MergeState) bool {
	return s == StateSurfaceResemblance || s == StateComplementary || s == StateRejected
}

// validateReason enforces that reason is present and valid exactly for rejected.
func validateReason(e *MergeStatusEntry) []string {
	switch {
	case e.State == StateRejected && e.Reason == "":
		return []string{"state rejected requires reason"}
	case e.State == StateRejected && !e.Reason.Valid():
		return []string{"unknown reason " + string(e.Reason)}
	case e.State != StateRejected && e.Reason != "":
		return []string{"reason is only valid with state rejected"}
	default:
		return nil
	}
}
