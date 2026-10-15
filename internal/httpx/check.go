package httpx

import (
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Limits on the values NewClient accepts.
const (
	// MaxCredentialBytes bounds an API key.
	MaxCredentialBytes = 4096

	// MaxUserAgentBytes bounds a User-Agent.
	MaxUserAgentBytes = 256
)

// TestEndpoint validates a loopback base URL for tests and returns its host,
// such as "127.0.0.1:8443" or "[::1]:8443".
//
// Only https is accepted, with a host that is a canonical IPv4 literal in
// 127.0.0.0/8 or exactly [::1], an optional numeric port, and no userinfo,
// query, fragment or path other than "/". Names such as "localhost" are
// rejected: they resolve through the system resolver and could leave the
// machine. ok is false for any other value.
func TestEndpoint(raw string) (host string, ok bool) {
	if raw == "" || strings.ContainsAny(raw, "?#@\\") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	switch {
	case u.Scheme != "https",
		u.Opaque != "", u.User != nil, u.RawQuery != "", u.ForceQuery, u.Fragment != "",
		u.Path != "" && u.Path != "/", u.RawPath != "":
		return "", false
	}
	if !loopbackHostPort(u.Host) {
		return "", false
	}
	return u.Host, true
}

// loopbackHostPort accepts "ip[:port]" for a canonical IPv4 address in
// 127.0.0.0/8, or "[::1][:port]", with a port from 1 to 65535 in canonical
// decimal form.
func loopbackHostPort(hostport string) bool {
	host, port := splitHostPort(hostport)
	if !loopbackHost(host) {
		return false
	}
	if port == "" {
		return true
	}
	digits, ok := strings.CutPrefix(port, ":")
	if !ok {
		return false
	}
	n, err := strconv.ParseUint(digits, 10, 16)
	return err == nil && n != 0 && strconv.FormatUint(n, 10) == digits
}

// splitHostPort splits "host[:port]" or "[host][:port]". The host keeps its
// brackets and the port keeps its colon, so neither part is silently dropped.
func splitHostPort(hostport string) (host, port string) {
	if end := strings.LastIndexByte(hostport, ']'); end >= 0 {
		return hostport[:end+1], hostport[end+1:]
	}
	if i := strings.LastIndexByte(hostport, ':'); i >= 0 {
		return hostport[:i], hostport[i:]
	}
	return hostport, ""
}

// loopbackHost reports whether host is exactly "[::1]" or a canonical IPv4
// address in 127.0.0.0/8.
func loopbackHost(host string) bool {
	if host == "[::1]" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.Is4() && ip.IsLoopback() && ip.String() == host
}

// ValidCredential reports whether s is 1 to MaxCredentialBytes bytes of
// visible ASCII (0x21 to 0x7E): safe in a header, with no spaces or controls.
func ValidCredential(s string) bool {
	return s != "" && visible(s, MaxCredentialBytes, false)
}

// ValidUserAgent reports whether s is 1 to MaxUserAgentBytes bytes of visible
// ASCII or spaces, starting and ending with a visible byte.
func ValidUserAgent(s string) bool {
	return s != "" && s[0] != ' ' && s[len(s)-1] != ' ' && visible(s, MaxUserAgentBytes, true)
}

// visible reports whether s is at most maxLen bytes, each visible ASCII (0x21
// to 0x7E), or a space when space is set.
func visible(s string, maxLen int, space bool) bool {
	if len(s) > maxLen {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if (c < 0x21 || c > 0x7e) && (!space || c != ' ') {
			return false
		}
	}
	return true
}

// RetryAfter parses the standard Retry-After header of h: integer
// delta-seconds or an HTTP-date relative to now. False means no usable hint:
// absent, empty or unparseable. Negative values and past dates clamp to 0, and
// hints above MaxRetryAfter, including counts too large to represent, clamp to
// MaxRetryAfter. An explicit "0" returns (0, true).
func RetryAfter(h http.Header, now time.Time) (time.Duration, bool) {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0, false
	}
	if d, ok := parseSeconds(v); ok {
		return d, true
	}
	t, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	return clampHint(t.Sub(now)), true
}

// parseSeconds parses an optionally negative base-10 count of seconds.
func parseSeconds(s string) (time.Duration, bool) {
	number, negative := strings.CutPrefix(s, "-")
	if !allDigits(number) {
		return 0, false
	}
	if negative {
		return 0, true
	}
	n, err := strconv.ParseInt(number, 10, 64)
	if err != nil || n > int64(MaxRetryAfter/time.Second) {
		// Only overflow can fail here: number is all digits.
		return MaxRetryAfter, true
	}
	return clampHint(time.Duration(n) * time.Second), true
}

// allDigits reports whether s is a non-empty run of ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// clampHint limits a hint to [0, MaxRetryAfter].
func clampHint(d time.Duration) time.Duration {
	return min(max(d, 0), MaxRetryAfter)
}
