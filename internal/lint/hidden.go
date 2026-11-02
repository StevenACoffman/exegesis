package lint

import (
	"fmt"

	"github.com/StevenACoffman/skillet/finding"
	"github.com/StevenACoffman/skillet/hidden"
)

// hiddenFindings reports the invisible codepoints in one file of a skill, one diagnostic
// per line per class so a payload of many characters does not bury every other finding.
//
// The classification and its context rules are skillet/hidden's: a zero-width joiner is
// reported only next to ASCII or another listed character (so emoji and Persian or
// Arabic text pass), the England, Scotland and Wales flags are exempt, and a byte-order
// mark is allowed at offset 0. Those are exact rules from the Unicode standard rather
// than tuned thresholds, which is what lets this block.
//
// It scans the whole file, fences included: the threat is what an agent reads, and a code
// fence does not stop it reading. A skill that legitimately documents one of these
// characters is the expected false positive, and the remedy is the escape sequence the
// message names rather than an exemption.
//
// Requires: nothing; text may be empty or invalid UTF-8.
// Ensures:  one error per (line, class), in line order; it is pure.
func hiddenFindings(name, text string) []finding.Diagnostic {
	type key struct {
		line  int
		class hidden.Class
	}
	seen := map[key]bool{}
	var ds []finding.Diagnostic
	for _, h := range hidden.Scan(text) {
		k := key{h.Line, h.Class}
		if seen[k] {
			continue
		}
		seen[k] = true
		ds = append(ds, diag(fmt.Sprintf(
			"hidden %s at %s:%d (%s) — remove it, or write it as \\u%04X if the skill means to document it",
			className(h.Class),
			name,
			h.Line,
			hidden.Codepoint(h.Rune),
			h.Rune,
		)))
	}
	return ds
}

// className is the reader-facing name of a hidden class, kept from exegesis's earlier
// messages except where the class widened: bidirectional marks are now included, so the
// class is "controls" rather than "overrides".
func className(c hidden.Class) string {
	switch c {
	case hidden.ClassZeroWidth:
		return "zero-width character"
	case hidden.ClassBidi:
		return "bidirectional control"
	case hidden.ClassTag:
		return "Unicode tag character"
	}
	return string(c)
}
