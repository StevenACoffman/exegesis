package book2skill

import "strings"

// AppendRelated returns md with rel added as a bullet in the "## Related Skills"
// section (created at end of document if absent), plus whether md changed. It is
// idempotent: if a bullet with the same kind and target already exists the
// document is returned unchanged. The bullet uses the same shape render emits.
func AppendRelated(md string, rel Relationship) (out string, changed bool) {
	for _, existing := range ParseRelated("", md) {
		if existing.Kind == rel.Kind && existing.To == rel.To {
			return md, false
		}
	}
	bullet := relatedBullet(rel)
	lines := strings.Split(md, "\n")
	start, end, found := sectionRange(lines, RelatedSkillsHeading)
	if !found {
		body := strings.TrimRight(md, "\n")
		return body + "\n\n" + headingPrefix + RelatedSkillsHeading + "\n\n" + bullet + "\n", true
	}
	insertAt := end
	for insertAt > start+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt-- // append after the last non-blank line of the section
	}
	updated := make([]string, 0, len(lines)+1)
	updated = append(updated, lines[:insertAt]...)
	updated = append(updated, bullet)
	updated = append(updated, lines[insertAt:]...)
	return strings.Join(updated, "\n"), true
}

// relatedBullet renders one "## Related Skills" bullet, matching render's shape.
func relatedBullet(rel Relationship) string {
	bullet := "- " + string(rel.Kind) + ": `" + rel.To + "`"
	if rel.Rationale != "" {
		bullet += " — " + rel.Rationale
	}
	return bullet
}
