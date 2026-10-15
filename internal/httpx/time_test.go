package httpx_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/httpx"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// timeCase is one way a call runs out of time or is canceled, in a synctest
// bubble with an in-memory transport, so elapsed times are exact.
type timeCase struct {
	name          string
	timeout       time.Duration // the call's own timeout
	clientTimeout time.Duration // an injected http.Client.Timeout; 0 means none
	callerLimit   time.Duration // a caller deadline; 0 means none
	cancelOnSend  bool          // the caller cancels once the request is sent
	cancelOnRead  bool          // the caller cancels once the body read starts
	status        int           // when set, headers arrive with it and the body stalls
	wantKind      httpx.FailureKind
	wantElapsed   time.Duration
}

// run makes the call of tc and returns its results, the elapsed time, the
// attempts and the stalled body, if any.
func (tc timeCase) run(t *testing.T) (httpx.Response, *httpx.Failure, time.Duration, int, *trackingBody) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if tc.callerLimit > 0 {
		var cancelDeadline context.CancelFunc
		ctx, cancelDeadline = context.WithTimeout(ctx, tc.callerLimit)
		defer cancelDeadline()
	}
	var body *trackingBody
	ct := &testutil.CountingTransport{Next: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if tc.cancelOnSend {
			cancel()
		}
		if tc.status == 0 {
			return testutil.BlockUntilDone(r)
		}
		testutil.Drain(r)
		body = stalledBody(r.Context())
		if tc.cancelOnRead {
			body.onRead = cancel
		}
		return response(r, tc.status, body, -1), nil
	})}
	hc := httpx.NewClient(&http.Client{Transport: ct, Timeout: tc.clientTimeout})
	cfg := baseConfig()
	cfg.Timeout = tc.timeout

	start := time.Now()
	resp, f := httpx.Do(ctx, hc, cfg, testPath, []byte(testBody))
	return resp, f, time.Since(start), ct.Attempts(), body
}

// TestDoTimeoutsAndCancellation covers every way a call can run out of time
// or be canceled, while awaiting headers or reading a 200 body. The call's
// own timeout and an injected http.Client.Timeout are timeouts; a caller
// deadline or cancellation that comes first is the caller's.
func TestDoTimeoutsAndCancellation(t *testing.T) {
	tests := []timeCase{
		{name: "call timeout awaiting headers", timeout: 2 * time.Second, wantKind: httpx.FailureTimeout, wantElapsed: 2 * time.Second},
		{name: "call timeout reading the body", timeout: 3 * time.Second, status: http.StatusOK, wantKind: httpx.FailureTimeout, wantElapsed: 3 * time.Second},
		{name: "caller deadline awaiting headers", timeout: 30 * time.Second, callerLimit: time.Second, wantKind: httpx.FailureDeadlineExceeded, wantElapsed: time.Second},
		{name: "caller deadline reading the body", timeout: 30 * time.Second, callerLimit: time.Second, status: http.StatusOK, wantKind: httpx.FailureDeadlineExceeded, wantElapsed: time.Second},
		{name: "caller canceled during send", timeout: 5 * time.Second, cancelOnSend: true, wantKind: httpx.FailureCanceled},
		{name: "caller canceled reading the body", timeout: 5 * time.Second, cancelOnRead: true, status: http.StatusOK, wantKind: httpx.FailureCanceled},
		{name: "client timeout awaiting headers", timeout: 30 * time.Second, clientTimeout: 2 * time.Second, wantKind: httpx.FailureTimeout, wantElapsed: 2 * time.Second},
		{name: "client timeout reading the body", timeout: 30 * time.Second, clientTimeout: 2 * time.Second, status: http.StatusOK, wantKind: httpx.FailureTimeout, wantElapsed: 2 * time.Second},
		{name: "caller deadline before client timeout", timeout: 30 * time.Second, clientTimeout: 2 * time.Second, callerLimit: time.Second, wantKind: httpx.FailureDeadlineExceeded, wantElapsed: time.Second},
		{name: "call timeout before client timeout", timeout: time.Second, clientTimeout: 2 * time.Second, wantKind: httpx.FailureTimeout, wantElapsed: time.Second},
		{name: "call timeout before caller deadline", timeout: time.Second, callerLimit: 2 * time.Second, wantKind: httpx.FailureTimeout, wantElapsed: time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				resp, f, elapsed, attempts, body := tt.run(t)
				f = requireFailure(t, resp, f, tt.wantKind)
				assert.Equal(t, tt.wantElapsed, elapsed)
				assert.Equal(t, 1, attempts)
				assert.Nil(t, f.BodyExcerpt)
				if tt.status == 0 {
					assert.Equal(t, httpx.Failure{Kind: tt.wantKind}, *f)
					return
				}
				assert.Equal(t, tt.status, f.Status, "a failed body read keeps the status")
				assert.Equal(t, "SENTINEL-RESPONSE", f.Header.Get("X-Sentinel"))
				require.NotNil(t, body)
				assert.Equal(t, int32(1), body.closes.Load())
			})
		})
	}
}

