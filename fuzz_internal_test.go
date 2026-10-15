package batchdata

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/httpx"
	"github.com/mrz1836/go-batchdata/internal/jsonx"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// FuzzDecodeEnvelope checks that any 200 body either reaches decode exactly
// once and succeeds, or fails as a rejection or a malformed response without
// reaching decode, and never panics.
func FuzzDecodeEnvelope(f *testing.F) {
	for _, s := range []string{
		`{"status":{"code":200,"text":"OK"},"result":{"data":[]}}`,
		`{"status":{"code":429,"message":"m","warnings":[{"code":"x"}]}}`,
		`{"result":{},"warnings":[{"code":"dataset_not_allowed","message":"m"}]}`,
		`{"status":"OK","result":{}}`,
		`{"result":null}`,
		`null`,
		`[]`,
		`{"result":{}} {}`,
		"\ufeff{}",
		"{\"status\":{\"message\":\"\xff\"}}",
		``,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		calls := 0
		err := decodeEnvelope(httpx.Response{Body: body}, func(result []byte, _ []Warning) error {
			calls++
			assert.True(t, jsonx.IsObject(result))
			return nil
		}, testNow())
		if err == nil {
			assert.Equal(t, 1, calls)
			return
		}
		assert.Zero(t, calls)
		e := testutil.RequireErrorAs[*Error](t, err)
		assert.Equal(t, http.StatusOK, e.StatusCode)
		assert.Contains(t, []Kind{KindRejected, KindMalformedResponse}, e.Kind)
		assert.NoError(t, errors.Unwrap(err))
	})
}

// FuzzRefusal checks that any rejection body, however broken, yields a
// rejection that keeps the HTTP status and never panics.
func FuzzRefusal(f *testing.F) {
	for _, s := range []string{
		`{"status":{"code":401,"message":"m"},"warnings":[{"code":"x","message":"y"}]}`,
		`{"status":{"code":"401"}}`,
		`<html></html>`,
		``,
	} {
		f.Add([]byte(s), 429)
	}
	f.Fuzz(func(t *testing.T, body []byte, status int) {
		if status >= 200 && status < 300 {
			status = 400
		}
		e := fromFailure(&httpx.Failure{Kind: httpx.FailureStatus, Status: status, BodyExcerpt: body}, testNow())
		assert.Equal(t, KindRejected, e.Kind)
		assert.Equal(t, status, e.StatusCode)
		require.ErrorIs(t, e, ErrRejected)
	})
}
