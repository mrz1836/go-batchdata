package httpx_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/httpx"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// seenRequest is what an in-memory transport saw of the request Do built.
type seenRequest struct {
	method        string
	url           string
	path          string
	rawQuery      string
	host          string
	header        http.Header
	body          []byte
	contentLength int64
	noBody        bool
	getBody       bool
}

func TestDoSendsExactRequest(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		body     string
		auth     string
		wantURL  string
		wantPath string
		wantAuth []string
	}{
		{
			name: "json body", path: testPath, body: testBody, auth: testAuth,
			wantURL: testURL, wantPath: testPath, wantAuth: []string{testAuth},
		},
		{
			name: "query and fragment characters stay in the path", path: "/api/v3/x?SENTINEL=1#frag", body: testBody, auth: testAuth,
			wantURL: "https://" + testHost + "/api/v3/x%3FSENTINEL=1%23frag", wantPath: "/api/v3/x?SENTINEL=1#frag", wantAuth: []string{testAuth},
		},
		{
			name: "empty authorization is omitted", path: testPath, body: testBody,
			wantURL: testURL, wantPath: testPath,
		},
		{
			name: "empty body", path: testPath, auth: testAuth,
			wantURL: testURL, wantPath: testPath, wantAuth: []string{testAuth},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The in-memory transport runs on the calling goroutine.
			var seen seenRequest
			hc, ct := fakeClient(func(r *http.Request) (*http.Response, error) {
				noBody := r.Body == http.NoBody
				rec := testutil.Record(r)
				seen = seenRequest{
					method:        rec.Method,
					url:           rec.URL,
					path:          r.URL.Path,
					rawQuery:      r.URL.RawQuery,
					host:          r.Host,
					header:        rec.Header,
					body:          rec.Body,
					contentLength: r.ContentLength,
					noBody:        noBody,
					getBody:       r.GetBody != nil,
				}
				return response(r, http.StatusOK, newBody(okBody), int64(len(okBody))), nil
			})
			cfg := baseConfig()
			cfg.Authorization = tt.auth

			resp, f := httpx.Do(t.Context(), hc, cfg, tt.path, []byte(tt.body))
			require.Nil(t, f)
			assert.JSONEq(t, okBody, string(resp.Body))
			assert.Equal(t, 1, ct.Attempts())

			assert.Equal(t, http.MethodPost, seen.method)
			assert.Equal(t, tt.wantURL, seen.url)
			assert.Equal(t, tt.wantPath, seen.path)
			assert.Empty(t, seen.rawQuery)
			assert.Equal(t, testHost, seen.host)
			assert.Equal(t, []byte(tt.body), seen.body)
			assert.Equal(t, int64(len(tt.body)), seen.contentLength)
			assert.False(t, seen.getBody, "the request must not be rewindable")
			assert.False(t, seen.noBody, "http.NoBody may be resent by HTTP/2, so even an empty body is not NoBody")

			want := http.Header{
				"Content-Type": {"application/json"},
				"Accept":       {"application/json"},
				"User-Agent":   {testUserAgent},
			}
			if tt.wantAuth != nil {
				want["Authorization"] = tt.wantAuth
			}
			assert.Equal(t, want, seen.header, "exactly these headers: no Accept-Encoding, no Idempotency-Key")
		})
	}
}

func TestDoSendsExactRequestOverTheWire(t *testing.T) {
	for _, proto := range testutil.Protocols() {
		for _, body := range []string{testBody, ""} {
			t.Run(proto.Name()+" body "+strconv.Itoa(len(body)), func(t *testing.T) {
				srv := testutil.NewServerProto(t, proto, func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, okBody)
				})
				resp, f := httpx.Do(t.Context(), httpx.NewClient(srv.Client()), serverConfig(t, srv), testPath, []byte(body))
				require.Nil(t, f)
				assert.JSONEq(t, okBody, string(resp.Body))
				assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

				got := srv.Requests()
				require.Len(t, got, 1)
				r := got[0]
				assert.Equal(t, string(proto), r.Proto)
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, testPath, r.URL)
				assert.Equal(t, body, string(r.Body))
				assert.Equal(t, []string{"application/json"}, r.Header.Values("Content-Type"))
				assert.Equal(t, []string{"application/json"}, r.Header.Values("Accept"))
				assert.Equal(t, []string{testUserAgent}, r.Header.Values("User-Agent"))
				assert.Equal(t, []string{testAuth}, r.Header.Values("Authorization"))
				assert.Empty(t, r.Header.Values("Idempotency-Key"))
				assert.Empty(t, r.Header.Values("X-Idempotency-Key"))
				assert.Empty(t, r.Header.Values("Cookie"))
				if body != "" {
					assert.Equal(t, []string{strconv.Itoa(len(body))}, r.Header.Values("Content-Length"))
				}
			})
		}
	}
}

