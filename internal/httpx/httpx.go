// Package httpx is the SDK's single HTTP transport: one bounded POST attempt
// per call, no redirects, no retries, no replays, and failures that carry no
// URLs, headers of the request, credentials, causes or bodies.
//
// It is neutral: it never imports the root batchdata package, and it returns
// a *Failure that the root maps onto its public errors.
package httpx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Transport limits.
const (
	// MaxErrorBodyBytes bounds the excerpt read from a response whose status
	// is not 200.
	MaxErrorBodyBytes = 64 << 10

	// MaxResponseHeaderBytes bounds the response headers on the transport the
	// SDK builds itself.
	MaxResponseHeaderBytes = 64 << 10

	// MaxRetryAfter is the longest retry hint reported; larger hints are
	// clamped.
	MaxRetryAfter = 24 * time.Hour
)

// Config is the immutable per-client setting of every call.
type Config struct {
	// Scheme and Host form the origin, such as "https" and
	// "api.batchdata.com". Do builds the URL from them and the path as a
	// url.URL, never by concatenating strings.
	Scheme string
	Host   string

	// Authorization is the full header value, such as "Bearer <token>".
	Authorization string

	// UserAgent is the User-Agent header value.
	UserAgent string

	// Timeout bounds the call, including reading the whole body. It must be
	// positive.
	Timeout time.Duration

	// MaxBytes is the response body limit for a 200 response. It must be
	// positive.
	MaxBytes int64
}

// Response is a fully read HTTP 200 response.
type Response struct {
	Header http.Header
	Body   []byte
}

// FailureKind classifies a Failure.
type FailureKind uint8

// Failure kinds.
const (
	// FailureCanceled: the caller's context was canceled, before or during
	// the call.
	FailureCanceled FailureKind = iota + 1

	// FailureDeadlineExceeded: the caller's context deadline expired.
	FailureDeadlineExceeded

	// FailureTimeout: the call's own timeout, or the http.Client's Timeout,
	// expired first.
	FailureTimeout

	// FailureTransport: any other failure to complete the exchange.
	FailureTransport

	// FailureStatus: the response status was not 200. Header and
	// BodyExcerpt are set.
	FailureStatus

	// FailureTooLarge: a 200 response body exceeded MaxBytes, by its
	// declared length or by what was read.
	FailureTooLarge

	// FailureEncoding: a 200 response carried a Content-Encoding the
	// transport did not decode.
	FailureEncoding
)

// Failure is a call that did not produce a usable 200 response. It holds no
// cause: the underlying error, which may quote URLs, proxy credentials or
// network addresses, is dropped.
type Failure struct {
	Kind FailureKind

	// Status is the HTTP status, or 0 when no response arrived.
	Status int

	// Header is the response header, or nil when no response arrived.
	Header http.Header

	// BodyExcerpt is the body of a FailureStatus response when it was read
	// completely within MaxErrorBodyBytes, and nil otherwise.
	BodyExcerpt []byte
}

// NewClient returns the client every call uses. A non-nil hc is copied with
// redirects disabled and no cookie jar: its Transport is shared and never
// mutated, its Timeout still applies, and hc itself is never modified. A nil
// hc yields a client on a new transport owned by the SDK, which mirrors
// http.DefaultTransport's settings, proxy variables included, and bounds
// response headers to MaxResponseHeaderBytes.
func NewClient(hc *http.Client) *http.Client {
	var c http.Client
	if hc == nil {
		c.Transport = newTransport()
	} else {
		c = *hc
	}
	c.CheckRedirect = noRedirect
	c.Jar = nil
	return &c
}

// newTransport returns a transport with the settings of http.DefaultTransport,
// built anew so that process-wide changes to http.DefaultTransport do not
// reach it, plus a bound on response headers.
func newTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           100,
		IdleConnTimeout:        90 * time.Second,
		TLSHandshakeTimeout:    10 * time.Second,
		ExpectContinueTimeout:  1 * time.Second,
		MaxResponseHeaderBytes: MaxResponseHeaderBytes,
	}
}

// noRedirect stops the client at the first response: a redirect is returned
// as a FailureStatus, and credentials are never sent to another URL.
func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// Do makes exactly one POST attempt of body to path on cfg's origin and reads
// the whole response, bounded by cfg.MaxBytes. It checks ctx immediately
// before the attempt and makes none when ctx is done. The timeout covers the
// request and the full body read; the caller's deadline still applies. The
// request has no GetBody, so the standard library cannot resend it, and the
// response body is closed on every path.
//
// Exactly one of the results is meaningful: a Response for status 200, or a
// non-nil *Failure. hc is a client from NewClient.
func Do(ctx context.Context, hc *http.Client, cfg Config, path string, body []byte) (Response, *Failure) {
	callCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	req, err := newRequest(callCtx, cfg, path, body)
	if err != nil {
		// The error quotes the URL, so it is dropped.
		return Response{}, &Failure{Kind: FailureTransport}
	}

	// A done context, the caller's or this call's, means no attempt at all.
	if callCtx.Err() != nil {
		return Response{}, classify(ctx, callCtx, nil)
	}
	resp, err := hc.Do(req)
	if err != nil {
		// No response body is open: on error, http.Client closes any response
		// it got before returning.
		return Response{}, classify(ctx, callCtx, err)
	}
	// A close error changes nothing: the outcome is already decided.
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Response{}, &Failure{
			Kind:        FailureStatus,
			Status:      resp.StatusCode,
			Header:      resp.Header,
			BodyExcerpt: excerpt(resp.Body, resp.ContentLength),
		}
	}
	if encoded(resp.Header) {
		return Response{}, &Failure{Kind: FailureEncoding, Status: http.StatusOK, Header: resp.Header}
	}
	// Clamped so that maxBytes+1 cannot overflow; a non-positive limit admits
	// only an empty body.
	maxBytes := min(max(cfg.MaxBytes, 0), math.MaxInt64-1)
	if resp.ContentLength > maxBytes {
		return Response{}, &Failure{Kind: FailureTooLarge, Status: http.StatusOK, Header: resp.Header}
	}
	data, err := readBody(resp.Body, resp.ContentLength, maxBytes)
	if err != nil {
		f := classify(ctx, callCtx, err)
		f.Status, f.Header = http.StatusOK, resp.Header
		return Response{}, f
	}
	if int64(len(data)) > maxBytes {
		return Response{}, &Failure{Kind: FailureTooLarge, Status: http.StatusOK, Header: resp.Header}
	}
	return Response{Header: resp.Header, Body: data}, nil
}

