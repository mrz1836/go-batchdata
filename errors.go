package batchdata

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Sentinel errors. Match them with errors.Is.
//
// ErrInvalidConfig and ErrInvalidRequest are local: an error matching either
// means no call was made. Every other sentinel is matched only by an *Error.
var (
	// ErrInvalidConfig reports an invalid NewClient argument or option, or a
	// service built from a nil or zero Client.
	ErrInvalidConfig = errors.New("batchdata: invalid configuration")

	// ErrInvalidRequest reports a nil context or a request that fails local
	// validation, including one whose encoded body exceeds MaxRequestBytes.
	ErrInvalidRequest = errors.New("batchdata: invalid request")

	// ErrRejected reports a call the provider refused: an HTTP status other
	// than 2xx (a redirect included, which is never followed), or an HTTP 200
	// whose body reports a status code other than 200.
	ErrRejected = errors.New("batchdata: rejected by provider")

	// ErrUnauthorized reports a rejection with status 401: a missing, invalid
	// or revoked token. It also matches ErrRejected.
	ErrUnauthorized = errors.New("batchdata: unauthorized")

	// ErrForbidden reports a rejection with status 403. BatchData uses 403 for
	// both insufficient permissions and insufficient balance; the SDK does not
	// guess which. It also matches ErrRejected.
	ErrForbidden = errors.New("batchdata: forbidden")

	// ErrRateLimited reports a rejection with status 429. It also matches
	// ErrRejected.
	ErrRateLimited = errors.New("batchdata: rate limited")

	// ErrMalformedResponse reports a response the SDK cannot use: a 2xx status
	// other than 200, a body over the size limit, or an HTTP 200 body that is
	// not the documented envelope, carries a Content-Encoding nobody decoded,
	// or holds far more JSON objects, arrays or values than any real answer.
	// The call may already have been processed and billed.
	ErrMalformedResponse = errors.New("batchdata: malformed response")
)

// Kind classifies a failed call. The list is closed.
type Kind string

// Failure kinds.
const (
	// KindCanceled means the caller's context was canceled. It matches
	// context.Canceled.
	KindCanceled Kind = "canceled"

	// KindDeadlineExceeded means the caller's context deadline expired. It
	// matches context.DeadlineExceeded.
	KindDeadlineExceeded Kind = "deadline_exceeded"

	// KindTimeout means the call's own timeout expired: WithTimeout, or the
	// Timeout of an injected http.Client. It matches context.DeadlineExceeded.
	KindTimeout Kind = "timeout"

	// KindTransport means the call failed in transit (DNS, TLS, a reset
	// connection or an incomplete body). The provider may or may not have
	// processed it.
	KindTransport Kind = "transport"

	// KindRejected means the provider refused the call. It matches
	// ErrRejected, and ErrUnauthorized, ErrForbidden or ErrRateLimited by
	// status.
	KindRejected Kind = "rejected"

	// KindMalformedResponse means the response could not be used. It matches
	// ErrMalformedResponse.
	KindMalformedResponse Kind = "malformed_response"

	// KindResponseTooLarge means the response body exceeded the size limit.
	// It matches ErrMalformedResponse.
	KindResponseTooLarge Kind = "response_too_large"
)

// ValidationError describes a local validation failure. Err is the matching
// sentinel, ErrInvalidConfig or ErrInvalidRequest. Field names what failed:
// the "apiKey" argument, an option such as "WithTimeout", or a Go field path
// such as "Items[3].PropertyAddress.Street"; it holds indexes, never values.
// Reason never contains caller data.
type ValidationError struct {
	Err    error
	Field  string
	Reason string
}

// Error returns the sentinel message followed by the field and the reason.
func (e *ValidationError) Error() string {
	if e == nil {
		return "batchdata: <nil>"
	}
	var b strings.Builder
	if e.Err != nil {
		b.WriteString(e.Err.Error())
	} else {
		b.WriteString("batchdata: validation failed")
	}
	if e.Field != "" {
		b.WriteString(": ")
		b.WriteString(e.Field)
	}
	if e.Reason != "" {
		b.WriteString(": ")
		b.WriteString(e.Reason)
	}
	return b.String()
}

// Unwrap returns the sentinel.
func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Error describes a call that passed local validation and then failed: the
// attempt failed, or it was never made because the context was already done.
// It is always returned as a *Error.
//
// Its Error string is built from a fixed vocabulary: the kind and the status
// codes. It never contains the API key, URLs, request or response bodies,
// provider messages or network addresses, and neither do GoString, fmt verbs
// that print fields, or the chain Unwrap returns. Provider text is available
// only through ProviderMessage and Warnings.
//
// RetryAfter is the provider's standard Retry-After hint, when it sent one. It
// is a hint, not proof that repeating the call is safe or free. HasRetryAfter
// distinguishes an explicit zero from an absent hint.
type Error struct {
	// Kind classifies the failure.
	Kind Kind

	// StatusCode is the HTTP status, or 0 when no response arrived.
	StatusCode int

	// ProviderCode is the status.code the response body reported, when the
	// body could be decoded; otherwise 0.
	ProviderCode int

	// RetryAfter is the Retry-After hint, clamped to [0, 24h].
	RetryAfter time.Duration

	// HasRetryAfter reports whether the response carried a usable
	// Retry-After header.
	HasRetryAfter bool

	// private holds the cause and the provider's text. fmt prints a func only
	// as an address, with every verb and at every depth, and reflection-based
	// dumpers cannot see what it captures.
	private func() errorDetail
}

