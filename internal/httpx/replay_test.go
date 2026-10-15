package httpx_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/httpx"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// HTTP/2 frame types, flags and error codes used by frameServer.
const (
	frameData     = 0x0
	frameHeaders  = 0x1
	frameRST      = 0x3
	frameSettings = 0x4
	framePing     = 0x6
	frameGoAway   = 0x7
	frameWindow   = 0x8
	flagEnd       = 0x1 // END_STREAM on HEADERS and DATA, ACK on SETTINGS and PING
	h2Preface     = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
	codeNoError   = 0x0
	codeProtocol  = 0x1 // PROTOCOL_ERROR
	codeRefused   = 0x7 // REFUSED_STREAM
	maxFrameBytes = 1 << 24
)

var errFrameTooLarge = errors.New("frame too large")

// frameServer is a minimal HTTP/2 server that reads each request completely
// and then refuses it: it resets the stream with code, or, with goAway set,
// sends a GOAWAY that claims no stream was processed. A server that has the
// whole request may already have acted on it, so the client must not send it
// again.
type frameServer struct {
	ln       net.Listener
	code     uint32
	goAway   bool
	requests atomic.Int64
	wg       sync.WaitGroup
	mu       sync.Mutex
	conns    []net.Conn
}

// startFrameServer starts a frameServer and returns it with a transport
// that trusts its certificate and negotiates HTTP/2, as the SDK's own
// transport does.
func startFrameServer(t *testing.T, code uint32, goAway bool) (*frameServer, *http.Transport) {
	t.Helper()
	ts := httptest.NewTLSServer(http.NotFoundHandler())
	cert := ts.TLS.Certificates[0]
	base, ok := ts.Client().Transport.(*http.Transport)
	require.True(t, ok)
	tr := base.Clone()
	tr.ForceAttemptHTTP2 = true
	ts.Close()

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2"},
		MinVersion:   tls.VersionTLS12,
	})
	require.NoError(t, err)
	s := &frameServer{ln: ln, code: code, goAway: goAway}
	s.wg.Go(s.accept)
	t.Cleanup(func() {
		_ = ln.Close()
		tr.CloseIdleConnections()
		s.mu.Lock()
		for _, c := range s.conns {
			_ = c.Close()
		}
		s.mu.Unlock()
		s.wg.Wait()
	})
	return s, tr
}

// url returns the server's base URL.
func (s *frameServer) url() string { return "https://" + s.ln.Addr().String() }

func (s *frameServer) accept() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns = append(s.conns, c)
		s.mu.Unlock()
		s.wg.Go(func() { s.serve(c) })
	}
}

func (s *frameServer) serve(c net.Conn) {
	defer func() { _ = c.Close() }()
	r := bufio.NewReader(c)
	if _, err := io.ReadFull(r, make([]byte, len(h2Preface))); err != nil {
		return
	}
	_ = writeFrame(c, frameSettings, 0, 0, nil)
	for {
		typ, flags, stream, payload, err := readFrame(r)
		if err != nil {
			return
		}
		s.handle(c, typ, flags, stream, payload)
	}
}

// handle answers one client frame: it acknowledges settings and pings,
// counts requests, returns flow-control credit, and refuses each request once
// it is complete. After a GOAWAY it keeps reading until the client closes
// the connection, so the client sees the GOAWAY and not a broken connection.
func (s *frameServer) handle(w io.Writer, typ, flags byte, stream uint32, payload []byte) {
	end := flags&flagEnd != 0
	switch typ {
	case frameSettings:
		if !end {
			_ = writeFrame(w, frameSettings, flagEnd, 0, nil) // an ACK carries no payload
		}
		return
	case framePing:
		if !end {
			_ = writeFrame(w, framePing, flagEnd, 0, payload)
		}
		return
	case frameHeaders:
		s.requests.Add(1)
	case frameData:
		if n := len(payload); n > 0 && n <= maxFrameBytes {
			_ = writeFrame(w, frameWindow, 0, 0, binary.BigEndian.AppendUint32(nil, uint32(n)))
		}
	default:
		return
	}
	if !end {
		return
	}
	if s.goAway {
		// Last stream ID 0: the server claims it processed nothing.
		payload := binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, 0), codeNoError)
		_ = writeFrame(w, frameGoAway, 0, 0, payload)
		return
	}
	_ = writeFrame(w, frameRST, 0, stream, binary.BigEndian.AppendUint32(nil, s.code))
}