// newRequest builds the POST of body to path on cfg's origin. The URL is a
// url.URL, never a concatenated string, so the path cannot add a query, a
// fragment or another host.
func newRequest(ctx context.Context, cfg Config, path string, body []byte) (*http.Request, error) {
	u := url.URL{Scheme: cfg.Scheme, Host: cfg.Host, Path: path}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	// A rewindable body lets the standard library send the request again on
	// its own: HTTP/2 resends it when a stream is refused or the connection
	// goes away, even after the server has read the whole body, and HTTP/1.1
	// resends a replayable request when a reused connection fails. Without
	// GetBody every call is one attempt.
	req.GetBody = nil
	if len(body) == 0 {
		// NewRequestWithContext turns an empty body into http.NoBody, which
		// HTTP/2 treats as safe to resend whatever GetBody says. An empty
		// reader of its own is never resent; it goes out as an empty chunked
		// body instead of Content-Length: 0.
		req.Body = io.NopCloser(bytes.NewReader(nil))
	}
	setHeaders(req.Header, cfg)
	return req, nil
}

// setHeaders sets the request headers of every call on h, a fresh header map.
// The values share one backing array allocated per call, so no two requests
// share header memory; each is a full slice expression, so an append to one
// cannot overwrite the next.
//
// Accept-Encoding is left to the transport, which then decodes gzip itself,
// and there is no Idempotency-Key: with one, the standard library may replay
// a POST on a reused connection.
func setHeaders(h http.Header, cfg Config) {
	values := []string{"application/json", "application/json", cfg.UserAgent, cfg.Authorization}
	h["Content-Type"] = values[0:1:1]
	h["Accept"] = values[1:2:2]
	h["User-Agent"] = values[2:3:3]
	if cfg.Authorization != "" {
		h["Authorization"] = values[3:4:4]
	}
}

// classify maps a call that ended without a usable response onto a failure
// kind: the caller's context first, then this call's own timeout, then any
// other error that matches context.DeadlineExceeded, such as an injected
// http.Client's Timeout, else a transport failure. err is never kept.
func classify(caller, call context.Context, err error) *Failure {
	if cerr := caller.Err(); cerr != nil {
		if errors.Is(cerr, context.DeadlineExceeded) {
			return &Failure{Kind: FailureDeadlineExceeded}
		}
		return &Failure{Kind: FailureCanceled}
	}
	if errors.Is(call.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return &Failure{Kind: FailureTimeout}
	}
	return &Failure{Kind: FailureTransport}
}

// excerpt returns the body of a response whose status is not 200 when it
// reads completely within MaxErrorBodyBytes, and nil when it is declared or
// found longer, or its read fails. A stalled read ends with the call's
// context, which the transport watches.
func excerpt(body io.Reader, contentLength int64) []byte {
	if contentLength > MaxErrorBodyBytes {
		return nil
	}
	b, err := readBody(body, contentLength, MaxErrorBodyBytes)
	if err != nil || len(b) > MaxErrorBodyBytes {
		return nil
	}
	return b
}

// encoded reports whether h carries a Content-Encoding other than identity.
// The transport removes the header when it decodes gzip itself, so one that
// remains names an encoding nobody decoded. Every value counts, as a list of
// codings; empty list elements are ignored.
func encoded(h http.Header) bool {
	for _, v := range h.Values("Content-Encoding") {
		for coding := range strings.SplitSeq(v, ",") {
			coding = strings.Trim(coding, " \t")
			if coding != "" && !strings.EqualFold(coding, "identity") {
				return true
			}
		}
	}
	return false
}

// maxPrealloc bounds the buffer sized from a declared length before any byte
// arrives. A larger declared length is read into a buffer that grows with
// what actually arrives, so a response that declares a large body and then
// stalls cannot pin that much memory until the timeout.
const maxPrealloc = 1 << 20

// readBody reads r to EOF, but never more than maxBytes+1 bytes, so a result
// longer than maxBytes means the body is too large; maxBytes+1 must not
// overflow. A declared length, already known to be within the limit, sizes
// the buffer exactly, with one spare byte to see EOF without growing. A body
// of unknown length is read by io.ReadAll, which grows as bytes arrive.
func readBody(r io.Reader, contentLength, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(r, maxBytes+1)
	if contentLength < 0 || contentLength > maxPrealloc {
		return io.ReadAll(limited)
	}
	buf := make([]byte, 0, contentLength+1)
	for len(buf) < cap(buf) {
		n, err := limited.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if errors.Is(err, io.EOF) {
			return buf, nil
		}
		if err != nil {
			return buf, err
		}
	}
	// The body is longer than declared: read on, still within the limit.
	rest, err := io.ReadAll(limited)
	return append(buf, rest...), err
}
