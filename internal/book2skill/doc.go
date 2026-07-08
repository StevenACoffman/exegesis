// Package book2skill defines the shared domain language for the book2skill
// pipeline: the value types that flow between stages, the small interfaces the
// stages depend on, and the Error type used throughout.
//
// It contains no I/O and no third-party dependencies. Adapters (the LLM client,
// filesystem store, and skillcheck wrapper) live in sibling internal packages
// and depend on this one; they are never imported here.
package book2skill
