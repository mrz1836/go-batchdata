// Package call is the sealed bridge between the root batchdata package and the
// service packages, such as skiptrace.
//
// The root Client embeds a Handle under an unexported alias, which promotes
// Handle's unexported sender method into the Client's method set. A service
// passes its *batchdata.Client to FuncOf, whose parameter is a Holder: an
// interface with that unexported method, so only a type embedding a Handle
// satisfies it, and the compiler rejects anything else. Code outside this
// module cannot import this package, so it can neither build a Handle nor
// reach the Sender.
//
// The root package owns every public error. A Sender returns only errors the
// root built, and a Call's Decode reports a failure with any error, which the
// root replaces with its own.
package call

import "context"

// Call is one provider call.
type Call[W any] struct {
	// Path is the endpoint path relative to the API root, such as
	// "/property/skip-trace". It is a constant of the service package.
	Path string

	// Body is the encoded JSON request, sent verbatim.
	Body []byte

	// Decode receives the raw "result" object of a successful response and
	// the envelope's warnings. A non-nil error marks the response malformed.
	Decode func(result []byte, warnings []W) error
}

// Sender makes one call. It makes at most one HTTP attempt.
type Sender[W any] func(ctx context.Context, c Call[W]) error

// Handle holds a Sender. It has no exported fields or methods.
type Handle[W any] struct {
	send Sender[W]
}

// NewHandle returns a Handle holding s.
func NewHandle[W any](s Sender[W]) Handle[W] {
	return Handle[W]{send: s}
}

// sender returns the held Sender, which is nil for the zero Handle.
func (h Handle[W]) sender() Sender[W] { return h.send }

// Holder is satisfied only by types that embed a Handle.
type Holder[W any] interface {
	sender() Sender[W]
}

// FuncOf returns the Sender held by h, or nil when h is nil or holds none. A
// typed nil pointer panics: callers check for one before calling FuncOf.
func FuncOf[W any](h Holder[W]) Sender[W] {
	if h == nil {
		return nil
	}
	return h.sender()
}
