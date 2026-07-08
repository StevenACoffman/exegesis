package skilllint

import (
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"
	tiktokenloader "github.com/pkoukk/tiktoken-go-loader"
)

// cachedCounter builds the exact cl100k_base counter exactly once, process-wide.
// tiktoken's BPE loader and encoding cache are process-global and not safe to
// initialize concurrently, so the build must be serialized; the resulting
// encoder is read-only and shared. A one-time, never-reset cache is the intended
// use of a package-level value here.
//
//nolint:gochecknoglobals // one-time read-only encoder; tiktoken init is process-global and must run once
var cachedCounter = sync.OnceValues(buildTokenCounter)

// TokenCounter estimates the token count of a text. It is injected into the
// token-budget checks so the concrete counter is chosen at the composition edge.
type TokenCounter func(string) int

// newTokenCounter returns the shared exact cl100k_base counter (built once), or
// the approximation if the offline encoder could not be built.
func newTokenCounter() TokenCounter {
	counter, _ := cachedCounter()
	return counter
}

// ExactTokenizer reports whether the exact cl100k_base encoder is available;
// when false, token-budget checks use an approximation and callers may warn.
func ExactTokenizer() bool {
	_, exact := cachedCounter()
	return exact
}

func buildTokenCounter() (TokenCounter, bool) {
	tiktoken.SetBpeLoader(tiktokenloader.NewOfflineLoader())
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return approxTokens, false
	}
	return func(text string) int {
		return len(enc.Encode(text, nil, nil))
	}, true
}

// resolveCounter returns the first non-nil counter, defaulting to the
// approximation. The token-budget checks accept an optional counter so existing
// callers and tests keep working unchanged.
func resolveCounter(count []TokenCounter) TokenCounter {
	if len(count) > 0 && count[0] != nil {
		return count[0]
	}
	return approxTokens
}
