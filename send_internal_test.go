package batchdata

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/httpx"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// errSentinelDecode is a decode failure whose text must never surface.
var errSentinelDecode = errors.New("SENTINEL-DECODE")

// testNow is the fixed clock of these tests.
func testNow() time.Time { return time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC) }

// sentinels are every public sentinel, for checking that an error matches
// exactly the expected ones.
func sentinels() []error {
	return []error{
		ErrInvalidConfig, ErrInvalidRequest, ErrRejected, ErrUnauthorized, ErrForbidden,
		ErrRateLimited, ErrMalformedResponse, context.Canceled, context.DeadlineExceeded,
	}
}

// assertMatches checks that err matches every sentinel in want and no other.
func assertMatches(t *testing.T, err error, want ...error) {
	t.Helper()
	for _, s := range sentinels() {
		assert.Equal(t, slices.Contains(want, s), errors.Is(err, s), "errors.Is(%v, %v)", err, s)
	}
}

func TestFromFailureKinds(t *testing.T) {
	tests := []struct {
		name    string
		f       httpx.Failure
		kind    Kind
		status  int
		matches []error
		text    string
	}{
		{"canceled", httpx.Failure{Kind: httpx.FailureCanceled}, KindCanceled, 0, []error{context.Canceled}, "batchdata: canceled"},
		{"deadline", httpx.Failure{Kind: httpx.FailureDeadlineExceeded}, KindDeadlineExceeded, 0, []error{context.DeadlineExceeded}, "batchdata: deadline_exceeded"},
		{"timeout", httpx.Failure{Kind: httpx.FailureTimeout}, KindTimeout, 0, []error{context.DeadlineExceeded}, "batchdata: timeout"},
		{"transport", httpx.Failure{Kind: httpx.FailureTransport}, KindTransport, 0, nil, "batchdata: transport"},
		{"unknown kind", httpx.Failure{Kind: 0}, KindTransport, 0, nil, "batchdata: transport"},
		{"canceled during 200 read", httpx.Failure{Kind: httpx.FailureCanceled, Status: 200}, KindCanceled, 200, []error{context.Canceled}, "batchdata: canceled (HTTP 200)"},
		{"timeout during 200 read", httpx.Failure{Kind: httpx.FailureTimeout, Status: 200}, KindTimeout, 200, []error{context.DeadlineExceeded}, "batchdata: timeout (HTTP 200)"},
		{"transport during 200 read", httpx.Failure{Kind: httpx.FailureTransport, Status: 200}, KindTransport, 200, nil, "batchdata: transport (HTTP 200)"},
		{"too large", httpx.Failure{Kind: httpx.FailureTooLarge, Status: 200}, KindResponseTooLarge, 200, []error{ErrMalformedResponse}, "batchdata: response_too_large (HTTP 200)"},
		{"encoding", httpx.Failure{Kind: httpx.FailureEncoding, Status: 200}, KindMalformedResponse, 200, []error{ErrMalformedResponse}, "batchdata: malformed_response (HTTP 200)"},
		{"201", httpx.Failure{Kind: httpx.FailureStatus, Status: 201}, KindMalformedResponse, 201, []error{ErrMalformedResponse}, "batchdata: malformed_response (HTTP 201)"},
		{"204", httpx.Failure{Kind: httpx.FailureStatus, Status: 204}, KindMalformedResponse, 204, []error{ErrMalformedResponse}, ""},
		{"206", httpx.Failure{Kind: httpx.FailureStatus, Status: 206}, KindMalformedResponse, 206, []error{ErrMalformedResponse}, ""},
		{"101", httpx.Failure{Kind: httpx.FailureStatus, Status: 101}, KindRejected, 101, []error{ErrRejected}, ""},
		{"301", httpx.Failure{Kind: httpx.FailureStatus, Status: 301}, KindRejected, 301, []error{ErrRejected}, "batchdata: rejected (HTTP 301)"},
		{"304", httpx.Failure{Kind: httpx.FailureStatus, Status: 304}, KindRejected, 304, []error{ErrRejected}, ""},
		{"308", httpx.Failure{Kind: httpx.FailureStatus, Status: 308}, KindRejected, 308, []error{ErrRejected}, ""},
		{"400", httpx.Failure{Kind: httpx.FailureStatus, Status: 400}, KindRejected, 400, []error{ErrRejected}, ""},
		{"401", httpx.Failure{Kind: httpx.FailureStatus, Status: 401}, KindRejected, 401, []error{ErrRejected, ErrUnauthorized}, "batchdata: rejected (HTTP 401)"},
		{"403", httpx.Failure{Kind: httpx.FailureStatus, Status: 403}, KindRejected, 403, []error{ErrRejected, ErrForbidden}, ""},
		{"404", httpx.Failure{Kind: httpx.FailureStatus, Status: 404}, KindRejected, 404, []error{ErrRejected}, ""},
		{"405", httpx.Failure{Kind: httpx.FailureStatus, Status: 405}, KindRejected, 405, []error{ErrRejected}, ""},
		{"410", httpx.Failure{Kind: httpx.FailureStatus, Status: 410}, KindRejected, 410, []error{ErrRejected}, ""},
		{"418", httpx.Failure{Kind: httpx.FailureStatus, Status: 418}, KindRejected, 418, []error{ErrRejected}, ""},
		{"422", httpx.Failure{Kind: httpx.FailureStatus, Status: 422}, KindRejected, 422, []error{ErrRejected}, ""},
		{"429", httpx.Failure{Kind: httpx.FailureStatus, Status: 429}, KindRejected, 429, []error{ErrRejected, ErrRateLimited}, "batchdata: rejected (HTTP 429)"},
		{"500", httpx.Failure{Kind: httpx.FailureStatus, Status: 500}, KindRejected, 500, []error{ErrRejected}, ""},
		{"502", httpx.Failure{Kind: httpx.FailureStatus, Status: 502}, KindRejected, 502, []error{ErrRejected}, ""},
		{"503", httpx.Failure{Kind: httpx.FailureStatus, Status: 503}, KindRejected, 503, []error{ErrRejected}, ""},
		{"504", httpx.Failure{Kind: httpx.FailureStatus, Status: 504}, KindRejected, 504, []error{ErrRejected}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := fromFailure(&tt.f, testNow())
			require.NotNil(t, e)
			assert.Equal(t, tt.kind, e.Kind)
			assert.Equal(t, tt.status, e.StatusCode)
			assert.Zero(t, e.ProviderCode)
			assert.False(t, e.HasRetryAfter)
			assertMatches(t, e, tt.matches...)
			if tt.text != "" {
				assert.Equal(t, tt.text, e.Error())
			}
		})
	}
}

