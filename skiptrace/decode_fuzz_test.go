package skiptrace_test

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/testutil"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

// FuzzDecodeMatchesTrace checks that any body decodes without panicking and
// without being modified, that decoding it twice gives the same outcome, that
// a decoded Result is well formed and an error has a call's shape, and that a
// call receiving the same body returns the same outcome.
func FuzzDecodeMatchesTrace(f *testing.F) {
	for _, b := range storedBodies() {
		f.Add([]byte(b.body), "row-1", "row-2")
	}
	f.Add([]byte(envelope(matched("a"))), "a", "A ") // two IDs equal after folding
	f.Add([]byte(envelope(matched("x"))), "", "")
	f.Fuzz(func(t *testing.T, body []byte, id0, id1 string) {
		address := skiptrace.Address{Street: "100 Example Ave", Zip: "00000"}
		req := skiptrace.Request{Items: []skiptrace.Item{
			{RequestID: id0, PropertyAddress: address},
			{RequestID: id1, PropertyAddress: address},
		}}

		kept := bytes.Clone(body)
		res, err := skiptrace.Decode(body, req)
		require.Equal(t, kept, body, "Decode must not modify the body")

		again, againErr := skiptrace.Decode(kept, req)
		requireSameOutcome(t, res, err, again, againErr)

		if err == nil {
			assertWellFormedResult(t, res, req)
		} else {
			assertCallShapedError(t, err)
		}

		if req.Validate() == nil && len(body) <= batchdata.DefaultMaxResponseBytes {
			tracer, _ := memoryTracer(t, reply(http.StatusOK, string(body)))
			traced, traceErr := tracer.Trace(t.Context(), req)
			requireSameOutcome(t, traced, traceErr, res, err)
		}
	})
}

// assertWellFormedResult checks that every record of res has a known outcome
// and association, that only a record linked by request ID points at an
// item, and at one whose ID it echoes, and that res survives a JSON round
// trip.
func assertWellFormedResult(t *testing.T, res *skiptrace.Result, req skiptrace.Request) {
	t.Helper()
	require.NotNil(t, res)
	outcomes := []skiptrace.Outcome{
		skiptrace.OutcomeFailed, skiptrace.OutcomeMatched, skiptrace.OutcomeNoMatch,
		skiptrace.OutcomeUnreported, skiptrace.OutcomeUndecodable,
	}
	for _, r := range res.Records {
		assert.Contains(t, outcomes, r.Outcome)
		switch r.Association {
		case skiptrace.AssociationRequestID:
			require.NotNil(t, r.ItemIndex)
			require.Less(t, *r.ItemIndex, len(req.Items))
			assert.Equal(t, req.Items[*r.ItemIndex].RequestID, r.Input.RequestID)
		case skiptrace.AssociationSingleItem:
			require.Fail(t, "a two-item request is never linked by elimination")
		case skiptrace.AssociationAmbiguous, skiptrace.AssociationNone:
			assert.Nil(t, r.ItemIndex)
		default:
			require.Failf(t, "unknown association", "%q", r.Association)
		}
	}
	testutil.RequireJSONRoundTrip(t, *res)
}

// assertCallShapedError checks that err is what a call returns for an HTTP 200
// body it cannot use: a *batchdata.Error with status 200, a kind a body can
// cause, and no cause.
func assertCallShapedError(t *testing.T, err error) {
	t.Helper()
	e := testutil.RequireErrorAs[*batchdata.Error](t, err)
	assert.Equal(t, http.StatusOK, e.StatusCode)
	assert.Contains(t,
		[]batchdata.Kind{batchdata.KindRejected, batchdata.KindMalformedResponse, batchdata.KindResponseTooLarge},
		e.Kind)
	require.NoError(t, errors.Unwrap(err))
}
