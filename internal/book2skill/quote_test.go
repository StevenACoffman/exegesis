package book2skill_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
)

func TestIsCJKDominant(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		text string
		want bool
	}{
		"empty":            {"", false},
		"pure latin":       {"the quick brown fox jumps over the lazy dog", false},
		"pure han":         {"投资决策必须在能力圈之内进行判断", true},
		"latin with punct": {"Invert, always invert! (1885-1955).", false},
		"mixed mostly han": {"能力圈 circle of 判断 the 决策", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := book2skill.IsCJKDominant(tc.text); got != tc.want {
				t.Errorf("IsCJKDominant(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestQuoteMaxRunes(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		text string
		want int
	}{
		"latin": {"the quick brown fox", book2skill.QuoteMaxRunesLatin},
		"cjk":   {"投资决策必须在能力圈之内", book2skill.QuoteMaxRunesCJK},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := book2skill.QuoteMaxRunes(tc.text); got != tc.want {
				t.Errorf("QuoteMaxRunes(%q) = %d, want %d", tc.text, got, tc.want)
			}
		})
	}
}

func TestValidateQuote(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		quote   string
		max     int
		wantErr bool
	}{
		"under limit":       {"short quote", 650, false},
		"exactly at limit":  {strings.Repeat("x", 150), 150, false},
		"one over limit":    {strings.Repeat("x", 151), 150, true},
		"multibyte counted": {strings.Repeat("投", 151), 150, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := book2skill.ValidateQuote(tc.quote, tc.max)
			if tc.wantErr && err == nil {
				t.Fatalf(
					"ValidateQuote(len=%d, max=%d) = nil, want error",
					len([]rune(tc.quote)),
					tc.max,
				)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf(
					"ValidateQuote(len=%d, max=%d) = %v, want nil",
					len([]rune(tc.quote)),
					tc.max,
					err,
				)
			}
			if tc.wantErr && book2skill.ErrorCode(err) != book2skill.EINVALID {
				t.Errorf("ErrorCode = %q, want %q", book2skill.ErrorCode(err), book2skill.EINVALID)
			}
		})
	}
}
