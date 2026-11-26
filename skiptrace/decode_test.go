package skiptrace_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/testutil"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

// storedBody is a named response body, as a caller would have kept it.
type storedBody struct {
	name string
	body string
}

// storedRequest is a named request, as a call would have sent it.
type storedRequest struct {
	name string
	req  skiptrace.Request
}

// storedBodies returns the bodies the stored-response tests and the fuzz
// target share: every record fixture, the result's options and meta, both
// warning lists, rejections inside an HTTP 200 body, and malformed bodies.
func storedBodies() []storedBody {
	return []storedBody{
		{name: "full record", body: envelope(fixtureFullRecord)},
		{name: "no-match record", body: envelope(fixtureNoMatchRecord)},
		{name: "error record", body: envelope(fixtureErrorRecord)},
		{name: "unreported record", body: envelope(fixtureUnreportedRecord)},
		{name: "nulls record", body: envelope(fixtureNullsRecord)},
		{name: "absent record", body: envelope(fixtureAbsentRecord)},
		{name: "unknown enums record", body: envelope(fixtureUnknownEnumsRecord)},
		{name: "over maxima record", body: envelope(fixtureOverMaximaRecord)},
		{name: "unknown fields record", body: envelope(fixtureUnknownFieldsRecord)},
		{name: "every record fixture", body: envelope(
			fixtureFullRecord, fixtureNoMatchRecord, fixtureErrorRecord, fixtureUnreportedRecord, fixtureNullsRecord,
			fixtureAbsentRecord, fixtureUnknownEnumsRecord, fixtureOverMaximaRecord, fixtureUnknownFieldsRecord,
		)},
		{name: "options, meta and unknown warnings", body: `{"status":{"code":200,"text":"OK"},"result":{"data":[` +
			fixtureFullRecord + `],"options":` + fixtureResultOptions + `,"meta":` + fixtureResultMeta +
			`},"warnings":` + fixtureUnknownWarnings + `}`},
		{name: "partial meta", body: `{"status":{"code":200,"text":"OK"},"result":{"data":[` +
			fixtureFullRecord + `],"options":` + fixtureResultOptions + `,"meta":` + fixtureResultMetaPartial + `}}`},
		{name: "matched and no match", body: envelope(matched("row-1"), noMatch("row-2"))},
		{name: "both warning lists and an undecodable record", body: `{"status":{"code":200,"text":"OK",` +
			`"warnings":[{"code":"warning_deprecated_field","message":"a"}]},"result":{"data":[` +
			noMatch("row-2") + `,` + matched("row-1") + `,null],` +
			`"meta":{"requestId":"01SYNTHETICCALLID000000000","results":{"requestCount":2,"matchCount":1}}},` +
			`"warnings":[{"code":"warning_rate_limit_approaching","message":"b"}]}`},
		{name: "embedded 401", body: `{"status":{"code":401,"text":"Unauthorized","message":"Synthetic message"}}`},
		{name: "embedded 429", body: `{"status":{"code":429},"result":{"data":[]}}`},
		{name: "embedded 403 with warnings", body: `{"status":{"code":403,` +
			`"warnings":[{"code":"dataset_not_allowed","message":"Synthetic warning"}]}}`},
		{name: "empty", body: ``},
		{name: "not json", body: `not json`},
		{name: "trailing data", body: `{"result":{"data":[]}} {}`},
		{name: "invalid utf8", body: "{\"result\":{\"data\":[]},\"x\":\"\xff\"}"},
		{name: "null data", body: `{"result":{"data":null}}`},
		{name: "status string", body: `{"status":"OK","result":{"data":[]}}`},
		{name: "top-level null", body: `null`},
		{name: "meta of the wrong type", body: `{"result":{"data":[],"meta":[]}}`},
	}
}

// storedRequests returns the requests a stored body is decoded with: two
// items with IDs, one item without an ID, one item whose ID the full record
// echoes, and three items whose IDs no fixture echoes.
func storedRequests() []storedRequest {
	return []storedRequest{
		{name: "two items", req: manyRequest()},
		{name: "one item without an id", req: oneRequest()},
		{name: "one item with the full record's id", req: skiptrace.Request{Items: []skiptrace.Item{fixtureFullItem()}}},
		{name: "three items", req: fixtureRequest(3)},
	}
}

// sentinels are the errors a call's error and a decoded one must match alike.
func sentinels() []error {
	return []error{
		batchdata.ErrRejected, batchdata.ErrUnauthorized, batchdata.ErrForbidden, batchdata.ErrRateLimited,
		batchdata.ErrMalformedResponse, batchdata.ErrInvalidConfig, batchdata.ErrInvalidRequest,
		context.Canceled, context.DeadlineExceeded,
	}
}