func readFrame(r io.Reader) (typ, flags byte, stream uint32, payload []byte, err error) {
	var hdr [9]byte
	if _, err = io.ReadFull(r, hdr[:]); err != nil {
		return 0, 0, 0, nil, err
	}
	n := int(hdr[0])<<16 | int(hdr[1])<<8 | int(hdr[2])
	if n > maxFrameBytes {
		return 0, 0, 0, nil, errFrameTooLarge
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(r, payload); err != nil {
		return 0, 0, 0, nil, err
	}
	return hdr[3], hdr[4], binary.BigEndian.Uint32(hdr[5:]) & 0x7fffffff, payload, nil
}

func writeFrame(w io.Writer, typ, flags byte, stream uint32, payload []byte) error {
	n := len(payload)
	if n > maxFrameBytes {
		return errFrameTooLarge
	}
	frame := make([]byte, 9, 9+n)
	binary.BigEndian.PutUint32(frame[0:4], uint32(n)<<8) // 24-bit length, then the type byte
	frame[3], frame[4] = typ, flags
	binary.BigEndian.PutUint32(frame[5:], stream)
	_, err := w.Write(append(frame, payload...))
	return err
}

// TestDoNeverReplaysOverHTTP2 guards the one-attempt contract against the
// standard HTTP/2 client, which resends a request whose body it can rewind,
// or which has no body, when a stream is refused or reset or the connection
// goes away, even after the server has read the whole request.
func TestDoNeverReplaysOverHTTP2(t *testing.T) {
	tests := []struct {
		name   string
		code   uint32
		goAway bool
		body   string
	}{
		{name: "refused stream", code: codeRefused, body: testBody},
		{name: "protocol error", code: codeProtocol, body: testBody},
		{name: "goaway", goAway: true, body: testBody},
		{name: "refused stream with an empty body", code: codeRefused},
		{name: "goaway with an empty body", goAway: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, tr := startFrameServer(t, tt.code, tt.goAway)
			host, ok := httpx.TestEndpoint(s.url())
			require.True(t, ok)
			cfg := baseConfig()
			cfg.Host = host
			cfg.Timeout = 5 * time.Second

			start := time.Now()
			resp, f := httpx.Do(t.Context(), httpx.NewClient(&http.Client{Transport: tr}), cfg, testPath, []byte(tt.body))
			elapsed := time.Since(start)
			requireFailure(t, resp, f, httpx.FailureTransport)
			assert.Equal(t, int64(1), s.requests.Load(), "the POST is sent exactly once")
			assert.Less(t, elapsed, cfg.Timeout, "the call fails at once instead of resending until the timeout")
		})
	}
}

// TestHTTP2ReplayControls proves that the frame server can detect a resend:
// the standard client resends a POST with GetBody, or with no body at all,
// on a refused stream and on GOAWAY. PROTOCOL_ERROR has no control: Go 1.26
// resends on it but Go 1.27 does not.
func TestHTTP2ReplayControls(t *testing.T) {
	tests := []struct {
		name   string
		code   uint32
		goAway bool
		body   func() io.Reader
	}{
		{name: "refused stream with get body", code: codeRefused, body: func() io.Reader { return bytes.NewReader([]byte(testBody)) }},
		{name: "goaway with get body", goAway: true, body: func() io.Reader { return bytes.NewReader([]byte(testBody)) }},
		{name: "refused stream with no body", code: codeRefused, body: func() io.Reader { return http.NoBody }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, tr := startFrameServer(t, tt.code, tt.goAway)
			ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url()+testPath, tt.body())
			require.NoError(t, err)
			resp, err := (&http.Client{Transport: tr}).Do(req)
			if resp != nil {
				_ = resp.Body.Close()
			}
			require.Error(t, err)
			assert.GreaterOrEqual(t, s.requests.Load(), int64(2), "the standard client resent the request")
		})
	}
}