// errorDetail is the part of an Error that is never printed.
type errorDetail struct {
	cause    error
	message  string
	warnings []Warning
}

// Error returns "batchdata: <kind>", with the HTTP status and the provider
// status when known, such as "batchdata: rejected (HTTP 429)".
func (e *Error) Error() string {
	if e == nil {
		return "batchdata: <nil>"
	}
	s := "batchdata: " + string(e.Kind)
	switch {
	case e.StatusCode != 0 && e.ProviderCode != 0:
		return s + " (HTTP " + strconv.Itoa(e.StatusCode) + ", provider status " + strconv.Itoa(e.ProviderCode) + ")"
	case e.StatusCode != 0:
		return s + " (HTTP " + strconv.Itoa(e.StatusCode) + ")"
	case e.ProviderCode != 0:
		return s + " (provider status " + strconv.Itoa(e.ProviderCode) + ")"
	default:
		return s
	}
}

// Unwrap returns context.Canceled for KindCanceled and
// context.DeadlineExceeded for KindDeadlineExceeded and KindTimeout, and nil
// otherwise. The underlying transport error is never kept.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.detail().cause
}

// Is reports whether target is a sentinel this error matches: ErrRejected for
// KindRejected, plus ErrUnauthorized, ErrForbidden or ErrRateLimited for status
// 401, 403 or 429, and ErrMalformedResponse for KindMalformedResponse and
// KindResponseTooLarge.
//
// A rejection with an HTTP status other than 200 is matched by that HTTP
// status; an HTTP 200 whose body reports another status is matched by
// ProviderCode.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	switch e.Kind {
	case KindRejected:
		if target == ErrRejected {
			return true
		}
		code := e.StatusCode
		if code == http.StatusOK {
			code = e.ProviderCode
		}
		switch code {
		case http.StatusUnauthorized:
			return target == ErrUnauthorized
		case http.StatusForbidden:
			return target == ErrForbidden
		case http.StatusTooManyRequests:
			return target == ErrRateLimited
		}
		return false
	case KindMalformedResponse, KindResponseTooLarge:
		return target == ErrMalformedResponse
	case KindCanceled, KindDeadlineExceeded, KindTimeout, KindTransport:
		return false
	}
	return false
}

// GoString prints the exported fields only, so %#v cannot reveal the provider's
// text.
func (e *Error) GoString() string {
	if e == nil {
		return "(*batchdata.Error)(nil)"
	}
	return fmt.Sprintf("&batchdata.Error{Kind:%q, StatusCode:%d, ProviderCode:%d, RetryAfter:%d, HasRetryAfter:%t}",
		string(e.Kind), e.StatusCode, e.ProviderCode, int64(e.RetryAfter), e.HasRetryAfter)
}

// ProviderMessage returns the status.message text of the provider's response
// body, or "" when there was none or it could not be decoded. It may echo
// request content, so treat it as sensitive: it is excluded from Error and
// GoString, and logging it is the caller's decision.
func (e *Error) ProviderMessage() string {
	if e == nil {
		return ""
	}
	return e.detail().message
}

// Warnings returns a copy of the warnings in the provider's response body:
// status.warnings, then top-level warnings, in order. Like ProviderMessage,
// they may echo request content.
func (e *Error) Warnings() []Warning {
	if e == nil {
		return nil
	}
	return slices.Clone(e.detail().warnings)
}

func (e *Error) detail() errorDetail {
	if e.private == nil {
		return errorDetail{}
	}
	return e.private()
}

// newError returns an *Error of kind with the given cause, provider message
// and warnings hidden inside its closure.
func newError(kind Kind, cause error, message string, warnings []Warning) *Error {
	d := errorDetail{cause: cause, message: message, warnings: slices.Clone(warnings)}
	return &Error{Kind: kind, private: func() errorDetail { return d }}
}

// Warning is a provider warning: a machine-readable code and a human-readable
// message. Unknown codes are kept as sent.
type Warning struct {
	Code    WarningCode `json:"code"`
	Message string      `json:"message,omitempty"`
}

// WarningCode identifies the type of a Warning. Values other than the
// documented constants are kept as sent.
type WarningCode string

// Documented warning codes.
const (
	// WarningDatasetNotAllowed reports a requested dataset that does not exist
	// or is not available for the token.
	WarningDatasetNotAllowed WarningCode = "dataset_not_allowed"

	// WarningDeprecatedField reports the use of a deprecated field.
	WarningDeprecatedField WarningCode = "warning_deprecated_field"

	// WarningRateLimitApproaching reports that the account is close to its
	// rate limit.
	WarningRateLimitApproaching WarningCode = "warning_rate_limit_approaching"
)

// invalid returns a *ValidationError. The reason must not contain the rejected
// value.
func invalid(sentinel error, field, reason string) error {
	return &ValidationError{Err: sentinel, Field: field, Reason: reason}
}
