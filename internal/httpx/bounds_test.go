package httpx_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/httpx"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

func TestDoResponseSizeLimit(t *testing.T) {
	x := func(n int) string { return strings.Repeat("x", n) }
	tests := []struct {
		name          string
		maxBytes      int64
		body          string
		contentLength int64
		wantKind      httpx.FailureKind // 0: success
		wantBody      string
		wantMaxRead   int64
	}{
		{name: "exactly the limit with length", maxBytes: 16, body: x(16), contentLength: 16, wantBody: x(16)},
		{name: "exactly the limit with unknown length", maxBytes: 16, body: x(16), contentLength: -1, wantBody: x(16)},
		{name: "one over with unknown length", maxBytes: 16, body: x(17), contentLength: -1, wantKind: httpx.FailureTooLarge, wantMaxRead: 17},
		{name: "one over by length is not read", maxBytes: 16, body: x(17), contentLength: 17, wantKind: httpx.FailureTooLarge},
		{name: "understated length over the limit", maxBytes: 16, body: x(40), contentLength: 3, wantKind: httpx.FailureTooLarge, wantMaxRead: 17},
		{name: "understated length within the limit", maxBytes: 16, body: x(10), contentLength: 3, wantBody: x(10)},
		{name: "overstated length within the limit", maxBytes: 16, body: x(5), contentLength: 16, wantBody: x(5)},
		{name: "overstated length over the limit is not read", maxBytes: 16, body: x(5), contentLength: 17, wantKind: httpx.FailureTooLarge},
		{name: "empty body with length", maxBytes: 16, contentLength: 0},
		{name: "empty body with unknown length", maxBytes: 16, contentLength: -1},
		{name: "zero limit admits an empty body", contentLength: -1},
		{name: "zero limit rejects one byte", body: "x", contentLength: -1, wantKind: httpx.FailureTooLarge, wantMaxRead: 1},
		{name: "negative limit rejects one byte", maxBytes: -5, body: "x", contentLength: -1, wantKind: httpx.FailureTooLarge, wantMaxRead: 1},
		{name: "largest limit does not overflow", maxBytes: math.MaxInt64, body: okBody, contentLength: -1, wantBody: okBody},
		{name: "huge declared length is not preallocated", maxBytes: math.MaxInt64, body: okBody, contentLength: 1 << 62, wantBody: okBody},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := newBody(tt.body)
			hc, ct := fakeClient(func(r *http.Request) (*http.Response, error) {
				return response(r, http.StatusOK, body, tt.contentLength), nil
			})
			cfg := baseConfig()
			cfg.MaxBytes = tt.maxBytes
			resp, f := httpx.Do(t.Context(), hc, cfg, testPath, []byte(testBody))
			assert.Equal(t, int32(1), body.closes.Load())
			assert.Equal(t, 1, ct.Attempts())
			if tt.wantKind == 0 {
				require.Nil(t, f)
				require.NotNil(t, resp.Body, "a body read completely is never nil")
				assert.Equal(t, tt.wantBody, string(resp.Body))
				assert.Equal(t, "SENTINEL-RESPONSE", resp.Header.Get("X-Sentinel"))
				return
			}
			f = requireFailure(t, resp, f, tt.wantKind)
			assert.Equal(t, http.StatusOK, f.Status)
			assert.Equal(t, "SENTINEL-RESPONSE", f.Header.Get("X-Sentinel"))
			assert.Nil(t, f.BodyExcerpt)
			assert.LessOrEqual(t, body.bytes.Load(), tt.wantMaxRead)
		})
	}
}

func TestDoEndlessBodyIsBounded(t *testing.T) {
	const limit = 1 << 20
	var body *endlessBody
	hc, ct := fakeClient(func(r *http.Request) (*http.Response, error) {
		body = &endlessBody{}
		return response(r, http.StatusOK, body, -1), nil
	})
	cfg := baseConfig()
	cfg.MaxBytes = limit

	resp, f := httpx.Do(t.Context(), hc, cfg, testPath, []byte(testBody))
	f = requireFailure(t, resp, f, httpx.FailureTooLarge)
	assert.Equal(t, http.StatusOK, f.Status)
	require.NotNil(t, body)
	assert.Equal(t, int64(limit+1), body.read.Load(), "exactly one byte past the limit is read")
	assert.Equal(t, int32(1), body.closes.Load())
	assert.Equal(t, 1, ct.Attempts())

	t.Run("allocation", func(t *testing.T) {
		testutil.SkipUnderRace(t)
		_, allocated := testutil.AllocStats(5, func() { _, _ = httpx.Do(t.Context(), hc, cfg, testPath, []byte(testBody)) })
		t.Logf("%.0f bytes allocated to reject an endless body at a %d-byte limit", allocated, limit)
		assert.Less(t, allocated, float64(4*limit), "memory stays a small multiple of the limit")
		assert.Equal(t, 7, ct.Attempts(), "one warm-up run and five measured runs")
	})
}

