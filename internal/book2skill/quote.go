package book2skill

import (
	"strconv"
	"unicode"
	"unicode/utf8"
)

// Source-quote length caps, measured in Unicode code points (runes), not bytes.
// The cap depends on the dominant script of the book text: CJK writing packs far
// more meaning per rune than Latin script, so its cap is tighter.
const (
	// QuoteMaxRunesCJK caps source quotes drawn from CJK-dominant book text.
	QuoteMaxRunesCJK = 150
	// QuoteMaxRunesLatin caps source quotes drawn from Latin or other text.
	QuoteMaxRunesLatin = 650
	// cjkShareThreshold is the fraction of letter runes that must be CJK for a
	// text to be treated as CJK-dominant when selecting the quote cap.
	cjkShareThreshold = 0.20
)

// QuoteMaxRunes returns the source-quote rune cap appropriate for bookText,
// selecting the CJK cap when the text is CJK-dominant and the Latin cap
// otherwise.
func QuoteMaxRunes(bookText string) int {
	if IsCJKDominant(bookText) {
		return QuoteMaxRunesCJK
	}
	return QuoteMaxRunesLatin
}

// IsCJKDominant reports whether CJK runes make up at least cjkShareThreshold of
// the letters in text. Non-letter runes are ignored so that punctuation and
// whitespace do not skew the ratio.
func IsCJKDominant(text string) bool {
	var cjk, letters int
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if isCJK(r) {
			cjk++
		}
	}
	if letters == 0 {
		return false
	}
	return float64(cjk)/float64(letters) >= cjkShareThreshold
}

// ValidateQuote returns an EINVALID error when quote exceeds maxRunes code
// points, and nil otherwise.
func ValidateQuote(quote string, maxRunes int) error {
	if n := utf8.RuneCountInString(quote); n > maxRunes {
		return &Error{
			Code: EINVALID,
			Message: "source quote is " + strconv.Itoa(n) +
				" runes; limit is " + strconv.Itoa(maxRunes),
		}
	}
	return nil
}

// isCJK reports whether r belongs to a Han, Hiragana, Katakana, or Hangul
// script — the ranges that justify the tighter CJK quote cap.
func isCJK(r rune) bool {
	switch {
	case unicode.Is(unicode.Han, r),
		unicode.Is(unicode.Hiragana, r),
		unicode.Is(unicode.Katakana, r),
		unicode.Is(unicode.Hangul, r):
		return true
	default:
		return false
	}
}