func TestDoRequestHeadersAreIndependent(t *testing.T) {
	// Each call gets its own header values, and growing one value cannot
	// overwrite the next.
	var (
		accepts []*string
		after   []http.Header
	)
	hc, ct := fakeClient(func(r *http.Request) (*http.Response, error) {
		accepts = append(accepts, &r.Header["Accept"][0])
		r.Header["Content-Type"] = append(r.Header["Content-Type"], "text/plain")
		after = append(after, r.Header.Clone())
		return response(r, http.StatusOK, newBody(`{}`), 2), nil
	})
	for range 2 {
		_, f := httpx.Do(t.Context(), hc, baseConfig(), testPath, []byte(testBody))
		require.Nil(t, f)
	}
	assert.Equal(t, 2, ct.Attempts())
	require.Len(t, after, 2)
	for _, h := range after {
		assert.Equal(t, []string{"application/json", "text/plain"}, h.Values("Content-Type"))
		assert.Equal(t, []string{"application/json"}, h.Values("Accept"))
		assert.Equal(t, []string{testUserAgent}, h.Values("User-Agent"))
		assert.Equal(t, []string{testAuth}, h.Values("Authorization"))
	}
	assert.NotSame(t, accepts[0], accepts[1])
}

func TestDoMakesNoAttemptWhenContextIsDone(t *testing.T) {
	tests := []struct {
		name     string
		ctx      func(t *testing.T) context.Context
		timeout  time.Duration
		wantKind httpx.FailureKind
	}{
		{
			name: "caller canceled",
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			timeout:  5 * time.Second,
			wantKind: httpx.FailureCanceled,
		},
		{
			name: "caller deadline passed",
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
				t.Cleanup(cancel)
				return ctx
			},
			timeout:  5 * time.Second,
			wantKind: httpx.FailureDeadlineExceeded,
		},
		{
			name: "caller canceled with a cause",
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancelCause(t.Context())
				cancel(errReset)
				return ctx
			},
			timeout:  5 * time.Second,
			wantKind: httpx.FailureCanceled,
		},
		{
			name:     "zero call timeout",
			ctx:      func(t *testing.T) context.Context { return t.Context() },
			wantKind: httpx.FailureTimeout,
		},
		{
			name:     "negative call timeout",
			ctx:      func(t *testing.T) context.Context { return t.Context() },
			timeout:  -time.Second,
			wantKind: httpx.FailureTimeout,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc, ct := fakeClient(func(r *http.Request) (*http.Response, error) {
				return response(r, http.StatusOK, newBody(okBody), -1), nil
			})
			cfg := baseConfig()
			cfg.Timeout = tt.timeout
			resp, f := httpx.Do(tt.ctx(t), hc, cfg, testPath, []byte(testBody))
			f = requireFailure(t, resp, f, tt.wantKind)
			assert.Zero(t, f.Status)
			assert.Nil(t, f.Header)
			assert.Nil(t, f.BodyExcerpt)
			assert.Zero(t, ct.Attempts(), "no attempt once the context is done")
		})
	}
}

