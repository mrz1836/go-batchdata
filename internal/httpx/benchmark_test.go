package httpx_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mrz1836/go-batchdata/internal/httpx"
)

// BenchmarkDo measures the local cost of one exchange against an in-memory
// 200 response: building the request, the client's own bookkeeping and
// reading the body.
func BenchmarkDo(b *testing.B) {
	hc, _ := fakeClient(func(r *http.Request) (*http.Response, error) {
		return response(r, http.StatusOK, io.NopCloser(strings.NewReader(okBody)), int64(len(okBody))), nil
	})
	cfg := baseConfig()
	body := []byte(testBody)
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		if _, f := httpx.Do(ctx, hc, cfg, testPath, body); f != nil {
			b.Fatalf("failure kind %d", f.Kind)
		}
	}
}
