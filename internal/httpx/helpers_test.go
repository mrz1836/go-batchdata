package httpx_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/httpx"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// Synthetic fixtures. Nothing here is a real key, host or record.
const (
	testKey       = "SENTINEL-KEY-0000"
	testAuth      = "Bearer " + testKey
	testHost      = "example.test"
	testPath      = "/api/v3/property/skip-trace"
	testURL       = "https://" + testHost + testPath
	testUserAgent = "go-batchdata-test SENTINEL-UA"
	testBody      = `{"requests":[{"propertyAddress":{"street":"1 SENTINEL Way","city":"Example","state":"EX","zip":"00000"}}]}`
	okBody        = `{"status":{"code":200,"text":"SENTINEL-OK"},"results":{}}`
)

var errReset = errors.New("SENTINEL-READ-ERR: connection reset by peer")

// baseConfig is the Config of in-memory calls to example.test.
func baseConfig() httpx.Config {
	return httpx.Config{
		Scheme:        "https",
		Host:          testHost,
		Authorization: testAuth,
		UserAgent:     testUserAgent,
		Timeout:       5 * time.Second,
		MaxBytes:      1024,
	}
}

// serverConfig is baseConfig aimed at s, whose loopback URL TestEndpoint
// turns into the host.
func serverConfig(t *testing.T, s *testutil.Server) httpx.Config {
	t.Helper()
	host, ok := httpx.TestEndpoint(s.URL())
	require.True(t, ok, "a test server URL must be a valid test endpoint")
	cfg := baseConfig()
	cfg.Host = host
	return cfg
}

// fakeClient returns a client from NewClient whose transport answers in
// memory with fn and counts the attempts.
func fakeClient(fn testutil.RoundTripFunc) (*http.Client, *testutil.CountingTransport) {
	ct := &testutil.CountingTransport{Next: fn}
	return httpx.NewClient(&http.Client{Transport: ct}), ct
}

// response is an in-memory response to r.
func response(r *http.Request, status int, body io.ReadCloser, contentLength int64) *http.Response {
	return &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"X-Sentinel": {"SENTINEL-RESPONSE"}},
		Body:          body,
		ContentLength: contentLength,
		Request:       r,
	}
}

// trackingBody is a response body that counts reads, bytes and closes. With
// done set, every Read first calls onRead, then waits for done and fails
// with doneErr.
type trackingBody struct {
	r        io.Reader
	readErr  error // replaces io.EOF at the end of r
	closeErr error
	done     <-chan struct{}
	doneErr  func() error
	onRead   func()
	reads    atomic.Int32
	bytes    atomic.Int64
	closes   atomic.Int32
}

func newBody(s string) *trackingBody { return &trackingBody{r: strings.NewReader(s)} }

// stalledBody returns a body whose reads block until ctx ends.
func stalledBody(ctx context.Context) *trackingBody {
	return &trackingBody{done: ctx.Done(), doneErr: ctx.Err}
}

func (b *trackingBody) Read(p []byte) (int, error) {
	b.reads.Add(1)
	if b.done != nil {
		if b.onRead != nil {
			b.onRead()
		}
		<-b.done
		return 0, b.doneErr()
	}
	n, err := b.r.Read(p)
	b.bytes.Add(int64(n))
	if errors.Is(err, io.EOF) && b.readErr != nil {
		return n, b.readErr
	}
	return n, err
}

func (b *trackingBody) Close() error {
	b.closes.Add(1)
	return b.closeErr
}

// endlessBody streams 'x' forever and counts the bytes read from it.
type endlessBody struct {
	read   atomic.Int64
	closes atomic.Int32
}

func (b *endlessBody) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	b.read.Add(int64(len(p)))
	return len(p), nil
}

func (b *endlessBody) Close() error {
	b.closes.Add(1)
	return nil
}

// kindName names a failure kind for readable assertion messages.
func kindName(k httpx.FailureKind) string {
	switch k {
	case httpx.FailureCanceled:
		return "canceled"
	case httpx.FailureDeadlineExceeded:
		return "deadline exceeded"
	case httpx.FailureTimeout:
		return "timeout"
	case httpx.FailureTransport:
		return "transport"
	case httpx.FailureStatus:
		return "status"
	case httpx.FailureTooLarge:
		return "too large"
	case httpx.FailureEncoding:
		return "encoding"
	default:
		return "unknown"
	}
}

// requireFailure checks that a call failed with kind and returned no
// Response, and returns the failure.
func requireFailure(t *testing.T, resp httpx.Response, f *httpx.Failure, kind httpx.FailureKind) *httpx.Failure {
	t.Helper()
	require.NotNil(t, f, "the call must fail")
	require.Equal(t, kindName(kind), kindName(f.Kind))
	require.Zero(t, resp, "a failed call returns no Response")
	return f
}