func TestDoRequestBuildFailure(t *testing.T) {
	for _, host := range []string{"example.test\x7f", "exa mple.test", "example.test:port"} {
		t.Run(strconv.Quote(host), func(t *testing.T) {
			hc, ct := fakeClient(func(r *http.Request) (*http.Response, error) {
				return response(r, http.StatusOK, newBody(okBody), -1), nil
			})
			cfg := baseConfig()
			cfg.Host = host
			resp, f := httpx.Do(t.Context(), hc, cfg, testPath, []byte(testBody))
			f = requireFailure(t, resp, f, httpx.FailureTransport)
			assert.Equal(t, httpx.Failure{Kind: httpx.FailureTransport}, *f)
			assert.Zero(t, ct.Attempts())
		})
	}
}

func TestDoStatusIsAFailure(t *testing.T) {
	statuses := []int{
		http.StatusCreated, http.StatusNoContent, http.StatusPartialContent,
		http.StatusMovedPermanently, http.StatusNotModified,
		http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound,
		http.StatusMethodNotAllowed, http.StatusGone, http.StatusTeapot, http.StatusUnprocessableEntity,
		http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout,
	}
	now := time.Now()
	for _, status := range statuses {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			body := fmt.Sprintf(`{"status":{"code":%d,"text":"SENTINEL-STATUS"}}`, status)
			if status == http.StatusBadGateway {
				body = "<html>SENTINEL bad gateway, not JSON</html>"
			}
			bodyAllowed := status != http.StatusNoContent && status != http.StatusNotModified
			srv := testutil.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Sentinel", "SENTINEL-HEADER")
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(status)
				if bodyAllowed {
					_, _ = io.WriteString(w, body)
				}
			})
			resp, f := httpx.Do(t.Context(), httpx.NewClient(srv.Client()), serverConfig(t, srv), testPath, []byte(testBody))
			f = requireFailure(t, resp, f, httpx.FailureStatus)
			assert.Equal(t, status, f.Status)
			assert.Equal(t, "SENTINEL-HEADER", f.Header.Get("X-Sentinel"))
			hint, ok := httpx.RetryAfter(f.Header, now)
			assert.True(t, ok)
			assert.Equal(t, 7*time.Second, hint)
			require.NotNil(t, f.BodyExcerpt, "a body read completely is kept, even when empty")
			if bodyAllowed {
				assert.Equal(t, body, string(f.BodyExcerpt))
			} else {
				assert.Empty(t, f.BodyExcerpt)
			}
			assert.Equal(t, 1, srv.Hits())
		})
	}
}

func TestDoStatusExcerpt(t *testing.T) {
	small := `{"status":{"code":400,"text":"SENTINEL-BAD-REQUEST"}}`
	atCap := strings.Repeat("e", httpx.MaxErrorBodyBytes)
	overCap := atCap + "e"
	tests := []struct {
		name          string
		body          func() *trackingBody
		contentLength int64
		want          *string // nil: no excerpt
		wantMaxRead   int64
	}{
		{name: "small body kept", body: func() *trackingBody { return newBody(small) }, contentLength: -1, want: &small},
		{name: "small body with length kept", body: func() *trackingBody { return newBody(small) }, contentLength: int64(len(small)), want: &small},
		{name: "not json kept raw", body: func() *trackingBody { return newBody("SENTINEL\x00\xff raw") }, contentLength: -1, want: new("SENTINEL\x00\xff raw")},
		{name: "empty body kept empty", body: func() *trackingBody { return newBody("") }, contentLength: 0, want: new("")},
		{name: "exactly the cap kept", body: func() *trackingBody { return newBody(atCap) }, contentLength: -1, want: &atCap},
		{name: "exactly the cap with length kept", body: func() *trackingBody { return newBody(atCap) }, contentLength: int64(len(atCap)), want: &atCap},
		{name: "one over the cap dropped", body: func() *trackingBody { return newBody(overCap) }, contentLength: -1, wantMaxRead: int64(len(overCap))},
		{name: "declared over the cap dropped unread", body: func() *trackingBody { return newBody(overCap) }, contentLength: int64(len(overCap))},
		{name: "understated length over the cap dropped", body: func() *trackingBody { return newBody(overCap) }, contentLength: 3, wantMaxRead: int64(len(overCap))},
		{name: "overstated length yields what was read", body: func() *trackingBody { return newBody(small) }, contentLength: 900, want: &small},
		{name: "read error dropped", body: func() *trackingBody {
			return &trackingBody{r: strings.NewReader(`{"status":`), readErr: errReset}
		}, contentLength: -1, wantMaxRead: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := tt.body()
			hc, ct := fakeClient(func(r *http.Request) (*http.Response, error) {
				return response(r, http.StatusBadRequest, body, tt.contentLength), nil
			})
			resp, f := httpx.Do(t.Context(), hc, baseConfig(), testPath, []byte(testBody))
			f = requireFailure(t, resp, f, httpx.FailureStatus)
			assert.Equal(t, http.StatusBadRequest, f.Status)
			assert.Equal(t, "SENTINEL-RESPONSE", f.Header.Get("X-Sentinel"))
			if tt.want == nil {
				assert.Nil(t, f.BodyExcerpt)
				assert.LessOrEqual(t, body.bytes.Load(), tt.wantMaxRead)
			} else {
				require.NotNil(t, f.BodyExcerpt)
				assert.Equal(t, *tt.want, string(f.BodyExcerpt))
			}
			assert.Equal(t, int32(1), body.closes.Load())
			assert.Equal(t, 1, ct.Attempts())
		})
	}
}