// staleServer is an HTTP/1.1 server that answers every request but the
// second: it reads that one in full, as the recording server does, and then
// closes the connection without answering. Its traced transport records
// whether each connection the client used was a reused one.
type staleServer struct {
	srv    *testutil.Server
	traced testutil.RoundTripFunc
	mu     sync.Mutex
	reused []bool
}

func newStaleServer(t *testing.T) *staleServer {
	t.Helper()
	s := &staleServer{}
	var calls atomic.Int32
	s.srv = testutil.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) != 2 {
			_, _ = io.WriteString(w, okBody)
			return
		}
		conn, _, err := http.NewResponseController(w).Hijack()
		if assert.NoError(t, err) {
			_ = conn.Close()
		}
	})
	inner := cloneTransport(t, s.srv)
	s.traced = func(r *http.Request) (*http.Response, error) {
		trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) {
			s.mu.Lock()
			s.reused = append(s.reused, info.Reused)
			s.mu.Unlock()
		}}
		return inner.RoundTrip(r.WithContext(httptrace.WithClientTrace(r.Context(), trace)))
	}
	return s
}

// reusedConns returns, for each connection the client got, whether it was
// reused.
func (s *staleServer) reusedConns() []bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.reused)
}

// sdkCall returns a call through Do that reports whether it succeeded.
func (s *staleServer) sdkCall(t *testing.T) func() bool {
	hc := httpx.NewClient(&http.Client{Transport: s.traced})
	cfg := serverConfig(t, s.srv)
	return func() bool {
		t.Helper()
		_, f := httpx.Do(t.Context(), hc, cfg, testPath, []byte(testBody))
		if f != nil {
			assert.Equal(t, kindName(httpx.FailureTransport), kindName(f.Kind))
		}
		return f == nil
	}
}

// rawCall returns a call through a raw client, with GetBody and, when set,
// the header, that reports whether it succeeded.
func (s *staleServer) rawCall(t *testing.T, header string) func() bool {
	hc := &http.Client{Transport: s.traced}
	return func() bool {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, s.srv.URL()+testPath, bytes.NewReader([]byte(testBody)))
		require.NoError(t, err)
		require.NotNil(t, req.GetBody)
		if header != "" {
			req.Header.Set(header, "SENTINEL-IDEMPOTENCY-0000")
		}
		resp, err := hc.Do(req)
		if err != nil {
			return false
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		require.NoError(t, resp.Body.Close())
		return true
	}
}

// TestDoNeverReplaysOnAStaleHTTP1Connection reuses a keep-alive connection
// that the server then closes after reading the next request in full,
// without answering. The standard client resends a replayable request, one
// with GetBody and an Idempotency-Key, on a new connection; the SDK's
// request is never replayable.
func TestDoNeverReplaysOnAStaleHTTP1Connection(t *testing.T) {
	tests := []struct {
		name     string
		sdk      bool
		header   string // an extra request header of the raw client
		wantHits int
	}{
		{name: "sdk", sdk: true, wantHits: 2},
		{name: "raw client control with an idempotency key", header: "Idempotency-Key", wantHits: 3},
		{name: "raw client control with an x idempotency key", header: "X-Idempotency-Key", wantHits: 3},
		{name: "raw client control without a key", wantHits: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStaleServer(t)
			call := s.rawCall(t, tt.header)
			if tt.sdk {
				call = s.sdkCall(t)
			}
			require.True(t, call(), "the first call succeeds and leaves an idle connection")
			second := call()
			assert.Equal(t, tt.wantHits, s.srv.Hits())
			assert.Equal(t, tt.wantHits == 3, second, "only a replayed call can succeed")
			reused := s.reusedConns()
			require.GreaterOrEqual(t, len(reused), 2)
			assert.True(t, reused[1], "the second call starts on the first call's connection")
		})
	}
}
