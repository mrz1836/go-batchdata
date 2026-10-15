package batchdata_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/call"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// TestClientSenderMakesOneCall drives a Client's sender the way a service
// does, through call.FuncOf: one POST to the API root plus the call's path,
// with the call's body, then the envelope's result and warnings to Decode, or
// a rejection without calling it.
func TestClientSenderMakesOneCall(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		decoded  bool
		result   string
		warnings []batchdata.Warning
		want     error
	}{
		{
			name: "success", status: http.StatusOK,
			body:    `{"status":{"code":200},"result":{"data":[]},"warnings":[{"code":"warning_deprecated_field","message":"m"}]}`,
			decoded: true, result: `{"data":[]}`,
			warnings: []batchdata.Warning{{Code: batchdata.WarningDeprecatedField, Message: "m"}},
		},
		{name: "rejection", status: http.StatusTooManyRequests, body: `{"status":{"code":429}}`, want: batchdata.ErrRateLimited},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := testutil.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			})
			c, err := batchdata.NewClient(testKey, batchdata.WithHTTPClient(srv.Client()), batchdata.WithTestEndpoint(srv.URL()))
			require.NoError(t, err)
			send := call.FuncOf[batchdata.Warning](c)
			require.NotNil(t, send)

			decoded := false
			var result string
			var warnings []batchdata.Warning
			err = send(t.Context(), call.Call[batchdata.Warning]{
				Path: "/test/path",
				Body: []byte(`{"q":1}`),
				Decode: func(r []byte, w []batchdata.Warning) error {
					decoded, result, warnings = true, string(r), w
					return nil
				},
			})
			if tt.want != nil {
				require.ErrorIs(t, err, tt.want)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.decoded, decoded)
			if tt.decoded {
				assert.JSONEq(t, tt.result, result)
			}
			assert.Equal(t, tt.warnings, warnings)

			reqs := srv.Requests()
			require.Len(t, reqs, 1)
			assert.Equal(t, http.MethodPost, reqs[0].Method)
			assert.Equal(t, "/api/v3/test/path", reqs[0].URL)
			assert.JSONEq(t, `{"q":1}`, string(reqs[0].Body))
		})
	}
}