func TestFromFailureRejectionBody(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		header   http.Header
		code     int
		message  string
		warnings []Warning
		matches  []error
		retry    time.Duration
		hasRetry bool
	}{
		{
			name: "documented error body", status: 429,
			body:    `{"status":{"code":429,"text":"Too Many Requests","message":"SENTINEL-MSG limit"}}`,
			header:  http.Header{"Retry-After": {"30"}},
			code:    429,
			message: "SENTINEL-MSG limit", matches: []error{ErrRejected, ErrRateLimited},
			retry: 30 * time.Second, hasRetry: true,
		},
		{
			name: "http 429 with body 200 matches by http status", status: 429,
			body:    `{"status":{"code":200,"text":"OK"}}`,
			code:    200,
			matches: []error{ErrRejected, ErrRateLimited},
		},
		{
			name: "http 400 with body 401 matches by http status", status: 400,
			body:    `{"status":{"code":401}}`,
			code:    401,
			matches: []error{ErrRejected},
		},
		{
			name: "warnings in status and top level", status: 400,
			body: `{"status":{"code":400,"message":"m","warnings":[{"code":"dataset_not_allowed","message":"a"}]},` +
				`"warnings":[{"code":"warning_new_code","message":"b"}]}`,
			code: 400, message: "m",
			warnings: []Warning{{Code: WarningDatasetNotAllowed, Message: "a"}, {Code: "warning_new_code", Message: "b"}},
			matches:  []error{ErrRejected},
		},
		{name: "not json", status: 502, body: `<html>SENTINEL-GATEWAY</html>`, matches: []error{ErrRejected}},
		{name: "empty", status: 503, body: ``, matches: []error{ErrRejected}},
		{name: "json array", status: 500, body: `[1,2]`, matches: []error{ErrRejected}},
		{name: "status wrong type", status: 500, body: `{"status":"SENTINEL"}`, matches: []error{ErrRejected}},
		{name: "code wrong type", status: 500, body: `{"status":{"code":"500","message":"x"}}`, matches: []error{ErrRejected}},
		{name: "invalid utf8", status: 500, body: "{\"status\":{\"message\":\"\xff\"}}", matches: []error{ErrRejected}},
		{
			name: "bad top-level warnings dropped", status: 403,
			body: `{"status":{"code":403,"message":"m"},"warnings":"SENTINEL"}`,
			code: 403, message: "m",
			matches: []error{ErrRejected, ErrForbidden},
		},
		{
			name: "retry-after date", status: 503, body: `{}`,
			header:  http.Header{"Retry-After": {testNow().Add(90 * time.Second).Format(http.TimeFormat)}},
			matches: []error{ErrRejected}, retry: 90 * time.Second, hasRetry: true,
		},
		{
			name: "retry-after garbage", status: 503, body: `{}`,
			header:  http.Header{"Retry-After": {"soon"}},
			matches: []error{ErrRejected},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var excerpt []byte
			if tt.body != "" {
				excerpt = []byte(tt.body)
			}
			e := fromFailure(&httpx.Failure{Kind: httpx.FailureStatus, Status: tt.status, Header: tt.header, BodyExcerpt: excerpt}, testNow())
			assert.Equal(t, KindRejected, e.Kind)
			assert.Equal(t, tt.status, e.StatusCode)
			assert.Equal(t, tt.code, e.ProviderCode)
			assert.Equal(t, tt.message, e.ProviderMessage())
			assert.Equal(t, tt.warnings, e.Warnings())
			assert.Equal(t, tt.retry, e.RetryAfter)
			assert.Equal(t, tt.hasRetry, e.HasRetryAfter)
			assertMatches(t, e, tt.matches...)
			assert.NotContains(t, e.Error(), "SENTINEL")
			assert.NotContains(t, e.GoString(), "SENTINEL")
		})
	}
}