func TestDoStatusExcerptOfAnEndlessBody(t *testing.T) {
	body := &endlessBody{}
	hc, ct := fakeClient(func(r *http.Request) (*http.Response, error) {
		return response(r, http.StatusServiceUnavailable, body, -1), nil
	})
	resp, f := httpx.Do(t.Context(), hc, baseConfig(), testPath, []byte(testBody))
	f = requireFailure(t, resp, f, httpx.FailureStatus)
	assert.Equal(t, http.StatusServiceUnavailable, f.Status)
	assert.Nil(t, f.BodyExcerpt)
	assert.Equal(t, int64(httpx.MaxErrorBodyBytes+1), body.read.Load())
	assert.Equal(t, int32(1), body.closes.Load())
	assert.Equal(t, 1, ct.Attempts())
}

// errRedirectPolicy is the result of an injected client's own redirect
// policy.
var errRedirectPolicy = errors.New("SENTINEL custom redirect policy")

func TestNewClientCopiesWithoutMutating(t *testing.T) {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	ct := &testutil.CountingTransport{Next: testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		resp := response(r, http.StatusOK, newBody(okBody), -1)
		resp.Header.Set("Set-Cookie", "sentinel=SENTINEL-COOKIE; Path=/")
		return resp, nil
	})}
	original := &http.Client{
		Transport:     ct,
		Jar:           jar,
		Timeout:       7 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errRedirectPolicy },
	}

	c := httpx.NewClient(original)
	require.NotSame(t, original, c)
	_, f := httpx.Do(t.Context(), c, baseConfig(), testPath, []byte(testBody))
	require.Nil(t, f)
	assert.Equal(t, 1, ct.Attempts())

	// The original is untouched.
	assert.Same(t, ct, original.Transport)
	assert.Same(t, jar, original.Jar)
	assert.Equal(t, 7*time.Second, original.Timeout)
	require.NotNil(t, original.CheckRedirect)
	require.ErrorIs(t, original.CheckRedirect(nil, nil), errRedirectPolicy)
	u, err := url.Parse(testURL)
	require.NoError(t, err)
	assert.Empty(t, jar.Cookies(u), "the original jar is never used")

	// The copy shares the transport and the timeout, without jar or redirects.
	assert.Same(t, ct, c.Transport)
	assert.Equal(t, 7*time.Second, c.Timeout)
	assert.Nil(t, c.Jar)
	require.NotNil(t, c.CheckRedirect)
	require.ErrorIs(t, c.CheckRedirect(nil, nil), http.ErrUseLastResponse)
}