func TestDoHugeContentLengthReadsNothing(t *testing.T) {
	var body *endlessBody
	hc, ct := fakeClient(func(r *http.Request) (*http.Response, error) {
		body = &endlessBody{}
		return response(r, http.StatusOK, body, 1<<40), nil
	})
	cfg := baseConfig()
	cfg.MaxBytes = 1 << 20
	resp, f := httpx.Do(t.Context(), hc, cfg, testPath, []byte(testBody))
	f = requireFailure(t, resp, f, httpx.FailureTooLarge)
	assert.Equal(t, http.StatusOK, f.Status)
	require.NotNil(t, body)
	assert.Zero(t, body.read.Load())
	assert.Equal(t, int32(1), body.closes.Load())
	assert.Equal(t, 1, ct.Attempts())
}

// TestDoDeclaredLengthMismatchOverTheWire sends raw HTTP/1.1 responses whose
// Content-Length disagrees with the bytes that follow. The transport frames
// the body by the declared length: a short body is an unexpected EOF, and
// bytes past the declared length are not part of the body.
func TestDoDeclaredLengthMismatchOverTheWire(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantKind httpx.FailureKind // 0: success
		wantBody string
	}{
		{
			name:     "body shorter than declared",
			raw:      "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 64\r\nConnection: close\r\n\r\n{\"SENTINEL\":1}",
			wantKind: httpx.FailureTransport,
		},
		{
			name:     "body longer than declared",
			raw:      "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}SENTINEL-TRAILING-BYTES",
			wantBody: "{}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := testutil.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
				conn, buf, err := http.NewResponseController(w).Hijack()
				if !assert.NoError(t, err) {
					return
				}
				defer func() { _ = conn.Close() }()
				_, _ = buf.WriteString(tt.raw)
				_ = buf.Flush()
			})
			resp, f := httpx.Do(t.Context(), httpx.NewClient(srv.Client()), serverConfig(t, srv), testPath, []byte(testBody))
			assert.Equal(t, 1, srv.Hits())
			if tt.wantKind == 0 {
				require.Nil(t, f)
				assert.Equal(t, tt.wantBody, string(resp.Body))
				return
			}
			f = requireFailure(t, resp, f, tt.wantKind)
			assert.Equal(t, http.StatusOK, f.Status)
			assert.Equal(t, "application/json", f.Header.Get("Content-Type"))
		})
	}
}

func TestDoContentEncoding(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		values   []string
		wantKind httpx.FailureKind // 0: success
	}{
		{name: "gzip", status: http.StatusOK, values: []string{"gzip"}, wantKind: httpx.FailureEncoding},
		{name: "br", status: http.StatusOK, values: []string{"br"}, wantKind: httpx.FailureEncoding},
		{name: "deflate", status: http.StatusOK, values: []string{"deflate"}, wantKind: httpx.FailureEncoding},
		{name: "identity", status: http.StatusOK, values: []string{"identity"}},
		{name: "identity in upper case", status: http.StatusOK, values: []string{"IDENTITY"}},
		{name: "empty value", status: http.StatusOK, values: []string{""}},
		{name: "empty list elements", status: http.StatusOK, values: []string{" , identity ,\t"}},
		{name: "list with a coding", status: http.StatusOK, values: []string{"identity, gzip"}, wantKind: httpx.FailureEncoding},
		{name: "second value with a coding", status: http.StatusOK, values: []string{"identity", "br"}, wantKind: httpx.FailureEncoding},
		{name: "status wins over encoding", status: http.StatusServiceUnavailable, values: []string{"gzip"}, wantKind: httpx.FailureStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := newBody(okBody)
			hc, ct := fakeClient(func(r *http.Request) (*http.Response, error) {
				resp := response(r, tt.status, body, int64(len(okBody)))
				resp.Header["Content-Encoding"] = tt.values
				return resp, nil
			})
			resp, f := httpx.Do(t.Context(), hc, baseConfig(), testPath, []byte(testBody))
			assert.Equal(t, int32(1), body.closes.Load())
			assert.Equal(t, 1, ct.Attempts())
			if tt.wantKind == 0 {
				require.Nil(t, f)
				assert.JSONEq(t, okBody, string(resp.Body))
				return
			}
			f = requireFailure(t, resp, f, tt.wantKind)
			assert.Equal(t, tt.status, f.Status)
			assert.Equal(t, tt.values, f.Header.Values("Content-Encoding"))
			if tt.wantKind == httpx.FailureEncoding {
				assert.Zero(t, body.reads.Load(), "an undecoded body is not read")
				assert.Nil(t, f.BodyExcerpt)
			}
		})
	}
}

