package httpx_test

import (
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/httpx"
)

// FuzzTestEndpoint checks that an accepted endpoint names a loopback
// address, and that its host validates again on its own.
func FuzzTestEndpoint(f *testing.F) {
	for _, seed := range []string{
		"https://127.0.0.1", "https://127.0.0.1:8443/", "https://[::1]:443", "https://127.1.2.3:1",
		"http://127.0.0.1", "https://localhost", "https://[::ffff:127.0.0.1]", "https://127.000.0.1",
		"https://127.0.0.1:08443", "https://user:SENTINEL-PASS@127.0.0.1", "https://127.0.0.1\\@example.test",
		"https://127.0.0.1/?#", "HTTPS://[::1]", "https://[::1%25lo0]", "",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		host, ok := httpx.TestEndpoint(raw)
		if !ok {
			assert.Empty(t, host)
			return
		}
		require.NotEmpty(t, host)
		assert.False(t, strings.ContainsAny(host, "@/?#\\ "), "host %q", host)

		addr, err := netip.ParseAddr((&url.URL{Host: host}).Hostname())
		require.NoError(t, err, "host %q", host)
		assert.True(t, addr.IsLoopback(), "host %q", host)
		assert.True(t, addr.Is4() || addr == netip.IPv6Loopback(), "host %q", host)

		again, ok := httpx.TestEndpoint("https://" + host)
		assert.True(t, ok, "host %q validates again", host)
		assert.Equal(t, host, again)
	})
}

// FuzzRetryAfter checks that any header value yields a hint within
// [0, MaxRetryAfter], and no hint at all when it is unusable.
func FuzzRetryAfter(f *testing.F) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	for _, seed := range []string{
		"", "0", "120", "-5", "1.5", strings.Repeat("9", 40), "SENTINEL",
		now.Add(time.Minute).Format(http.TimeFormat), "Fri, 31 Dec 9999 23:59:59 GMT",
		"Mon, 01 Jan 0001 00:00:00 GMT", now.Format(time.RFC850), now.Format(time.ANSIC),
	} {
		f.Add(seed, int64(0))
	}
	f.Add("120", int64(-1<<62))
	f.Add("Fri, 31 Dec 9999 23:59:59 GMT", int64(1<<62))
	f.Fuzz(func(t *testing.T, value string, offset int64) {
		h := http.Header{"Retry-After": {value}}
		d, ok := httpx.RetryAfter(h, now.Add(time.Duration(offset)))
		assert.GreaterOrEqual(t, d, time.Duration(0))
		assert.LessOrEqual(t, d, httpx.MaxRetryAfter)
		if !ok {
			assert.Zero(t, d)
		}
	})
}
