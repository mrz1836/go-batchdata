package batchdata

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/httpx"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// repeated returns n copies of item joined by commas.
func repeated(item string, n int) string {
	return strings.TrimSuffix(strings.Repeat(item+",", n), ",")
}

func TestDecodeEnvelopeShapeBudget(t *testing.T) {
	decode := func([]byte, []Warning) error { return nil }
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		// The envelope and result objects and the data array are 3 containers.
		{"containers at the limit", `{"result":{"data":[` + repeated(`{}`, maxContainers-3) + `]}}`, true},
		{"containers over the limit", `{"result":{"data":[` + repeated(`{}`, maxContainers-2) + `]}}`, false},
		{"values at the limit", `{"result":{"data":[` + repeated(`0`, maxValues-3) + `]}}`, true},
		{"values over the limit", `{"result":{"data":[` + repeated(`0`, maxValues-2) + `]}}`, false},
		{"warnings count", `{"status":{"code":429,"warnings":[` + repeated(`{}`, maxContainers) + `]}}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := decodeEnvelope(httpx.Response{Body: []byte(tt.body)}, decode, testNow())
			if tt.ok {
				require.NoError(t, err)
				return
			}
			e := testutil.RequireErrorAs[*Error](t, err)
			assert.Equal(t, KindMalformedResponse, e.Kind, "an oversized shape is malformed, even inside a rejection")
			assert.Equal(t, http.StatusOK, e.StatusCode)
		})
	}
}