func TestDecodeEnvelopeSuccess(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		result   string
		warnings []Warning
	}{
		{"documented", `{"status":{"code":200,"text":"OK","message":"Success"},"result":{"data":[]}}`, `{"data":[]}`, nil},
		{"no status", `{"result":{}}`, `{}`, nil},
		{"null status", `{"status":null,"result":{}}`, `{}`, nil},
		{"status without code", `{"status":{"text":"OK"},"result":{}}`, `{}`, nil},
		{"null warnings", `{"result":{},"warnings":null}`, `{}`, nil},
		{"empty warnings", `{"result":{},"warnings":[]}`, `{}`, nil},
		{"surrounding whitespace", " \n{\"result\":{}}\t\r\n", `{}`, nil},
		{"unknown fields", `{"extra":[1],"result":{"x":1},"status":{"code":200,"extra":true}}`, `{"x":1}`, nil},
		{
			"warnings merged in order",
			`{"status":{"code":200,"warnings":[{"code":"warning_deprecated_field","message":"a"}]},` +
				`"result":{},"warnings":[{"code":"warning_rate_limit_approaching","message":"b"},{"code":"x"}]}`,
			`{}`,
			[]Warning{{Code: WarningDeprecatedField, Message: "a"}, {Code: WarningRateLimitApproaching, Message: "b"}, {Code: "x"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotResult string
			var gotWarnings []Warning
			calls := 0
			err := decodeEnvelope(httpx.Response{Body: []byte(tt.body)}, func(result []byte, w []Warning) error {
				calls++
				gotResult, gotWarnings = string(result), w
				return nil
			}, testNow())
			require.NoError(t, err)
			assert.Equal(t, 1, calls)
			assert.JSONEq(t, tt.result, gotResult)
			assert.Equal(t, tt.warnings, gotWarnings)
		})
	}
}

