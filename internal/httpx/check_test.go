package httpx_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/go-batchdata/internal/httpx"
)

func TestTestEndpointAccepts(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "ipv4 loopback", raw: "https://127.0.0.1", want: "127.0.0.1"},
		{name: "ipv4 loopback with port", raw: "https://127.0.0.1:8443", want: "127.0.0.1:8443"},
		{name: "other 127 address with port 1", raw: "https://127.1.2.3:1", want: "127.1.2.3:1"},
		{name: "highest 127 address and port", raw: "https://127.255.255.254:65535", want: "127.255.255.254:65535"},
		{name: "ipv6 loopback", raw: "https://[::1]", want: "[::1]"},
		{name: "ipv6 loopback with port", raw: "https://[::1]:443", want: "[::1]:443"},
		{name: "trailing slash", raw: "https://127.0.0.1:8443/", want: "127.0.0.1:8443"},
		{name: "ipv6 trailing slash", raw: "https://[::1]:443/", want: "[::1]:443"},
		{name: "upper case scheme", raw: "HTTPS://127.0.0.1:1", want: "127.0.0.1:1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, ok := httpx.TestEndpoint(tt.raw)
			assert.True(t, ok)
			assert.Equal(t, tt.want, host)
		})
	}
}

func TestTestEndpointRejects(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "garbage", raw: "SENTINEL"},
		{name: "missing scheme", raw: "://127.0.0.1"},
		{name: "http", raw: "http://127.0.0.1:8443"},
		{name: "other scheme", raw: "ftp://127.0.0.1"},
		{name: "opaque", raw: "https:127.0.0.1"},
		{name: "localhost", raw: "https://localhost"},
		{name: "localhost with port", raw: "https://localhost:8443"},
		{name: "public name", raw: "https://example.test"},
		{name: "unspecified ipv4", raw: "https://0.0.0.0"},
		{name: "private ipv4", raw: "https://10.0.0.1"},
		{name: "just above 127", raw: "https://128.0.0.1"},
		{name: "just below 127", raw: "https://126.255.255.255"},
		{name: "ipv4 mapped loopback", raw: "https://[::ffff:127.0.0.1]"},
		{name: "ipv6 other address", raw: "https://[::2]"},
		{name: "ipv6 unspecified", raw: "https://[::]:9000"},
		{name: "ipv6 long form", raw: "https://[0:0:0:0:0:0:0:1]"},
		{name: "ipv6 zone", raw: "https://[::1%25lo0]"},
		{name: "ipv6 without brackets", raw: "https://::1"},
		{name: "ipv4 in brackets", raw: "https://[127.0.0.1]"},
		{name: "short ipv4", raw: "https://127.1"},
		{name: "non-canonical ipv4", raw: "https://127.000.0.1"},
		{name: "percent-encoded host", raw: "https://127.0.0.%31"},
		{name: "userinfo", raw: "https://user@127.0.0.1"},
		{name: "userinfo with password", raw: "https://user:" + "SENTINEL-PASS" + "@127.0.0.1"},
		{name: "empty userinfo", raw: "https://@127.0.0.1"},
		{name: "query", raw: "https://127.0.0.1?x=1"},
		{name: "empty query", raw: "https://127.0.0.1/?"},
		{name: "fragment", raw: "https://127.0.0.1#frag"},
		{name: "empty fragment", raw: "https://127.0.0.1/#"},
		{name: "path", raw: "https://127.0.0.1/api/v3"},
		{name: "double slash path", raw: "https://127.0.0.1//"},
		{name: "escaped slash path", raw: "https://127.0.0.1/%2F"},
		{name: "empty port", raw: "https://127.0.0.1:"},
		{name: "port 0", raw: "https://127.0.0.1:0"},
		{name: "port 65536", raw: "https://127.0.0.1:65536"},
		{name: "leading zero port", raw: "https://127.0.0.1:08443"},
		{name: "named port", raw: "https://127.0.0.1:https"},
		{name: "ipv6 port 0", raw: "https://[::1]:0"},
		{name: "backslash", raw: "https://127.0.0.1\\@example.test"},
		{name: "backslash path", raw: "https://127.0.0.1\\"},
		{name: "empty host", raw: "https://"},
		{name: "empty host with path", raw: "https:///api"},
		{name: "leading space", raw: " https://127.0.0.1"},
		{name: "trailing newline", raw: "https://127.0.0.1\n"},
		{name: "tab in host", raw: "https://127.0.0.1\t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, ok := httpx.TestEndpoint(tt.raw)
			assert.False(t, ok)
			assert.Empty(t, host)
		})
	}
}

func TestValidCredential(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{name: "key", s: testKey, want: true},
		{name: "visible punctuation", s: "abc!~_.-+/=\"", want: true},
		{name: "single byte", s: "k", want: true},
		{name: "longest", s: strings.Repeat("k", httpx.MaxCredentialBytes), want: true},
		{name: "longest is 4096 bytes", s: strings.Repeat("k", 4096), want: true},
		{name: "empty", s: ""},
		{name: "too long", s: strings.Repeat("k", 4097)},
		{name: "space", s: "SENTINEL KEY"},
		{name: "leading space", s: " " + testKey},
		{name: "trailing space", s: testKey + " "},
		{name: "tab", s: "SENTINEL\tKEY"},
		{name: "carriage return", s: testKey + "\r"},
		{name: "line feed", s: testKey + "\n"},
		{name: "header injection", s: testKey + "\r\nX-Evil: 1"},
		{name: "nul", s: "SENTINEL\x00KEY"},
		{name: "delete", s: "SENTINEL\x7fKEY"},
		{name: "non-ASCII", s: "SENTINEL-KÉY"},
		{name: "high byte", s: "SENTINEL\xffKEY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, httpx.ValidCredential(tt.s))
		})
	}
}