func TestDoContentEncodingOverTheWire(t *testing.T) {
	gzipped := gzipBytes(t, []byte(okBody))
	tests := []struct {
		name               string
		encoding           string
		payload            []byte
		disableCompression bool
		wantAcceptEncoding string
		wantKind           httpx.FailureKind // 0: success
	}{
		{name: "gzip the transport requested is decoded", encoding: "gzip", payload: gzipped, wantAcceptEncoding: "gzip"},
		{name: "unrequested gzip", encoding: "gzip", payload: gzipped, disableCompression: true, wantKind: httpx.FailureEncoding},
		{name: "unrequested br", encoding: "br", payload: []byte("SENTINEL-BROTLI"), wantAcceptEncoding: "gzip", wantKind: httpx.FailureEncoding},
	}
	for _, proto := range testutil.Protocols() {
		for _, tt := range tests {
			t.Run(proto.Name()+" "+tt.name, func(t *testing.T) {
				srv := testutil.NewServerProto(t, proto, func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Content-Encoding", tt.encoding)
					_, _ = w.Write(tt.payload)
				})
				tr := cloneTransport(t, srv)
				tr.DisableCompression = tt.disableCompression
				resp, f := httpx.Do(t.Context(), httpx.NewClient(&http.Client{Transport: tr}), serverConfig(t, srv), testPath, []byte(testBody))
				got := srv.Requests()
				require.Len(t, got, 1)
				assert.Equal(t, tt.wantAcceptEncoding, got[0].Header.Get("Accept-Encoding"),
					"only the transport ever asks for an encoding")
				if tt.wantKind == 0 {
					require.Nil(t, f)
					assert.JSONEq(t, okBody, string(resp.Body))
					assert.Empty(t, resp.Header.Values("Content-Encoding"), "the transport removes the header it decoded")
					return
				}
				f = requireFailure(t, resp, f, tt.wantKind)
				assert.Equal(t, http.StatusOK, f.Status)
				assert.Equal(t, tt.encoding, f.Header.Get("Content-Encoding"))
			})
		}
	}
}

// TestDoGzipBombIsBounded serves a small gzip stream that decodes to far
// more than the limit, only to a client that asked for gzip. The transport
// decodes it, and the limit applies to the decoded bytes.
func TestDoGzipBombIsBounded(t *testing.T) {
	const (
		limit   = 1 << 20
		decoded = 32 << 20
	)
	bomb := gzipBytes(t, make([]byte, decoded))
	require.Less(t, len(bomb), limit/4, "the bomb is small on the wire")
	for _, proto := range testutil.Protocols() {
		t.Run(proto.Name(), func(t *testing.T) {
			srv := testutil.NewServerProto(t, proto, func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
					_, _ = io.WriteString(w, okBody)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", "gzip")
				_, _ = w.Write(bomb)
			})
			hc := httpx.NewClient(srv.Client())
			cfg := serverConfig(t, srv)
			cfg.MaxBytes = limit

			resp, f := httpx.Do(t.Context(), hc, cfg, testPath, []byte(testBody))
			f = requireFailure(t, resp, f, httpx.FailureTooLarge)
			assert.Equal(t, http.StatusOK, f.Status)
			got := srv.Requests()
			require.Len(t, got, 1)
			assert.Equal(t, "gzip", got[0].Header.Get("Accept-Encoding"), "the transport asked for gzip")

			t.Run("allocation", func(t *testing.T) {
				testutil.SkipUnderRace(t)
				// The server shares the process; its share is small next to
				// the decoded size.
				_, allocated := testutil.AllocStats(3, func() { _, _ = httpx.Do(t.Context(), hc, cfg, testPath, []byte(testBody)) })
				t.Logf("%.0f bytes allocated to reject a %d-byte gzip bomb at a %d-byte limit", allocated, decoded, limit)
				assert.Less(t, allocated, float64(6*limit), "memory is bounded by the limit, not the decoded size")
				assert.Equal(t, 5, srv.Hits(), "one call, one warm-up run and three measured runs")
			})
		})
	}
}

