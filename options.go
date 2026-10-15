package batchdata

import (
	"net/http"
	"time"

	"github.com/mrz1836/go-batchdata/internal/httpx"
)

// Option configures a Client in NewClient. Nil options are skipped, options
// apply in order, and the last option of each kind wins. Options only record
// their argument; NewClient validates the result and reports an invalid value
// as ErrInvalidConfig.
type Option func(*config)

// config is the Client configuration options build.
type config struct {
	httpClient    *http.Client
	hasHTTPClient bool
	timeout       time.Duration
	maxBytes      int64
	userAgent     string
	endpoint      string
	hasEndpoint   bool
}

// check validates the options in a fixed order and returns the API host:
// production, or the loopback host of WithTestEndpoint. Errors match
// ErrInvalidConfig, name the option, and never quote the rejected value.
func (c *config) check() (string, error) {
	switch {
	case c.hasHTTPClient && c.httpClient == nil:
		return "", invalid(ErrInvalidConfig, "WithHTTPClient", "must not be nil")
	case c.timeout <= 0:
		return "", invalid(ErrInvalidConfig, "WithTimeout", "must be positive")
	case c.maxBytes < 1 || c.maxBytes > MaxResponseBytesLimit:
		return "", invalid(ErrInvalidConfig, "WithMaxResponseBytes", "must be from 1 byte to 64 MiB")
	case !httpx.ValidUserAgent(c.userAgent):
		return "", invalid(ErrInvalidConfig, "WithUserAgent",
			"must be 1 to 256 bytes of visible ASCII or spaces, starting and ending with a visible character")
	case !c.hasEndpoint:
		return productionHost, nil
	}
	host, ok := httpx.TestEndpoint(c.endpoint)
	if !ok {
		return "", invalid(ErrInvalidConfig, "WithTestEndpoint",
			"must be an https URL on 127.0.0.0/8 or [::1] with an optional port and no path, query, fragment or userinfo")
	}
	return host, nil
}

// WithHTTPClient sets the HTTP client that carries every call. NewClient
// copies it with redirects disabled and no cookie jar, and never modifies hc:
// its Transport is shared and never mutated, and its Timeout still applies.
// Changes to hc after NewClient have no effect. hc must not be nil.
//
// The transport is trusted caller code. A transport that retries, follows
// redirects or replays requests on its own defeats the SDK's one-attempt
// guarantee. If hc.Transport is nil, Go uses http.DefaultTransport: changes
// to it then apply, and the SDK's 64 KiB response header limit does not.
//
// Without this option the SDK builds its own transport with the settings of
// http.DefaultTransport, so later changes to http.DefaultTransport have no
// effect on it. Like http.DefaultTransport, it takes a proxy from the
// HTTPS_PROXY and NO_PROXY environment variables; every call is https, so
// HTTP_PROXY is never used.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *config) { c.httpClient, c.hasHTTPClient = hc, true }
}

// WithTimeout bounds each call, including sending the request and reading the
// whole response body. The caller's context deadline still applies when it is
// earlier. The default is DefaultTimeout; d must be positive.
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// WithMaxResponseBytes limits the response body size. A larger body fails the
// call with KindResponseTooLarge, which matches ErrMalformedResponse, after the
// provider may already have billed it. The default is DefaultMaxResponseBytes;
// n must be from 1 to MaxResponseBytesLimit.
func WithMaxResponseBytes(n int64) Option {
	return func(c *config) { c.maxBytes = n }
}

// WithUserAgent replaces the User-Agent header, "go-batchdata" by default. ua
// must be 1 to 256 bytes of visible ASCII or spaces, starting and ending with
// a visible character.
func WithUserAgent(ua string) Option {
	return func(c *config) { c.userAgent = ua }
}

// WithTestEndpoint replaces https://api.batchdata.com with a loopback base URL,
// such as the URL of an httptest.NewTLSServer. It exists for tests: only https
// on 127.0.0.0/8 or [::1], with an optional port and no path, query, fragment
// or userinfo, is accepted. Host names, including "localhost", are rejected
// because they resolve through the system resolver. Requests go to
// <base>/api/v3/<endpoint path>.
func WithTestEndpoint(baseURL string) Option {
	return func(c *config) { c.endpoint, c.hasEndpoint = baseURL, true }
}