// requireSameOutcome fails t unless two decodes agree: both succeeded with
// equal results, or both failed with *batchdata.Errors that are alike in every
// field, accessor and sentinel, and returned no result.
func requireSameOutcome(t *testing.T, wantRes *skiptrace.Result, wantErr error, gotRes *skiptrace.Result, gotErr error) {
	t.Helper()
	if wantErr == nil {
		require.NoError(t, gotErr)
		assert.Equal(t, wantRes, gotRes)
		return
	}
	want := testutil.RequireErrorAs[*batchdata.Error](t, wantErr)
	got := testutil.RequireErrorAs[*batchdata.Error](t, gotErr)
	assert.Equal(t, want.Kind, got.Kind)
	assert.Equal(t, want.StatusCode, got.StatusCode)
	assert.Equal(t, want.ProviderCode, got.ProviderCode)
	assert.Equal(t, want.RetryAfter, got.RetryAfter)
	assert.Equal(t, want.HasRetryAfter, got.HasRetryAfter)
	assert.Equal(t, want.ProviderMessage(), got.ProviderMessage())
	assert.Equal(t, want.Warnings(), got.Warnings())
	assert.Equal(t, wantErr.Error(), gotErr.Error())
	assert.Equal(t, errors.Unwrap(wantErr), errors.Unwrap(gotErr))
	for _, s := range sentinels() {
		assert.Equal(t, errors.Is(wantErr, s), errors.Is(gotErr, s), "errors.Is(%v)", s)
	}
	assert.Nil(t, wantRes)
	assert.Nil(t, gotRes)
}

func TestDecodeMatchesTrace(t *testing.T) {
	for _, proto := range testutil.Protocols() {
		t.Run(proto.Name(), func(t *testing.T) {
			for _, b := range storedBodies() {
				t.Run(b.name, func(t *testing.T) {
					for _, r := range storedRequests() {
						t.Run(r.name, func(t *testing.T) {
							srv := testutil.NewServerProto(t, proto, serveOK(b.body))
							tracer, ct := serverTracer(t, srv)
							wantRes, wantErr := tracer.Trace(t.Context(), r.req)
							gotRes, gotErr := skiptrace.Decode([]byte(b.body), r.req)
							requireSameOutcome(t, wantRes, wantErr, gotRes, gotErr)
							assert.Equal(t, 1, srv.Hits())
							assert.Equal(t, 1, ct.Attempts())
						})
					}
				})
			}
		})
	}
}

func TestDecodeReadsOnlyRequestIDs(t *testing.T) {
	body := []byte(envelope(matched("row-1"), noMatch("row-2")))
	idsOnly := skiptrace.Request{Items: []skiptrace.Item{{RequestID: "row-1"}, {RequestID: "row-2"}}}
	require.Error(t, idsOnly.Validate(), "items with only an ID are not a valid request")

	want, err := skiptrace.Decode(body, manyRequest())
	require.NoError(t, err)
	got, err := skiptrace.Decode(body, idsOnly)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	for i, item := range idsOnly.Items {
		recs := got.RecordsFor(i)
		require.Len(t, recs, 1, "item %d", i)
		assert.Equal(t, skiptrace.AssociationRequestID, recs[0].Association, "item %d", i)
		assert.Equal(t, item.RequestID, recs[0].Input.RequestID, "item %d", i)
	}

	none, err := skiptrace.Decode(body, skiptrace.Request{})
	require.NoError(t, err)
	require.Len(t, none.Records, 2)
	for _, rec := range none.Records {
		assert.Equal(t, skiptrace.AssociationNone, rec.Association)
		assert.Nil(t, rec.ItemIndex)
	}
}

func TestDecodeRejectionHasNoRetryAfter(t *testing.T) {
	const body = `{"status":{"code":429},"result":{"data":[]}}`
	srv := testutil.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "60")
		_, _ = io.WriteString(w, body)
	})
	tracer, _ := serverTracer(t, srv)
	_, traceErr := tracer.Trace(t.Context(), manyRequest())
	_, decodeErr := skiptrace.Decode([]byte(body), manyRequest())

	traced := testutil.RequireErrorAs[*batchdata.Error](t, traceErr)
	decoded := testutil.RequireErrorAs[*batchdata.Error](t, decodeErr)
	assert.True(t, traced.HasRetryAfter)
	assert.Equal(t, 60*time.Second, traced.RetryAfter)
	assert.False(t, decoded.HasRetryAfter, "a stored body has no Retry-After header")
	assert.Zero(t, decoded.RetryAfter)
	assert.Equal(t, traced.Kind, decoded.Kind)
	assert.Equal(t, traced.StatusCode, decoded.StatusCode)
	assert.Equal(t, traced.ProviderCode, decoded.ProviderCode)
	assert.Equal(t, traceErr.Error(), decodeErr.Error())
	require.ErrorIs(t, traceErr, batchdata.ErrRateLimited)
	require.ErrorIs(t, decodeErr, batchdata.ErrRateLimited)
}

func TestDecodeLeavesTheBodyAlone(t *testing.T) {
	decodes := []string{"full record", "every record fixture", "options, meta and unknown warnings"}
	for _, b := range storedBodies() {
		if !slices.Contains(decodes, b.name) {
			continue
		}
		t.Run(b.name, func(t *testing.T) {
			original := []byte(b.body)
			kept := bytes.Clone(original)
			res, err := skiptrace.Decode(kept, manyRequest())
			require.NoError(t, err)
			assert.Equal(t, original, kept, "Decode must not modify the body")

			for i := range kept {
				kept[i] = 'x'
			}
			want, err := skiptrace.Decode(original, manyRequest())
			require.NoError(t, err)
			assert.Equal(t, want, res, "the result must share no memory with the body")

			again, err := skiptrace.Decode(original, manyRequest())
			require.NoError(t, err)
			assert.Equal(t, want, again)
		})
	}
}

func TestDecodeKeepsItsSignature(t *testing.T) {
	assert.IsType(t, (func([]byte, skiptrace.Request) (*skiptrace.Result, error))(nil), skiptrace.Decode)
}