func TestNewClientKeepsAnInjectedTransportUnchanged(t *testing.T) {
	tr := &http.Transport{MaxResponseHeaderBytes: 123, DisableCompression: true, MaxIdleConns: 7}
	original := &http.Client{Transport: tr}
	c := httpx.NewClient(original)
	assert.Same(t, tr, c.Transport)
	assert.Same(t, tr, original.Transport)
	assert.Nil(t, original.CheckRedirect)
	assert.Nil(t, original.Jar)
	assert.Zero(t, original.Timeout)
	assert.Equal(t, int64(123), tr.MaxResponseHeaderBytes)
	assert.True(t, tr.DisableCompression)
	assert.Equal(t, 7, tr.MaxIdleConns)
	assert.Nil(t, tr.Proxy)
}

func TestNewClientNil(t *testing.T) {
	c := httpx.NewClient(nil)
	require.NotNil(t, c)
	assert.Nil(t, c.Jar)
	assert.Zero(t, c.Timeout)
	require.NotNil(t, c.CheckRedirect)
	require.ErrorIs(t, c.CheckRedirect(nil, nil), http.ErrUseLastResponse)

	tr, ok := c.Transport.(*http.Transport)
	require.True(t, ok, "NewClient(nil) builds an *http.Transport")
	assert.NotSame(t, http.DefaultTransport, tr)
	other, ok := httpx.NewClient(nil).Transport.(*http.Transport)
	require.True(t, ok)
	assert.NotSame(t, tr, other, "every client gets its own transport")

	require.NotNil(t, tr.Proxy)
	assert.Equal(t, reflect.ValueOf(http.ProxyFromEnvironment).Pointer(), reflect.ValueOf(tr.Proxy).Pointer(),
		"the proxy variables apply")
	assert.NotNil(t, tr.DialContext)
	assert.Equal(t, int64(httpx.MaxResponseHeaderBytes), tr.MaxResponseHeaderBytes)
	assert.Equal(t, int64(64<<10), tr.MaxResponseHeaderBytes)
	assert.True(t, tr.ForceAttemptHTTP2)
	assert.False(t, tr.DisableCompression, "the transport may request and decode gzip")
	assert.Equal(t, 100, tr.MaxIdleConns)
	assert.Equal(t, 90*time.Second, tr.IdleConnTimeout)
	assert.Equal(t, 10*time.Second, tr.TLSHandshakeTimeout)
	assert.Equal(t, time.Second, tr.ExpectContinueTimeout)
}

func TestNewClientRefusesEveryRedirect(t *testing.T) {
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, testURL, nil)
	require.NoError(t, err)
	for _, hc := range []*http.Client{
		nil,
		{},
		{CheckRedirect: func(*http.Request, []*http.Request) error { return nil }},
	} {
		c := httpx.NewClient(hc)
		require.NotNil(t, c.CheckRedirect)
		require.ErrorIs(t, c.CheckRedirect(req, []*http.Request{req}), http.ErrUseLastResponse)
	}
}

func TestDoNeverSendsCookies(t *testing.T) {
	tests := []struct {
		name        string
		sdk         bool
		wantCookies bool
	}{
		{name: "sdk client drops the jar", sdk: true},
		{name: "raw client with the jar sends the cookie", wantCookies: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := testutil.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
				http.SetCookie(w, &http.Cookie{
					Name: "sentinel", Value: "SENTINEL-COOKIE", Path: "/",
					Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
				})
				_, _ = io.WriteString(w, okBody)
			})
			jar, err := cookiejar.New(nil)
			require.NoError(t, err)
			injected := srv.Client()
			injected.Jar = jar
			for range 2 {
				if tt.sdk {
					_, f := httpx.Do(t.Context(), httpx.NewClient(injected), serverConfig(t, srv), testPath, []byte(testBody))
					require.Nil(t, f)
					continue
				}
				req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL()+testPath, strings.NewReader(testBody))
				require.NoError(t, err)
				resp, err := injected.Do(req)
				require.NoError(t, err)
				_, _ = io.Copy(io.Discard, resp.Body)
				require.NoError(t, resp.Body.Close())
			}
			got := srv.Requests()
			require.Len(t, got, 2)
			assert.Empty(t, got[0].Header.Values("Cookie"))
			if tt.wantCookies {
				assert.Equal(t, []string{"sentinel=SENTINEL-COOKIE"}, got[1].Header.Values("Cookie"))
			} else {
				assert.Empty(t, got[1].Header.Values("Cookie"))
			}
		})
	}
}

