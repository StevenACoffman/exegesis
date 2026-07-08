package book2skill

import "errors"

// Error codes. These start with the five the pipeline needs; add more only as
// concrete cases arise rather than speculatively.
const (
	// EINVALID indicates that validation failed (bad input or model output).
	EINVALID = "invalid"
	// ENOTFOUND indicates that a requested entity does not exist.
	ENOTFOUND = "not_found"
	// EINTERNAL indicates an internal error with no more specific code.
	EINTERNAL = "internal"
	// ECONFLICT indicates that the action cannot be performed in this state.
	ECONFLICT = "conflict"
	// EUNAUTHORIZED indicates that the caller lacks permission.
	EUNAUTHORIZED = "unauthorized"
)

// Error is the single error type used across the book2skill packages.
//
// A leaf error carries Code and Message. A wrapping error carries Op (in the
// form "package.Type.Method") and Err. Never set both Code and Err on the same
// value: leaf errors describe what went wrong, wrapping errors describe where.
type Error struct {
	Code    string // machine-readable; set only on leaf errors
	Message string // human-readable; set only on leaf errors
	Op      string // operation that failed; set only on wrapping errors
	Err     error  // nested cause; set only on wrapping errors
}

// Error returns a single-line logical stack trace, wrapping errors prefixing
// their Op onto the nested cause.
func (e *Error) Error() string {
	switch {
	case e.Op != "" && e.Err != nil:
		return e.Op + ": " + e.Err.Error()
	case e.Code != "":
		return e.Code + ": " + e.Message
	default:
		return e.Message
	}
}

// Unwrap exposes the nested cause so errors.Is and errors.As traverse the chain.
func (e *Error) Unwrap() error { return e.Err }

// ErrorCode returns the machine-readable code of the first *Error in err's
// chain that carries one, EINTERNAL if err is non-nil but carries no code, and
// the empty string if err is nil.
func ErrorCode(err error) string {
	var e *Error
	switch {
	case err == nil:
		return ""
	case errors.As(err, &e) && e.Code != "":
		return e.Code
	case errors.As(err, &e) && e.Err != nil:
		return ErrorCode(e.Err)
	default:
		return EINTERNAL
	}
}

// ErrorMessage returns the human-readable message of the first *Error in err's
// chain that carries one, a generic message if err is non-nil but carries none,
// and the empty string if err is nil.
func ErrorMessage(err error) string {
	var e *Error
	switch {
	case err == nil:
		return ""
	case errors.As(err, &e) && e.Message != "":
		return e.Message
	case errors.As(err, &e) && e.Err != nil:
		return ErrorMessage(e.Err)
	default:
		return "an internal error occurred"
	}
}
