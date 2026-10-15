package testutil_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// sentinel is the planted secret the leak assertions must catch.
const sentinel = "SENTINEL-SECRET-5e1d"

// errPlanted is an error that is not a *plantedError.
var errPlanted = errors.New("planted failure")

// plantedError is an error type for RequireErrorAs to find in a chain.
type plantedError struct{}

func (*plantedError) Error() string { return "planted" }

// recorder is a testing.TB that records failures and skips instead of acting
// on them, so a test can watch a helper fail. Methods it does not override
// panic, through the nil embedded TB.
type recorder struct {
	testing.TB

	failures []string
	skipped  bool
}

func (r *recorder) Helper() {}

func (r *recorder) Name() string { return "recorder" }

func (r *recorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *recorder) FailNow() { r.failures = append(r.failures, "FailNow") }

func (r *recorder) Skip(...any) { r.skipped = true }

// leaky prints nothing secret through String, but holds the secret in an
// unexported field that reflection reaches.
type leaky struct {
	secret string
}

func (leaky) String() string { return "leaky{redacted}" }

// sealed keeps its secret behind a closure, which fmt cannot print.
type sealed struct {
	secret func() string
}

func (sealed) String() string { return "sealed{redacted}" }

func TestAssertRedacted(t *testing.T) {
	h := hex.EncodeToString([]byte(sentinel))
	tests := []struct {
		name   string
		out    string
		leaked bool
	}{
		{name: "clean", out: "nothing to see"},
		{name: "plain", out: "x " + sentinel + " y", leaked: true},
		{name: "lower-case hex", out: "x " + h + " y", leaked: true},
		{name: "upper-case hex", out: "x " + strings.ToUpper(h) + " y", leaked: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorder{}
			ok := testutil.AssertRedacted(rec, tt.out, []string{"other", sentinel})
			assert.Equal(t, !tt.leaked, ok)
			assert.Equal(t, tt.leaked, len(rec.failures) > 0, "failures: %v", rec.failures)
		})
	}
}

func TestAssertNeverPrints(t *testing.T) {
	t.Run("reflection reaches an unexported field", func(t *testing.T) {
		rec := &recorder{}
		assert.False(t, testutil.AssertNeverPrints(rec, leaky{secret: sentinel}, sentinel))
		require.NotEmpty(t, rec.failures)
		assert.Contains(t, rec.failures[0], "on testutil_test.leaky")
	})
	t.Run("a closure hides the secret", func(t *testing.T) {
		rec := &recorder{}
		assert.True(t, testutil.AssertNeverPrints(rec, sealed{secret: func() string { return sentinel }}, sentinel))
		assert.Empty(t, rec.failures)
	})
}

func TestShapes(t *testing.T) {
	v := leaky{secret: sentinel}
	shapes := testutil.Shapes(&v)
	require.Len(t, shapes, 10)
	names := map[string]bool{}
	for _, s := range shapes {
		names[s.Name] = true
		if s.Name == "pointer to pointer" {
			continue // fmt prints only its address
		}
		rec := &recorder{}
		assert.False(t, testutil.AssertNeverPrints(rec, s.Value, sentinel), "every shape reaches the field: %s", s.Name)
	}
	assert.Len(t, names, len(shapes), "shape names are unique")
	assert.Len(t, testutil.FormatVerbs(), 15)
}

func TestAssertShapesNeverPrint(t *testing.T) {
	t.Run("reflection reaches an unexported field", func(t *testing.T) {
		rec := &recorder{}
		v := leaky{secret: sentinel}
		assert.False(t, testutil.AssertShapesNeverPrint(rec, &v, sentinel))
		assert.NotEmpty(t, rec.failures)
	})
	t.Run("a closure hides the secret", func(t *testing.T) {
		rec := &recorder{}
		v := sealed{secret: func() string { return sentinel }}
		assert.True(t, testutil.AssertShapesNeverPrint(rec, &v, sentinel))
		assert.Empty(t, rec.failures)
	})
}

func TestAssertDiffRedacted(t *testing.T) {
	t.Run("a diff prints the secret", func(t *testing.T) {
		rec := &recorder{}
		a, b := leaky{secret: sentinel}, leaky{secret: "other"}
		assert.False(t, testutil.AssertDiffRedacted(rec, &a, &b, sentinel))
		assert.NotEmpty(t, rec.failures)
	})
	t.Run("a diff without secrets", func(t *testing.T) {
		rec := &recorder{}
		a := sealed{secret: func() string { return sentinel }}
		b := sealed{secret: func() string { return "other" }}
		assert.True(t, testutil.AssertDiffRedacted(rec, &a, &b, sentinel))
		assert.Empty(t, rec.failures)
	})
	t.Run("equal values print nothing to check", func(t *testing.T) {
		rec := &recorder{}
		a := leaky{secret: "same"}
		assert.False(t, testutil.AssertDiffRedacted(rec, &a, &a, sentinel))
		assert.NotEmpty(t, rec.failures)
	})
}