// TestDoStalledExcerptEndsWithTheCall reads the excerpt of a refusal whose
// body never arrives. The call still ends by its timeout, or the caller's,
// and still reports the status, without an excerpt.
func TestDoStalledExcerptEndsWithTheCall(t *testing.T) {
	tests := []timeCase{
		{name: "call timeout", timeout: 4 * time.Second, wantElapsed: 4 * time.Second},
		{name: "client timeout", timeout: 30 * time.Second, clientTimeout: 2 * time.Second, wantElapsed: 2 * time.Second},
		{name: "caller deadline", timeout: 30 * time.Second, callerLimit: time.Second, wantElapsed: time.Second},
		{name: "caller canceled", timeout: 30 * time.Second, cancelOnRead: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				tt.status = http.StatusServiceUnavailable
				resp, f, elapsed, attempts, body := tt.run(t)
				f = requireFailure(t, resp, f, httpx.FailureStatus)
				assert.Equal(t, http.StatusServiceUnavailable, f.Status)
				assert.Equal(t, "SENTINEL-RESPONSE", f.Header.Get("X-Sentinel"))
				assert.Nil(t, f.BodyExcerpt)
				assert.Equal(t, tt.wantElapsed, elapsed)
				assert.Equal(t, 1, attempts)
				require.NotNil(t, body)
				assert.Equal(t, int32(1), body.closes.Load())
			})
		})
	}
}

// TestDoStalledBodyOverTheWire checks with real connections, so in real
// time, that the standard transports end a stalled body read at the call's
// timeout: an error excerpt is dropped, and a 200 body is a timeout.
func TestDoStalledBodyOverTheWire(t *testing.T) {
	const timeout = 250 * time.Millisecond
	tests := []struct {
		name     string
		status   int
		wantKind httpx.FailureKind
	}{
		{name: "error excerpt", status: http.StatusBadGateway, wantKind: httpx.FailureStatus},
		{name: "200 body", status: http.StatusOK, wantKind: httpx.FailureTimeout},
	}
	for _, proto := range testutil.Protocols() {
		for _, tt := range tests {
			t.Run(proto.Name()+" "+tt.name, func(t *testing.T) {
				release := make(chan struct{})
				srv := testutil.NewServerProto(t, proto, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("X-Sentinel", "SENTINEL-STALL")
					w.WriteHeader(tt.status)
					_, _ = io.WriteString(w, `{"partial":`)
					_ = http.NewResponseController(w).Flush()
					select {
					case <-r.Context().Done():
					case <-release:
					}
				})
				t.Cleanup(func() { close(release) }) // runs before the server closes
				cfg := serverConfig(t, srv)
				cfg.Timeout = timeout

				start := time.Now()
				resp, f := httpx.Do(t.Context(), httpx.NewClient(srv.Client()), cfg, testPath, []byte(testBody))
				elapsed := time.Since(start)
				f = requireFailure(t, resp, f, tt.wantKind)
				assert.Equal(t, tt.status, f.Status)
				assert.Equal(t, "SENTINEL-STALL", f.Header.Get("X-Sentinel"))
				assert.Nil(t, f.BodyExcerpt)
				assert.GreaterOrEqual(t, elapsed, timeout)
				assert.Less(t, elapsed, 10*timeout, "the call ends at its timeout")
				assert.Equal(t, 1, srv.Hits())
			})
		}
	}
}
