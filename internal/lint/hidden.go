package lint

import (
	"fmt"
	"strings"
)

// The three classes this file gates on. Each is a range from the Unicode standard
// rather than a tuned threshold, which is what makes them eligible to block: there
// is no corpus to calibrate against and no false-positive rate to trade off.
//
// Deliberately absent is the mixed-script confusable test. It is a heuristic —
// multilingual prose legitimately puts Latin and Cyrillic in one word — so it would
// need exactly the calibration these three do not, and it is excluded rather than
// demoted to a warning because a lint tier that mixes exact and approximate rules
// teaches readers to distrust both.
const (
	classZeroWidth hiddenClass = "zero-width character"
	classBidi      hiddenClass = "bidirectional override"
	classTag       hiddenClass = "Unicode tag character"
)

// hiddenClass names a class of codepoint that is invisible when a SKILL.md is read
// but present when an agent obeys it.
type hiddenClass string

// hiddenHit is one invisible codepoint, located well enough to remove by hand.
//
// Char is kept as a rune rather than rendered at construction so the caller
// chooses how to display something that has no visible form.
type hiddenHit struct {
	Line  int
	Class hiddenClass
	Char  rune
}

// scanHidden reports invisible codepoints in raw, one hit per line per class.
//
// It scans the whole document, fences included: the threat is what an agent reads,
// and a code fence does not stop it reading. A skill that legitimately documents one
// of these characters is the expected false positive, and the answer is the escape
// sequence named in the message rather than an exemption — writing `​` states
// the same thing and survives a copy-paste, which a literal cannot.
//
// Requires: nothing; raw may be empty or invalid UTF-8.
// Ensures:  one hit per (line, class) pair, in line order, class order within a
//
//	line; Line is 1-based; an empty result means raw holds no codepoint from
//	any class.
func scanHidden(raw string) []hiddenHit {
	if raw == "" {
		return nil
	}
	var hits []hiddenHit
	for i, line := range strings.Split(raw, "\n") {
		seen := map[hiddenClass]bool{}
		for _, r := range line {
			c, ok := classify(r)
			if !ok || seen[c] {
				continue
			}
			seen[c] = true
			hits = append(hits, hiddenHit{Line: i + 1, Class: c, Char: r})
		}
	}
	return hits
}

// classify names the class r belongs to, or reports false for a visible rune.
func classify(r rune) (hiddenClass, bool) {
	switch {
	case r == 0x200B, r == 0x200C, r == 0x200D, r == 0x2060, r == 0xFEFF:
		return classZeroWidth, true
	// LRE/RLE/PDF/LRO/RLO, then the isolates LRI/RLI/FSI/PDI: the Trojan Source
	// class, which reorders how a line displays without changing what it says.
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
		return classBidi, true
	case r >= 0xE0001 && r <= 0xE007F:
		return classTag, true
	}
	return "", false
}

// message states the class, the codepoint, and the remedy.
//
// The codepoint is spelled U+XXXX because the character itself is invisible by
// definition: a reader cannot search for what they cannot see, and the escape in the
// remedy is also the fix.
func (h hiddenHit) message() string {
	return fmt.Sprintf(
		"hidden %s at SKILL.md:%d (U+%04X) — remove it, or write it as \\u%04X if the skill means to document it",
		h.Class,
		h.Line,
		h.Char,
		h.Char,
	)
}