func TestRequireErrorAs(t *testing.T) {
	want := &plantedError{}
	assert.Same(t, want, testutil.RequireErrorAs[*plantedError](t, fmt.Errorf("wrapped: %w", want)))

	rec := &recorder{}
	assert.Nil(t, testutil.RequireErrorAs[*plantedError](rec, nil))
	assert.Len(t, rec.failures, 4, "a nil error fails both checks, each with a message and FailNow")

	rec = &recorder{}
	assert.Nil(t, testutil.RequireErrorAs[*plantedError](rec, errPlanted))
	require.Len(t, rec.failures, 2, "another type fails the type check: a message and FailNow")
	assert.Contains(t, rec.failures[0], "want a *testutil_test.plantedError, got *errors.errorString")
}

func TestRequireJSONRoundTrip(t *testing.T) {
	type exported struct {
		A int    `json:"a"`
		B string `json:"b"`
	}
	testutil.RequireJSONRoundTrip(t, exported{A: 1, B: "b"})

	type lossy struct {
		A int `json:"a"`
		b int
	}
	rec := &recorder{}
	testutil.RequireJSONRoundTrip(rec, lossy{A: 1, b: 2}, "SENTINEL-NOTE")
	require.Len(t, rec.failures, 2, "a lost field fails the comparison: a message and FailNow")
	assert.Contains(t, rec.failures[0], "SENTINEL-NOTE")

	rec = &recorder{}
	testutil.RequireJSONRoundTrip(rec, make(chan int))
	assert.NotEmpty(t, rec.failures, "a value that cannot be encoded fails")
}

func TestSkipUnderRace(t *testing.T) {
	rec := &recorder{}
	testutil.SkipUnderRace(rec)
	assert.Equal(t, testutil.RaceEnabled, rec.skipped)
}

func TestAllocStats(t *testing.T) {
	var sink []byte
	allocs, allocated := testutil.AllocStats(10, func() { sink = make([]byte, 1024) })
	assert.GreaterOrEqual(t, allocs, 1.0)
	assert.GreaterOrEqual(t, allocated, 1024.0)
	_ = sink
}

func TestServerRecordsRequests(t *testing.T) {
	for _, proto := range testutil.Protocols() {
		t.Run(proto.Name(), func(t *testing.T) {
			srv := testutil.NewServerProto(t, proto, func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(w, r.Body)
			})
			ct := &testutil.CountingTransport{Next: srv.Client().Transport}
			hc := &http.Client{Transport: ct}
			for _, body := range []string{"first", "second"} {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL()+"/path?q=1", strings.NewReader(body))
				require.NoError(t, err)
				resp, err := hc.Do(req)
				require.NoError(t, err)
				echo, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
				assert.Equal(t, body, string(echo), "the handler still reads the recorded body")
			}
			assert.Equal(t, 2, ct.Attempts())
			assert.Equal(t, 2, srv.Hits())
			reqs := srv.Requests()
			require.Len(t, reqs, 2)
			assert.Equal(t, string(proto), reqs[1].Proto)
			assert.Equal(t, http.MethodPost, reqs[1].Method)
			assert.Equal(t, "/path?q=1", reqs[1].URL)
			assert.Equal(t, "second", string(reqs[1].Body))
		})
	}
}

func TestNewServerSpeaksHTTP1(t *testing.T) {
	srv := testutil.NewServer(t, func(http.ResponseWriter, *http.Request) {})
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL(), nil)
	require.NoError(t, err)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	reqs := srv.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, string(testutil.HTTP1), reqs[0].Proto)
}

func TestProtocolName(t *testing.T) {
	assert.Equal(t, "http1", testutil.HTTP1.Name())
	assert.Equal(t, "http2", testutil.HTTP2.Name())
	assert.Equal(t, []testutil.Protocol{testutil.HTTP1, testutil.HTTP2}, testutil.Protocols())
}

func TestRequestHelpers(t *testing.T) {
	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://127.0.0.1/", strings.NewReader("12345"))
	require.NoError(t, err)
	assert.Equal(t, int64(5), testutil.Drain(r))
	r.Body = nil
	assert.Zero(t, testutil.Drain(r))
	assert.Equal(t, testutil.Request{Method: http.MethodPost, Proto: "HTTP/1.1", URL: "https://127.0.0.1/", Header: http.Header{}}, testutil.Record(r))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	resp, err := testutil.RoundTripFunc(testutil.BlockUntilDone).RoundTrip(r.WithContext(ctx))
	if resp != nil {
		_ = resp.Body.Close()
	}
	assert.Nil(t, resp)
	require.ErrorIs(t, err, context.Canceled)
}
