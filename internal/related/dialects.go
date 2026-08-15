package related

import "strings"

// This file is the tolerant read path. related.go defines the one bullet format
// exegesis *writes* (Bullet); the dialects below are the formats found in skill
// trees written before that format settled, which `index` must still be able to
// read. Reading them is not an endorsement: a target given as a markdown link to
// `../slug/SKILL.md` is still reported by `exegesis lint` as a parent-escaping
// link, because "can this edge be resolved" and "is the body in house style" are
// separate questions. The reader takes the slug and ignores the path.
//
// The write path deliberately does not use any of this — see parseBullet.

// sectionBullets returns the logical bullets in the section body lines[head+1:end),
// each carrying the line span it occupies so a rewrite can replace exactly the lines
// it came from. Lines inside a code fence are skipped.
//
// A bullet whose rationale wraps across lines is one logical bullet: the
// continuation lines are folded in, because a reader that stopped at the newline
// would silently truncate the rationale — 9 such lines exist in the real books.
//
// Ensures: the spans are ascending and non-overlapping, and every span begins at a
//
//	bullet line; it is pure.
func sectionBullets(lines []string, head, end int) []logicalBullet {
	var out []logicalBullet
	inFence := false
	for i := head + 1; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if isFence(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence || !isBulletLine(trimmed) {
			continue
		}
		text, next := foldContinuations(lines, i, end)
		out = append(out, logicalBullet{text: text, start: i, end: next})
		i = next - 1 // the loop's own increment moves past the folded lines
	}
	return out
}

// foldContinuations joins the bullet at lines[start] with the continuation lines
// that follow it, returning the joined text and the index one past the last line
// consumed. A blank line, another bullet, a heading, or a fence ends the bullet.
func foldContinuations(lines []string, start, end int) (text string, next int) {
	parts := []string{strings.TrimSpace(lines[start])}
	i := start + 1
	for ; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || isBulletLine(trimmed) || isHeading(trimmed) || isFence(trimmed) {
			break
		}
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, " "), i
}

// isBulletLine reports whether a trimmed line opens a list item. A thematic break
// ("---") starts with a dash but is not a bullet.
func isBulletLine(trimmed string) bool {
	if strings.HasPrefix(trimmed, "---") {
		return false
	}
	_, ok := stripMarker(trimmed)
	return ok
}

// readBullet parses one `## Related skills` bullet in any dialect, returning one
// edge per target it names. The dialects, all of which appear in real trees:
//
//   - depends-on: `slug` — why                        (canonical)
//   - depends-on: `a`, `b`                            (multi-target)
//   - **composes-with** → [`slug`](../slug/SKILL.md): why
//   - depends-on: [slug](../slug/SKILL.md) — why
//   - **slug** (composes-with): why                   (reversed)
//   - depends-on: slug (why)                          (bare token)
//   - **superseded-by**: merged/run/slug — why        (bold kind, colon, qualified)
//
// A target may be tree-qualified in any of these — see isTarget.
//
// ok is false for a line that is not a bullet, whose kind is not one of the known
// kinds, or that names no resolvable target — a bullet whose "target" is
// prose, such as "- contrasts-with: (traditional headcount-scaling model)", names
// no skill and so yields no edge.
//
// Requires: line is a single line.
// Ensures:  every returned edge has Kind.Valid() and a non-empty Target; ok is
//
//	true iff at least one edge is returned; it is pure.
func readBullet(line string) ([]Edge, bool) {
	rest, ok := stripMarker(strings.TrimSpace(line))
	if !ok {
		return nil, false
	}
	kind, tail, ok := splitKind(rest)
	if !ok {
		return nil, false
	}
	targets, rationale := takeTargets(tail)
	if len(targets) == 0 {
		return nil, false
	}
	edges := make([]Edge, 0, len(targets))
	for _, t := range targets {
		edges = append(edges, Edge{Kind: kind, Target: t, Rationale: rationale})
	}
	return edges, true
}