func TestDoClosesBodyOnEveryPath(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          *trackingBody
		contentLength int64
		encoding      string
		wantKind      httpx.FailureKind // 0: success
		wantReads     bool
	}{
		{name: "success", status: http.StatusOK, body: newBody(okBody), contentLength: -1, wantReads: true},
		{name: "success with length", status: http.StatusOK, body: newBody(okBody), contentLength: int64(len(okBody)), wantReads: true},
		{name: "close error after a full read", status: http.StatusOK, body: &trackingBody{r: strings.NewReader(okBody), closeErr: errReset}, contentLength: -1, wantReads: true},
		{name: "status", status: http.StatusTooManyRequests, body: newBody(`{"error":"SENTINEL slow down"}`), contentLength: -1, wantKind: httpx.FailureStatus, wantReads: true},
		{name: "status with an oversized length", status: http.StatusInternalServerError, body: newBody(`{}`), contentLength: 1 << 40, wantKind: httpx.FailureStatus},
		{name: "redirect", status: http.StatusFound, body: newBody("moved"), contentLength: 5, wantKind: httpx.FailureStatus, wantReads: true},
		{name: "too large by length", status: http.StatusOK, body: newBody(`{}`), contentLength: 1 << 40, wantKind: httpx.FailureTooLarge},
		{name: "too large while reading", status: http.StatusOK, body: newBody(strings.Repeat("a", 1025)), contentLength: -1, wantKind: httpx.FailureTooLarge, wantReads: true},
		{name: "encoding", status: http.StatusOK, body: newBody("\x1f\x8b"), contentLength: 2, encoding: "gzip", wantKind: httpx.FailureEncoding},
		{name: "read error", status: http.StatusOK, body: &trackingBody{r: strings.NewReader(`{"res`), readErr: errReset}, contentLength: -1, wantKind: httpx.FailureTransport, wantReads: true},
		{name: "read error with length", status: http.StatusOK, body: &trackingBody{r: strings.NewReader(`{"res`), readErr: errReset}, contentLength: 14, wantKind: httpx.FailureTransport, wantReads: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc, ct := fakeClient(func(r *http.Request) (*http.Response, error) {
				resp := response(r, tt.status, tt.body, tt.contentLength)
				if tt.encoding != "" {
					resp.Header.Set("Content-Encoding", tt.encoding)
				}
				return resp, nil
			})
			resp, f := httpx.Do(t.Context(), hc, baseConfig(), testPath, []byte(testBody))
			assert.Equal(t, int32(1), tt.body.closes.Load(), "the body is closed exactly once")
			assert.Equal(t, 1, ct.Attempts())
			assert.Equal(t, tt.wantReads, tt.body.reads.Load() > 0)
			if tt.wantKind == 0 {
				require.Nil(t, f)
				assert.Equal(t, "SENTINEL-RESPONSE", resp.Header.Get("X-Sentinel"))
				return
			}
			f = requireFailure(t, resp, f, tt.wantKind)
			assert.Equal(t, tt.status, f.Status)
			assert.Equal(t, "SENTINEL-RESPONSE", f.Header.Get("X-Sentinel"))
		})
	}
}

func TestDoTransportWithoutAResponse(t *testing.T) {
	tests := []struct {
		name     string
		resp     func(r *http.Request) (*http.Response, error)
		wantKind httpx.FailureKind // 0: success
	}{
		{
			name:     "nil response and nil error",
			resp:     func(*http.Request) (*http.Response, error) { return nil, nil }, //nolint:nilnil // a broken transport is the case under test
			wantKind: httpx.FailureTransport,
		},
		{
			name:     "error",
			resp:     func(*http.Request) (*http.Response, error) { return nil, errReset },
			wantKind: httpx.FailureTransport,
		},
		{
			name: "nil body with a positive length",
			resp: func(r *http.Request) (*http.Response, error) {
				return response(r, http.StatusOK, nil, 5), nil
			},
			wantKind: httpx.FailureTransport,
		},
		{
			name: "nil body with zero length reads as empty",
			resp: func(r *http.Request) (*http.Response, error) {
				return response(r, http.StatusOK, nil, 0), nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc, ct := fakeClient(tt.resp)
			resp, f := httpx.Do(t.Context(), hc, baseConfig(), testPath, []byte(testBody))
			assert.Equal(t, 1, ct.Attempts())
			if tt.wantKind == 0 {
				require.Nil(t, f)
				assert.Empty(t, resp.Body)
				return
			}
			f = requireFailure(t, resp, f, tt.wantKind)
			assert.Equal(t, httpx.Failure{Kind: tt.wantKind}, *f)
		})
	}
}

