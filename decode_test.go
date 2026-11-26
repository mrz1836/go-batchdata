package batchdata_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// errDecodeFailed is what a failing decode callback returns.
var errDecodeFailed = errors.New("decode failed")

// decodeRecorder is a decode callback that records each call and its
// arguments.
type decodeRecorder struct {
	calls    int
	result   []byte
	warnings []batchdata.Warning
}

// decode records the call and succeeds.
func (r *decodeRecorder) decode(result []byte, warnings []batchdata.Warning) error {
	r.calls++
	r.result = result
	r.warnings = warnings
	return nil
}

// paddedBody returns an empty result envelope followed by spaces, n bytes in
// all: a valid body of exactly that size.
func paddedBody(n int) []byte {
	body := bytes.Repeat([]byte(" "), n)
	copy(body, `{"result":{"data":[]}}`)
	return body
}

func TestDecodeResponseHandsOverTheResultOnce(t *testing.T) {
	body := []byte(`{"status":{"code":200,"warnings":[{"code":"warning_deprecated_field","message":"a"}]},` +
		`"result":{"data":[]},"warnings":[{"code":"x"}]}`)
	var rec decodeRecorder
	require.NoError(t, batchdata.DecodeResponse(body, rec.decode))
	assert.Equal(t, 1, rec.calls)
	assert.JSONEq(t, `{"data":[]}`, string(rec.result))
	assert.Equal(t, []batchdata.Warning{{Code: batchdata.WarningDeprecatedField, Message: "a"}, {Code: "x"}}, rec.warnings)
}

func TestDecodeResponseReportsAnEmbeddedRejection(t *testing.T) {
	var rec decodeRecorder
	err := batchdata.DecodeResponse([]byte(`{"status":{"code":429,"message":"Synthetic message"}}`), rec.decode)
	e := testutil.RequireErrorAs[*batchdata.Error](t, err)
	assert.Equal(t, batchdata.KindRejected, e.Kind)
	assert.Equal(t, 200, e.StatusCode)
	assert.Equal(t, 429, e.ProviderCode)
	assert.Equal(t, "Synthetic message", e.ProviderMessage())
	assert.False(t, e.HasRetryAfter)
	assert.Zero(t, e.RetryAfter)
	require.ErrorIs(t, err, batchdata.ErrRejected)
	require.ErrorIs(t, err, batchdata.ErrRateLimited)
	assert.Equal(t, "batchdata: rejected (HTTP 200, provider status 429)", err.Error())
	assert.Zero(t, rec.calls, "a rejection never reaches decode")
}

func TestDecodeResponseReportsAMalformedBody(t *testing.T) {
	failing := func([]byte, []batchdata.Warning) error { return errDecodeFailed }
	succeeding := func([]byte, []batchdata.Warning) error { return nil }
	tests := []struct {
		name   string
		body   string
		decode func([]byte, []batchdata.Warning) error
	}{
		{name: "empty", body: ``, decode: succeeding},
		{name: "trailing data", body: `{"result":{}} {}`, decode: succeeding},
		{name: "result missing", body: `{"status":{"code":200}}`, decode: succeeding},
		{name: "decode fails", body: `{"result":{}}`, decode: failing},
		{name: "nil decode", body: `{"result":{}}`, decode: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := batchdata.DecodeResponse([]byte(tt.body), tt.decode)
			e := testutil.RequireErrorAs[*batchdata.Error](t, err)
			assert.Equal(t, batchdata.KindMalformedResponse, e.Kind)
			assert.Equal(t, 200, e.StatusCode)
			require.ErrorIs(t, err, batchdata.ErrMalformedResponse)
			require.NoError(t, errors.Unwrap(err))
			assert.Equal(t, "batchdata: malformed_response (HTTP 200)", err.Error())
		})
	}
}

func TestDecodeResponseRefusesABodyOverTheLimit(t *testing.T) {
	t.Run("one byte over the limit", func(t *testing.T) {
		var rec decodeRecorder
		err := batchdata.DecodeResponse(paddedBody(batchdata.MaxResponseBytesLimit+1), rec.decode)
		e := testutil.RequireErrorAs[*batchdata.Error](t, err)
		assert.Equal(t, batchdata.KindResponseTooLarge, e.Kind)
		assert.Equal(t, 200, e.StatusCode)
		require.ErrorIs(t, err, batchdata.ErrMalformedResponse)
		assert.Zero(t, rec.calls, "an oversized body never reaches decode")
	})
	t.Run("at the limit", func(t *testing.T) {
		var rec decodeRecorder
		require.NoError(t, batchdata.DecodeResponse(paddedBody(batchdata.MaxResponseBytesLimit), rec.decode))
		assert.Equal(t, 1, rec.calls)
	})
}
