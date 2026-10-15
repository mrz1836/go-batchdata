package batchdata

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/mrz1836/go-batchdata/internal/call"
	"github.com/mrz1836/go-batchdata/internal/httpx"
	"github.com/mrz1836/go-batchdata/internal/jsonx"
)

// apiRoot is the path of the V3 API on the origin. A service's call path is
// relative to it.
const apiRoot = "/api/v3"

// Bounds on the shape of a 200 body, checked before decoding. Decoding
// multiplies small JSON values into large Go values: "{}," is 3 bytes, and a
// record or a person decoded from it is close to 200, so the byte limit alone
// would not bound memory. A full answer to 100 items holds a few thousand
// objects and arrays and a few tens of thousands of values, an order of
// magnitude below these limits.
const (
	// maxContainers bounds the objects and arrays in a 200 body.
	maxContainers = 100_000

	// maxValues bounds the values in a 200 body: objects, arrays, strings,
	// numbers and literals, keys excluded.
	maxValues = 1_000_000
)

// newSender returns the Sender of a Client. Everything a call needs, the
// Authorization value and the origin included, is captured here and nowhere
// else.
func newSender(hc *http.Client, cfg httpx.Config) call.Sender[Warning] {
	return func(ctx context.Context, c call.Call[Warning]) error {
		resp, f := httpx.Do(ctx, hc, cfg, apiRoot+c.Path, c.Body)
		if f != nil {
			return fromFailure(f, time.Now())
		}
		return decodeEnvelope(resp, c.Decode, time.Now())
	}
}

// fromFailure maps a transport failure onto an *Error. A status other than 2xx
// is a rejection; a 2xx status other than 200 is a malformed response.
func fromFailure(f *httpx.Failure, now time.Time) *Error {
	var e *Error
	switch f.Kind {
	case httpx.FailureCanceled:
		e = newError(KindCanceled, context.Canceled, "", nil)
	case httpx.FailureDeadlineExceeded:
		e = newError(KindDeadlineExceeded, context.DeadlineExceeded, "", nil)
	case httpx.FailureTimeout:
		e = newError(KindTimeout, context.DeadlineExceeded, "", nil)
	case httpx.FailureTooLarge:
		e = newError(KindResponseTooLarge, nil, "", nil)
	case httpx.FailureEncoding:
		e = newError(KindMalformedResponse, nil, "", nil)
	case httpx.FailureStatus:
		if f.Status >= http.StatusOK && f.Status < http.StatusMultipleChoices {
			e = newError(KindMalformedResponse, nil, "", nil)
			break
		}
		st, top := refusal(f.BodyExcerpt)
		e = rejected(st, top, f.Header, now)
	case httpx.FailureTransport:
		e = newError(KindTransport, nil, "", nil)
	default:
		e = newError(KindTransport, nil, "", nil)
	}
	e.StatusCode = f.Status
	return e
}

// wireEnvelope is the top level of every response body.
type wireEnvelope struct {
	Status   json.RawMessage `json:"status"`
	Result   json.RawMessage `json:"result"`
	Warnings json.RawMessage `json:"warnings"`
}

// wireStatus is the envelope's status object.
type wireStatus struct {
	Code     *int      `json:"code"`
	Message  string    `json:"message"`
	Warnings []Warning `json:"warnings"`
}

// decodeEnvelope checks a 200 body strictly and hands its result object and
// warnings to decode. The body must be valid UTF-8 and exactly one JSON object;
// status, when present and not null, must be an object; a status code other
// than 200 is a rejection; result must be an object; warnings, when present
// and not null, must be an array of warnings. Any other shape, and any error
// from decode, is a malformed response.
func decodeEnvelope(resp httpx.Response, decode func([]byte, []Warning) error, now time.Time) error {
	malformed := func() error {
		e := newError(KindMalformedResponse, nil, "", nil)
		e.StatusCode = http.StatusOK
		return e
	}
	if !utf8.Valid(resp.Body) || !json.Valid(resp.Body) || !jsonx.IsObject(resp.Body) {
		return malformed()
	}
	if containers, values := jsonx.Count(resp.Body); containers > maxContainers || values > maxValues {
		return malformed()
	}
	var env wireEnvelope
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return malformed()
	}
	var st wireStatus
	if jsonx.Present(env.Status) {
		if !jsonx.IsObject(env.Status) || json.Unmarshal(env.Status, &st) != nil {
			return malformed()
		}
	}
	if st.Code != nil && *st.Code != http.StatusOK {
		top, _ := warnings(env.Warnings)
		e := rejected(st, top, resp.Header, now)
		e.StatusCode = http.StatusOK
		return e
	}
	top, ok := warnings(env.Warnings)
	if !ok || !jsonx.IsObject(env.Result) {
		return malformed()
	}
	if decode == nil || decode(env.Result, slices.Concat(st.Warnings, top)) != nil {
		return malformed()
	}
	return nil
}

// refusal reads a rejection body best-effort. Anything it cannot decode is
// dropped: the HTTP status alone classifies the rejection.
func refusal(body []byte) (wireStatus, []Warning) {
	var st wireStatus
	if !utf8.Valid(body) || !json.Valid(body) || !jsonx.IsObject(body) {
		return st, nil
	}
	var env wireEnvelope
	if json.Unmarshal(body, &env) != nil {
		return st, nil
	}
	if jsonx.IsObject(env.Status) && json.Unmarshal(env.Status, &st) != nil {
		st = wireStatus{}
	}
	top, _ := warnings(env.Warnings)
	return st, top
}

// rejected builds a KindRejected *Error with the provider's code, message and
// warnings, and the Retry-After hint of header.
func rejected(st wireStatus, top []Warning, header http.Header, now time.Time) *Error {
	e := newError(KindRejected, nil, st.Message, slices.Concat(st.Warnings, top))
	if st.Code != nil {
		e.ProviderCode = *st.Code
	}
	e.RetryAfter, e.HasRetryAfter = httpx.RetryAfter(header, now)
	return e
}

// warnings decodes the envelope's top-level warnings. Absent or null is no
// warnings; any other value must be an array of warning objects.
func warnings(raw json.RawMessage) ([]Warning, bool) {
	if !jsonx.Present(raw) {
		return nil, true
	}
	var w []Warning
	if !jsonx.IsArray(raw) || json.Unmarshal(raw, &w) != nil {
		return nil, false
	}
	return w, true
}