// stripMarker removes the list marker from a trimmed line. Both "-" and "*" appear
// as bullet markers in the wild.
func stripMarker(trimmed string) (string, bool) {
	for _, marker := range []string{"- ", "* "} {
		if rest, found := strings.CutPrefix(trimmed, marker); found {
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

// splitKind takes the edge kind off the head of rest, in any of the three
// orientations: "kind:", "**kind**" (optionally followed by an arrow), and the
// reversed "**slug** (kind):". For the reversed form the slug is left at the head
// of the returned tail, so the caller extracts targets the same way in every case.
//
// Ensures: ok is false unless the kind found is Valid.
func splitKind(rest string) (Kind, string, bool) {
	if tail, kind, ok := splitReversed(rest); ok {
		return kind, tail, true
	}
	if tail, kind, ok := splitReversedDash(rest); ok {
		return kind, tail, true
	}
	name, tail, ok := splitKindForward(rest)
	if !ok {
		return "", "", false
	}
	kind, ok := canonicalKind(name)
	if !ok {
		return "", "", false
	}
	return kind, strings.TrimSpace(tail), true
}

// canonicalKind resolves a dialect's spelling of a kind to the canonical one. A
// canonical name maps to itself, so this is the single gate every orientation passes
// through and Valid stays the definition of what may be written.
//
// The synonyms are the ones a survey of the 233-skill corpus actually found, with the
// rationale of every instance read: "combines" always says used-together and "compares"
// always says alternatives-by-different-means, so both are unambiguous.
//
// "prerequisite for" is deliberately absent, and must stay absent. It is the *inverse*
// of depends-on -- proved by the corpus rather than inferred, since 13 of its 18 edges
// have their exact flip already present as a depends-on bullet in the other skill. A
// reader cannot absorb it: the flipped edge belongs in the target's file, and
// ParseSection only ever speaks for the file it is reading. Mapping it here without
// flipping would reverse real edges; mapping it *with* a flip would attribute an edge to
// a skill whose text never declared it. Moving these is a rewrite, not a read.
func canonicalKind(name string) (Kind, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "combines":
		return ComposesWith, true
	case "compares":
		return ContrastsWith, true
	case "depends on":
		return DependsOn, true
	}
	k := Kind(strings.TrimSpace(name))
	return k, k.Valid()
}

// splitReversedDash matches the "**slug** — kind: why" orientation, the one the books
// write most and the reader understood least: measured against the corpus, not one of
// its 110 bullets parsed before this, including the "depends on" spellings an earlier
// note had called unambiguous.
//
// It returns the slug at the head of the tail, exactly as splitReversed does, so the
// caller extracts targets the same way in every orientation.
func splitReversedDash(rest string) (tail string, kind Kind, ok bool) {
	bold, isBold := boldToken(rest)
	if !isBold || !isSlug(bold) {
		return "", "", false
	}
	after := strings.TrimSpace(afterBold(rest))
	for _, dash := range []string{"—", "–", "--"} {
		if trimmed, found := strings.CutPrefix(after, dash); found {
			after = strings.TrimSpace(trimmed)
			break
		}
	}
	name, why, found := strings.Cut(after, ":")
	if !found {
		return "", "", false
	}
	k, valid := canonicalKind(name)
	if !valid {
		return "", "", false
	}
	// The em dash is load-bearing, not decoration: takeTargets keeps taking targets while
	// the head parses as one, and a rationale beginning with bare words ("shapes how it is
	// applied") yields four more targets without a separator it stops at.
	return bold + " — " + strings.TrimSpace(why), k, true
}

// splitKindForward takes the kind name off the two kind-first orientations,
// "**kind** ..." and "kind: ...", leaving the separator handling to afterBoldKind for
// the bold form.
func splitKindForward(rest string) (name, tail string, ok bool) {
	if bold, isBold := boldToken(rest); isBold {
		return bold, afterBoldKind(rest), true
	}
	return strings.Cut(rest, ":")
}

// afterBoldKind returns what follows a bold kind, without the separator the dialect
// puts between the kind and its target.
//
// Both separators are real: "→" is what the linked-target family writes, and ":" is
// what all 26 `superseded-by` bullets in the books write. Only "→" was stripped, so
// `- **depends-on**: some-skill` — bold kind, colon, bare target — parsed as no edge
// at all, for every kind. Measured with a probe against the pre-change reader before
// this was widened, rather than assumed from the shape of the code.
func afterBoldKind(rest string) string {
	tail := strings.TrimSpace(afterBold(rest))
	for _, separator := range []string{"→", ":"} {
		if trimmed, found := strings.CutPrefix(tail, separator); found {
			return strings.TrimSpace(trimmed)
		}
	}
	return tail
}

// splitReversed matches the "**slug** (kind): why" orientation. It returns the slug
// followed by what came after the kind, so the caller extracts the target and the
// rationale exactly as it does for every other orientation.
func splitReversed(rest string) (tail string, kind Kind, ok bool) {
	bold, isBold := boldToken(rest)
	if !isBold || !isSlug(bold) {
		return "", "", false
	}
	after := strings.TrimSpace(afterBold(rest))
	inner, closed := strings.CutPrefix(after, "(")
	if !closed {
		return "", "", false
	}
	name, why, found := strings.Cut(inner, ")")
	if !found {
		return "", "", false
	}
	k := Kind(strings.TrimSpace(name))
	if !k.Valid() {
		return "", "", false
	}
	// This form has the same latent defect splitReversedDash guards against: with no
	// separator takeTargets stops at, a rationale of bare words becomes extra targets --
	// "**alpha** (composes-with) use them together" yields three edges, inventing "use"
	// and "them". Deliberately NOT fixed here: the one-word fix changes what Normalize
	// rewrites and fails TestNormalize, so it needs its own before/after over the corpus
	// rather than riding along with the dialect work. Recorded in TODO.md.
	return bold + " " + why, k, true
}

// boldToken returns the contents of a leading "**...**" span.
func boldToken(s string) (string, bool) {
	inner, ok := strings.CutPrefix(s, "**")
	if !ok {
		return "", false
	}
	token, _, found := strings.Cut(inner, "**")
	if !found {
		return "", false
	}
	return strings.TrimSpace(token), true
}

// afterBold returns what follows a leading "**...**" span.
func afterBold(s string) string {
	inner, ok := strings.CutPrefix(s, "**")
	if !ok {
		return s
	}
	_, after, found := strings.Cut(inner, "**")
	if !found {
		return s
	}
	return after
}

// takeTargets pulls the run of skill references off the head of tail and returns
// them with whatever remains as the rationale.
//
// Extraction is anchored to the head and stops at the first item that is not a
// reference: a backticked span or markdown link later in the line belongs to the
// rationale. Scanning the whole line instead would turn prose like "flags
// (`--force`, `--yes`)" into edges to skills named `--force` and `--yes`, which
// the verify graph gate would then report as dangling.
func takeTargets(tail string) (targets []string, rationale string) {
	rest := tail
	for {
		next := strings.TrimLeft(rest, " \t,")
		target, after, ok := takeOneTarget(next)
		if !ok {
			break
		}
		targets = append(targets, target)
		rest = after
	}
	return targets, trimRationale(rest)
}

// takeOneTarget reads a single reference — `slug`, [`slug`](path), [slug](path),
// or a bare slug — from the head of s, returning the remainder.
func takeOneTarget(s string) (target, after string, ok bool) {
	if inner, found := strings.CutPrefix(s, "["); found {
		label, tail, closed := strings.Cut(inner, "]")
		if !closed {
			return "", "", false
		}
		if slug, isRef := bareOrQuoted(label); isRef {
			return slug, skipLinkPath(tail), true
		}
		return "", "", false
	}
	if inner, found := strings.CutPrefix(s, "`"); found {
		slug, tail, closed := strings.Cut(inner, "`")
		if !closed || !isTarget(slug) {
			return "", "", false
		}
		return slug, tail, true
	}
	// A bare token counts only when it is unmistakably a slug, so that prose such
	// as "(traditional headcount-scaling model)" yields no target.
	token, tail := cutToken(s)
	if !isTarget(token) {
		return "", "", false
	}
	return token, tail, true
}

// bareOrQuoted returns the slug in a link label, written either `slug` or slug.
func bareOrQuoted(label string) (string, bool) {
	trimmed := strings.TrimSpace(label)
	if inner, found := strings.CutPrefix(trimmed, "`"); found {
		slug, _, closed := strings.Cut(inner, "`")
		return slug, closed && isTarget(slug)
	}
	return trimmed, isTarget(trimmed)
}

// skipLinkPath drops a "(path)" immediately following a link label.
func skipLinkPath(s string) string {
	inner, found := strings.CutPrefix(s, "(")
	if !found {
		return s
	}
	_, after, closed := strings.Cut(inner, ")")
	if !closed {
		return s
	}
	return after
}

// cutToken splits off the leading whitespace-delimited token.
func cutToken(s string) (token, rest string) {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i], s[i:]
	}
	return s, ""
}

