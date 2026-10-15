package httpx_test

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/httpx"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// targetPath is where every redirect of the suite points.
const targetPath = "/api/v3/SENTINEL-TARGET"

// Where a redirect points.
const (
	sameOriginRelative = iota
	sameOriginAbsolute
	crossOrigin
)

// redirectPair is an origin that redirects testPath to a target, which is the
// origin itself or a second server.
type redirectPair struct {
	origin   *testutil.Server
	target   *testutil.Server
	location atomic.Pointer[string]
}

// newRedirectPair starts the servers of one redirect case. Each answers
// targetPath with 200 and redirects anything else.
func newRedirectPair(t *testing.T, proto testutil.Protocol, status, kind int) *redirectPair {
	t.Helper()
	p := &redirectPair{}
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == targetPath {
			_, _ = io.WriteString(w, okBody)
			return
		}
		http.Redirect(w, r, *p.location.Load(), status)
	}
	p.origin = testutil.NewServerProto(t, proto, handler)
	p.target = p.origin
	location := targetPath
	switch kind {
	case sameOriginAbsolute:
		location = p.origin.URL() + targetPath
	case crossOrigin:
		p.target = testutil.NewServerProto(t, proto, handler)
		location = p.target.URL() + targetPath
	}
	p.location.Store(&location)
	return p
}

// targetHits counts the requests that reached targetPath on the target.
func (p *redirectPair) targetHits() int {
	n := 0
	for _, r := range p.target.Requests() {
		if r.URL == targetPath {
			n++
		}
	}
	return n
}

// TestDoDoesNotFollowRedirects runs every redirect status, to the same origin
// and to another one, over both protocols. The SDK returns the redirect as a
// FailureStatus and never reaches the target, so no credential can either;
// the raw client control proves the harness would see a followed redirect.
func TestDoDoesNotFollowRedirects(t *testing.T) {
	statuses := []int{
		http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect,
	}
	origins := []struct {
		name string
		kind int
	}{
		{name: "same origin relative", kind: sameOriginRelative},
		{name: "same origin absolute", kind: sameOriginAbsolute},
		{name: "cross origin", kind: crossOrigin},
	}
	for _, proto := range testutil.Protocols() {
		for _, status := range statuses {
			for _, origin := range origins {
				name := proto.Name() + " " + strconv.Itoa(status) + " " + origin.name
				t.Run(name+" sdk", func(t *testing.T) {
					checkRedirectNotFollowed(t, newRedirectPair(t, proto, status, origin.kind), status)
				})
				t.Run(name+" raw client control follows", func(t *testing.T) {
					checkRedirectFollowed(t, newRedirectPair(t, proto, status, origin.kind))
				})
			}
		}
	}
}

// checkRedirectNotFollowed calls p's origin through the SDK and checks that
// the redirect came back as a FailureStatus without reaching the target.
func checkRedirectNotFollowed(t *testing.T, p *redirectPair, status int) {
	t.Helper()
	resp, f := httpx.Do(t.Context(), httpx.NewClient(p.origin.Client()), serverConfig(t, p.origin), testPath, []byte(testBody))
	f = requireFailure(t, resp, f, httpx.FailureStatus)
	assert.Equal(t, status, f.Status)
	assert.Equal(t, *p.location.Load(), f.Header.Get("Location"))
	assert.Zero(t, p.targetHits(), "the redirect is never followed")
	assert.Equal(t, 1, p.origin.Hits())
	if p.target != p.origin {
		assert.Zero(t, p.target.Hits())
	}
}

// checkRedirectFollowed calls p's origin with a raw client, which follows
// the redirect to the target.
func checkRedirectFollowed(t *testing.T, p *redirectPair) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, p.origin.URL()+testPath, strings.NewReader(testBody))
	require.NoError(t, err)
	req.Header.Set("Authorization", testAuth)
	raw := p.origin.Client()
	require.Nil(t, raw.CheckRedirect, "the control client follows redirects")
	resp, err := raw.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, okBody, string(body))
	assert.Equal(t, 1, p.targetHits(), "the control reached the target")
}
