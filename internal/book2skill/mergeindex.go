package book2skill

// MergeIndex is the deterministic model behind a cross-book merged-skills index.
// It is assembled from the merge-status ledgers and the source/merged trees, and
// rendered by render.MergeIndex. The judgment-only sections of the merge INDEX
// template (source-verification summary, free-text notes) are not modeled here.
type MergeIndex struct {
	RunSlug      string
	Sources      []MergeSourceBook
	Merges       []MergeRecord
	Verification []VerificationRow // per-pair Phase-1.5 verdicts (may be empty)
}

// MergeSourceBook is one source book scanned by a merge run.
type MergeSourceBook struct {
	Slug       string
	Title      string
	Author     string
	Skills     []string        // skill slugs in this book, sorted
	Superseded map[string]bool // skill slug -> superseded by a merged skill this run
	Edges      []Relationship  // intra-book relationships (both ends in this book)
}

// MergeRecord is one merged skill and the source skills that fed it.
type MergeRecord struct {
	Slug    string
	Title   string
	Parents []MergeParent
	Edges   []Relationship // the merged skill's own ## Related Skills edges
}

// MergeParent is one source skill merged into a merged skill.
type MergeParent struct {
	BookSlug  string
	SkillSlug string
	State     MergeState
}