func TestDecodeEnvelopeMalformed(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty", ``},
		{"whitespace only", " \n\t"},
		{"invalid json", `{"result":`},
		{"invalid utf8", "{\"result\":{},\"x\":\"\xff\"}"},
		{"byte order mark", "\ufeff{\"result\":{}}"},
		{"trailing data", `{"result":{}} {}`},
		{"trailing garbage", `{"result":{}}x`},
		{"top-level null", `null`},
		{"top-level array", `[]`},
		{"top-level number", `0`},
		{"top-level string", `"SENTINEL"`},
		{"result missing", `{"status":{"code":200}}`},
		{"result null", `{"result":null}`},
		{"result array", `{"result":[]}`},
		{"result string", `{"result":"x"}`},
		{"status string", `{"status":"OK","result":{}}`},
		{"status array", `{"status":[],"result":{}}`},
		{"status number", `{"status":200,"result":{}}`},
		{"status code string", `{"status":{"code":"200"},"result":{}}`},
		{"status code fraction", `{"status":{"code":200.5},"result":{}}`},
		{"status code overflow", `{"status":{"code":1e400},"result":{}}`},
		{"status message number", `{"status":{"code":200,"message":5},"result":{}}`},
		{"status warnings object", `{"status":{"code":200,"warnings":{}},"result":{}}`},
		{"warnings object", `{"result":{},"warnings":{}}`},
		{"warnings string", `{"result":{},"warnings":"SENTINEL"}`},
		{"warnings of strings", `{"result":{},"warnings":["SENTINEL"]}`},
		{"warning code number", `{"result":{},"warnings":[{"code":1}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := decodeEnvelope(httpx.Response{Body: []byte(tt.body)}, func([]byte, []Warning) error {
				calls++
				return nil
			}, testNow())
			e := testutil.RequireErrorAs[*Error](t, err)
			assert.Equal(t, KindMalformedResponse, e.Kind)
			assert.Equal(t, http.StatusOK, e.StatusCode)
			assert.Zero(t, e.ProviderCode)
			assertMatches(t, err, ErrMalformedResponse)
			assert.Zero(t, calls, "decode must not run")
			assert.Equal(t, "batchdata: malformed_response (HTTP 200)", e.Error())
		})
	}
}

func TestDecodeEnvelopeDecodeFailure(t *testing.T) {
	err := decodeEnvelope(httpx.Response{Body: []byte(`{"result":{}}`)}, func([]byte, []Warning) error {
		return errSentinelDecode
	}, testNow())
	e := testutil.RequireErrorAs[*Error](t, err)
	assert.Equal(t, KindMalformedResponse, e.Kind)
	require.NoError(t, errors.Unwrap(err))
	assert.NotContains(t, e.Error(), "SENTINEL")

	err = decodeEnvelope(httpx.Response{Body: []byte(`{"result":{}}`)}, nil, testNow())
	assert.Equal(t, KindMalformedResponse, testutil.RequireErrorAs[*Error](t, err).Kind)
}

func TestDecodeEnvelopeEmbeddedRejection(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		header   http.Header
		code     int
		message  string
		warnings []Warning
		matches  []error
		text     string
		hasRetry bool
	}{
		{
			name: "401", body: `{"status":{"code":401,"text":"Unauthorized","message":"SENTINEL-TOKEN bad"}}`,
			code: 401, message: "SENTINEL-TOKEN bad", matches: []error{ErrRejected, ErrUnauthorized},
			text: "batchdata: rejected (HTTP 200, provider status 401)",
		},
		{
			name: "429 with retry-after", body: `{"status":{"code":429},"result":{"data":[]}}`,
			header: http.Header{"Retry-After": {"7"}},
			code:   429, matches: []error{ErrRejected, ErrRateLimited}, hasRetry: true,
		},
		{name: "403", body: `{"status":{"code":403}}`, code: 403, matches: []error{ErrRejected, ErrForbidden}},
		{name: "500", body: `{"status":{"code":500}}`, code: 500, matches: []error{ErrRejected}},
		{name: "201", body: `{"status":{"code":201},"result":{}}`, code: 201, matches: []error{ErrRejected}},
		{
			name: "zero code", body: `{"status":{"code":0},"result":{}}`,
			code: 0, matches: []error{ErrRejected}, text: "batchdata: rejected (HTTP 200)",
		},
		{
			name: "warnings kept, bad top-level warnings dropped",
			body: `{"status":{"code":400,"warnings":[{"code":"dataset_not_allowed","message":"SENTINEL"}]},"warnings":5}`,
			code: 400, warnings: []Warning{{Code: WarningDatasetNotAllowed, Message: "SENTINEL"}}, matches: []error{ErrRejected},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := decodeEnvelope(httpx.Response{Header: tt.header, Body: []byte(tt.body)}, func([]byte, []Warning) error {
				calls++
				return nil
			}, testNow())
			e := testutil.RequireErrorAs[*Error](t, err)
			assert.Zero(t, calls)
			assert.Equal(t, KindRejected, e.Kind)
			assert.Equal(t, http.StatusOK, e.StatusCode)
			assert.Equal(t, tt.code, e.ProviderCode)
			assert.Equal(t, tt.message, e.ProviderMessage())
			assert.Equal(t, tt.warnings, e.Warnings())
			assert.Equal(t, tt.hasRetry, e.HasRetryAfter)
			assertMatches(t, err, tt.matches...)
			if tt.text != "" {
				assert.Equal(t, tt.text, e.Error())
			}
			assert.NotContains(t, e.Error(), "SENTINEL")
		})
	}
}

func TestErrorNilSafety(t *testing.T) {
	var e *Error
	assert.Equal(t, "batchdata: <nil>", e.Error())
	assert.Equal(t, "(*batchdata.Error)(nil)", e.GoString())
	require.NoError(t, e.Unwrap())
	assert.False(t, e.Is(ErrRejected))
	assert.Empty(t, e.ProviderMessage())
	assert.Nil(t, e.Warnings())

	zero := &Error{}
	assert.Equal(t, "batchdata: ", zero.Error())
	require.NoError(t, zero.Unwrap())
	assert.Empty(t, zero.ProviderMessage())
	assert.Nil(t, zero.Warnings())

	var ve *ValidationError
	assert.Equal(t, "batchdata: <nil>", ve.Error())
	require.NoError(t, ve.Unwrap())
}

func TestErrorWarningsAreCopies(t *testing.T) {
	src := []Warning{{Code: WarningDeprecatedField, Message: "a"}}
	e := newError(KindRejected, nil, "", src)
	src[0].Message = "changed by caller"
	got := e.Warnings()
	require.Len(t, got, 1)
	assert.Equal(t, "a", got[0].Message)
	got[0].Message = "changed again"
	assert.Equal(t, "a", e.Warnings()[0].Message)
}

func TestErrorGoString(t *testing.T) {
	e := newError(KindRejected, nil, "SENTINEL-MSG", []Warning{{Message: "SENTINEL-WARN"}})
	e.StatusCode, e.ProviderCode, e.RetryAfter, e.HasRetryAfter = 429, 429, 2*time.Second, true
	assert.Equal(t,
		`&batchdata.Error{Kind:"rejected", StatusCode:429, ProviderCode:429, RetryAfter:2000000000, HasRetryAfter:true}`,
		e.GoString())
}

func TestValidationErrorText(t *testing.T) {
	tests := []struct {
		name string
		err  *ValidationError
		want string
	}{
		{"full", &ValidationError{Err: ErrInvalidRequest, Field: "Items[0]", Reason: "is bad"}, "batchdata: invalid request: Items[0]: is bad"},
		{"no field", &ValidationError{Err: ErrInvalidConfig, Reason: "is bad"}, "batchdata: invalid configuration: is bad"},
		{"no sentinel", &ValidationError{Field: "f"}, "batchdata: validation failed: f"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
			assert.Equal(t, tt.err.Err, tt.err.Unwrap())
		})
	}
}

func TestErrorTextAndUnknownKind(t *testing.T) {
	assert.Equal(t, "batchdata: rejected (provider status 429)", (&Error{Kind: KindRejected, ProviderCode: 429}).Error())
	unknown := &Error{Kind: "SENTINEL-KIND", StatusCode: 401}
	for _, s := range sentinels() {
		assert.NotErrorIs(t, unknown, s)
	}
}