func TestValidUserAgent(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{name: "simple", s: "go-batchdata", want: true},
		{name: "inner spaces", s: "go-batchdata/1.0 (SENTINEL test)", want: true},
		{name: "single byte", s: "u", want: true},
		{name: "longest", s: strings.Repeat("u", httpx.MaxUserAgentBytes), want: true},
		{name: "longest is 256 bytes", s: strings.Repeat("u", 256), want: true},
		{name: "longest with inner spaces", s: "u" + strings.Repeat(" ", 254) + "u", want: true},
		{name: "empty", s: ""},
		{name: "too long", s: strings.Repeat("u", 257)},
		{name: "single space", s: " "},
		{name: "leading space", s: " go-batchdata"},
		{name: "trailing space", s: "go-batchdata "},
		{name: "tab", s: "go\tbatchdata"},
		{name: "carriage return", s: "go-batchdata\r"},
		{name: "line feed", s: "go-batchdata\nX-Evil: 1"},
		{name: "nul", s: "go\x00batchdata"},
		{name: "delete", s: "go\x7fbatchdata"},
		{name: "non-ASCII", s: "go-bätchdata"},
		{name: "high byte", s: "go\xffbatchdata"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, httpx.ValidUserAgent(tt.s))
		})
	}
}

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	date := func(d time.Duration) string { return now.Add(d).Format(http.TimeFormat) }
	tests := []struct {
		name   string
		values []string // nil: no header
		want   time.Duration
		wantOK bool
	}{
		{name: "absent"},
		{name: "empty", values: []string{""}},
		{name: "spaces only", values: []string{"   "}},
		{name: "zero", values: []string{"0"}, want: 0, wantOK: true},
		{name: "seconds", values: []string{"120"}, want: 2 * time.Minute, wantOK: true},
		{name: "seconds with spaces", values: []string{" 7 "}, want: 7 * time.Second, wantOK: true},
		{name: "leading zeros", values: []string{"007"}, want: 7 * time.Second, wantOK: true},
		{name: "negative clamps to zero", values: []string{"-5"}, want: 0, wantOK: true},
		{name: "huge negative clamps to zero", values: []string{"-" + strings.Repeat("9", 40)}, want: 0, wantOK: true},
		{name: "exactly the maximum", values: []string{"86400"}, want: httpx.MaxRetryAfter, wantOK: true},
		{name: "above the maximum clamps", values: []string{"86401"}, want: httpx.MaxRetryAfter, wantOK: true},
		{name: "int64 overflow clamps", values: []string{"99999999999999999999"}, want: httpx.MaxRetryAfter, wantOK: true},
		{name: "very long digits clamp", values: []string{strings.Repeat("9", 400)}, want: httpx.MaxRetryAfter, wantOK: true},
		{name: "fraction dropped", values: []string{"1.5"}},
		{name: "plus sign dropped", values: []string{"+5"}},
		{name: "lone minus dropped", values: []string{"-"}},
		{name: "double minus dropped", values: []string{"--5"}},
		{name: "hex dropped", values: []string{"0x10"}},
		{name: "exponent dropped", values: []string{"1e3"}},
		{name: "unit dropped", values: []string{"5s"}},
		{name: "garbage dropped", values: []string{"SENTINEL-LATER"}},
		{name: "first value wins", values: []string{"3", "9"}, want: 3 * time.Second, wantOK: true},
		{name: "future date", values: []string{date(90 * time.Second)}, want: 90 * time.Second, wantOK: true},
		{name: "date now", values: []string{date(0)}, want: 0, wantOK: true},
		{name: "past date clamps to zero", values: []string{date(-time.Hour)}, want: 0, wantOK: true},
		{name: "far future date clamps", values: []string{date(48 * time.Hour)}, want: httpx.MaxRetryAfter, wantOK: true},
		{name: "very far future date clamps", values: []string{"Fri, 31 Dec 9999 23:59:59 GMT"}, want: httpx.MaxRetryAfter, wantOK: true},
		{name: "rfc 850 date", values: []string{now.Add(time.Minute).Format(time.RFC850)}, want: time.Minute, wantOK: true},
		{name: "ansi c date", values: []string{now.Add(2 * time.Minute).Format(time.ANSIC)}, want: 2 * time.Minute, wantOK: true},
		{name: "date-shaped garbage dropped", values: []string{"Wed, 99 Oct 2026 12:00:00 GMT"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			if tt.values != nil {
				h["Retry-After"] = tt.values
			}
			got, ok := httpx.RetryAfter(h, now)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRetryAfterNilHeader(t *testing.T) {
	got, ok := httpx.RetryAfter(nil, time.Now())
	assert.False(t, ok)
	assert.Zero(t, got)
}