// trimRationale strips the punctuation a dialect may use to introduce the
// rationale.
func trimRationale(s string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(s), ":—-→,"))
}

// isTarget reports whether s is a skill reference: a slug, or a tree-qualified path
// of slugs such as "merged/all-books-v1/some-skill".
//
// Every segment must be a strict slug, so the tolerance costs almost nothing on the
// prose-rejection side: "(traditional headcount-scaling model)" still names no skill.
// The residual case is a bare two-word phrase joined by a slash — "and/or" reads as a
// target — which is the same residual the bare-slug rule already carries and which no
// bullet in the corpus exhibits. Narrowing it would mean refusing bare qualified
// tokens, and all 26 real cross-tree bullets are written exactly that way.
//
// Ensures: isTarget(s) implies every "/"-separated segment satisfies isSlug; it is pure.
func isTarget(s string) bool {
	for _, segment := range strings.Split(s, "/") {
		if !isSlug(segment) {
			return false
		}
	}
	return true
}

// isSlug reports whether s is a strict Agent Skills slug: one or more
// lowercase-alphanumeric runs joined by single hyphens. It is deliberately strict
// because a bare token is only treated as a skill reference when it cannot be
// mistaken for prose.
func isSlug(s string) bool {
	if s == "" {
		return false
	}
	prevHyphen := true // a leading hyphen is not allowed
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			prevHyphen = false
		case r == '-':
			if prevHyphen {
				return false // leading or doubled hyphen
			}
			prevHyphen = true
		default:
			return false
		}
	}
	return !prevHyphen // a trailing hyphen is not allowed
}