// TestDoResponseHeaderLimit runs the SDK's header bound over a real
// connection. NewClient(nil) cannot trust the test certificate, so the
// server's client transport is cloned with the same bound; TestNewClientNil
// checks that NewClient(nil) sets it.
func TestDoResponseHeaderLimit(t *testing.T) {
	tests := []struct {
		name      string
		size      int
		unbounded bool              // keep the transport's default bound of 1 MiB
		wantKind  httpx.FailureKind // 0: success
	}{
		{name: "headers within the limit", size: 32 << 10},
		{name: "headers over the limit", size: httpx.MaxResponseHeaderBytes + 1024, wantKind: httpx.FailureTransport},
		{name: "same headers pass the default bound", size: httpx.MaxResponseHeaderBytes + 1024, unbounded: true},
	}
	for _, proto := range testutil.Protocols() {
		for _, tt := range tests {
			t.Run(proto.Name()+" "+tt.name, func(t *testing.T) {
				srv := testutil.NewServerProto(t, proto, func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("X-Sentinel-Large", strings.Repeat("h", tt.size))
					_, _ = io.WriteString(w, okBody)
				})
				tr := cloneTransport(t, srv)
				if !tt.unbounded {
					tr.MaxResponseHeaderBytes = httpx.MaxResponseHeaderBytes
				}
				resp, f := httpx.Do(t.Context(), httpx.NewClient(&http.Client{Transport: tr}), serverConfig(t, srv), testPath, []byte(testBody))
				assert.Equal(t, 1, srv.Hits())
				if tt.wantKind == 0 {
					require.Nil(t, f)
					assert.JSONEq(t, okBody, string(resp.Body))
					return
				}
				f = requireFailure(t, resp, f, tt.wantKind)
				assert.Equal(t, httpx.Failure{Kind: tt.wantKind}, *f)
			})
		}
	}
}

// cloneTransport returns a copy of s's client transport, closed when the
// test ends, so a test can change its settings without touching s's.
func cloneTransport(t *testing.T, s *testutil.Server) *http.Transport {
	t.Helper()
	base, ok := s.Client().Transport.(*http.Transport)
	require.True(t, ok)
	tr := base.Clone()
	t.Cleanup(tr.CloseIdleConnections)
	return tr
}

// gzipBytes returns p compressed with gzip.
func gzipBytes(t *testing.T, p []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	require.NoError(t, err)
	_, err = zw.Write(p)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// TestDoDoesNotPreallocateALargeDeclaredLength guards against pinning memory
// for a body that has been declared but has not arrived: a response that
// declares a large length and then stalls must not allocate it up front.
func TestDoDoesNotPreallocateALargeDeclaredLength(t *testing.T) {
	testutil.SkipUnderRace(t)
	const declared = 32 << 20
	hc, _ := fakeClient(func(r *http.Request) (*http.Response, error) {
		testutil.Drain(r)
		return response(r, http.StatusOK, stalledBody(r.Context()), declared), nil
	})
	cfg := baseConfig()
	cfg.MaxBytes = 64 << 20
	cfg.Timeout = 20 * time.Millisecond
	var f *httpx.Failure
	_, allocated := testutil.AllocStats(1, func() {
		_, f = httpx.Do(t.Context(), hc, cfg, "/p", []byte(`{}`))
	})
	require.NotNil(t, f)
	assert.Equal(t, httpx.FailureTimeout, f.Kind)
	assert.Less(t, allocated, float64(declared/4), "the declared length must not be allocated before the bytes arrive")
}
