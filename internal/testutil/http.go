package testutil

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

// RoundTripFunc adapts a function to http.RoundTripper.
type RoundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip returns f(r).
func (f RoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// CountingTransport counts the round trips it passes on to Next.
type CountingTransport struct {
	Next     http.RoundTripper
	attempts atomic.Int32
}

// RoundTrip counts the attempt and delegates to Next.
func (c *CountingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.attempts.Add(1)
	return c.Next.RoundTrip(r)
}

// Attempts returns the number of round trips so far.
func (c *CountingTransport) Attempts() int { return int(c.attempts.Load()) }

// Drain reads and closes the request body, as a real transport would, and
// returns its length.
func Drain(r *http.Request) int64 {
	if r.Body == nil {
		return 0
	}
	n, _ := io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()
	return n
}

// BlockUntilDone is an in-memory round trip that never answers: it drains the
// request, waits for the request's context to end and returns its error.
func BlockUntilDone(r *http.Request) (*http.Response, error) {
	Drain(r)
	<-r.Context().Done()
	return nil, r.Context().Err()
}

// Request is one request as a test saw it.
type Request struct {
	Method string
	Proto  string
	// URL is the request's URL as received: the path and query on a server,
	// the full URL in a transport.
	URL    string
	Header http.Header
	Body   []byte
}

// Record reads and closes r's body and returns a copy of the request.
func Record(r *http.Request) Request {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()
	}
	return Request{Method: r.Method, Proto: r.Proto, URL: r.URL.String(), Header: r.Header.Clone(), Body: body}
}

// Protocol selects the HTTP version a Server and its Client speak.
type Protocol string

// Protocols.
const (
	HTTP1 Protocol = "HTTP/1.1"
	HTTP2 Protocol = "HTTP/2.0"
)

// Name names p for subtests: "http1" or "http2". Unlike p itself, it has no
// slash, which -run would read as a subtest separator.
func (p Protocol) Name() string {
	if p == HTTP2 {
		return "http2"
	}
	return "http1"
}

// Protocols returns both protocols, for suites that run over each.
func Protocols() []Protocol { return []Protocol{HTTP1, HTTP2} }

// Server is a local TLS server that records every request before handing it
// to its handler. Handlers run on server goroutines, so they should only
// answer; tests assert on Requests afterward.
type Server struct {
	srv  *httptest.Server
	mu   sync.Mutex
	reqs []Request
}

// NewServer starts an HTTP/1.1 Server; see NewServerProto.
func NewServer(tb testing.TB, h http.HandlerFunc) *Server {
	tb.Helper()
	return NewServerProto(tb, HTTP1, h)
}

// NewServerProto starts a Server speaking proto that records each request and
// then calls h, which can still read the request body. It is closed when the test ends. Its error log is discarded:
// when a test ends while the client is still finishing a spare TLS handshake,
// the closing server would log a harmless "use of closed network connection".
func NewServerProto(tb testing.TB, proto Protocol, h http.HandlerFunc) *Server {
	tb.Helper()
	s := &Server{}
	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := Record(r)
		s.mu.Lock()
		s.reqs = append(s.reqs, rec)
		s.mu.Unlock()
		r.Body = io.NopCloser(bytes.NewReader(rec.Body))
		h(w, r)
	}))
	s.srv.EnableHTTP2 = proto == HTTP2
	s.srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	s.srv.StartTLS()
	tb.Cleanup(s.srv.Close)
	return s
}

// URL returns the server's base URL, such as https://127.0.0.1:port.
func (s *Server) URL() string { return s.srv.URL }

// Client returns an HTTP client that trusts the server's certificate and
// speaks the server's protocol.
func (s *Server) Client() *http.Client { return s.srv.Client() }

// Requests returns a copy of the recorded requests, in arrival order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.reqs)
}

// Hits returns the number of requests received.
func (s *Server) Hits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.reqs)
}