// TestFailureHasNoCause pins the shape of Failure: no field can hold an
// error, so no cause can ever be printed.
func TestFailureHasNoCause(t *testing.T) {
	typ := reflect.TypeFor[httpx.Failure]()
	names := make([]string, 0, typ.NumField())
	for i := range typ.NumField() {
		field := typ.Field(i)
		names = append(names, field.Name)
		assert.NotEqual(t, reflect.Interface, field.Type.Kind(), "field %s must not hold an interface", field.Name)
	}
	assert.Equal(t, []string{"Kind", "Status", "Header", "BodyExcerpt"}, names)
}

// TestDoFailurePrintsNoSecrets proves that no failure, however it is
// printed, reveals the request's credentials, URL, proxy userinfo or the
// transport error.
func TestDoFailurePrintsNoSecrets(t *testing.T) {
	const (
		transportSecret = "SENTINEL-TRANSPORT-ERR"
		proxyPass       = "SENTINEL-PROXY-PASS"
		proxyURL        = "https://user:" + proxyPass + "@127.0.0.1"
	)
	secrets := []string{testKey, transportSecret, proxyPass, proxyURL, testHost, testPath, "SENTINEL-UA"}
	tests := []struct {
		name     string
		client   func(t *testing.T) *http.Client
		wantKind httpx.FailureKind
	}{
		{
			name: "transport error quoting a proxy",
			client: func(*testing.T) *http.Client {
				hc, _ := fakeClient(func(*http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("proxyconnect tcp %s: %s", proxyURL, transportSecret) //nolint:err113 // a dynamic error is the case under test
				})
				return hc
			},
			wantKind: httpx.FailureTransport,
		},
		{
			name: "url error quoting the request",
			client: func(*testing.T) *http.Client {
				hc, _ := fakeClient(func(r *http.Request) (*http.Response, error) {
					return nil, &url.Error{Op: "Post " + r.Header.Get("Authorization"), URL: r.URL.String(), Err: errors.New(transportSecret)} //nolint:err113 // a dynamic error is the case under test
				})
				return hc
			},
			wantKind: httpx.FailureTransport,
		},
		{
			name: "unreachable proxy with userinfo",
			client: func(t *testing.T) *http.Client {
				proxy, err := url.Parse("http://user:" + proxyPass + "@127.0.0.1:1")
				require.NoError(t, err)
				tr := &http.Transport{Proxy: http.ProxyURL(proxy)}
				t.Cleanup(tr.CloseIdleConnections)
				return httpx.NewClient(&http.Client{Transport: tr})
			},
			wantKind: httpx.FailureTransport,
		},
		{
			name: "status",
			client: func(*testing.T) *http.Client {
				hc, _ := fakeClient(func(r *http.Request) (*http.Response, error) {
					return response(r, http.StatusUnauthorized, newBody(`{"error":"SENTINEL-BODY"}`), -1), nil
				})
				return hc
			},
			wantKind: httpx.FailureStatus,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, f := httpx.Do(t.Context(), tt.client(t), baseConfig(), testPath, []byte(testBody))
			f = requireFailure(t, resp, f, tt.wantKind)
			testutil.AssertShapesNeverPrint(t, f, secrets...)
			testutil.AssertDiffRedacted(t, f, &httpx.Failure{Kind: httpx.FailureEncoding, Status: 599}, secrets...)
		})
	}
}
