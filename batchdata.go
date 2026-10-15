package batchdata

import (
	"strconv"
	"time"

	"github.com/mrz1836/go-batchdata/internal/call"
	"github.com/mrz1836/go-batchdata/internal/httpx"
)

const (
	// productionHost is the origin of BatchData's API.
	productionHost = "api.batchdata.com"

	// defaultUserAgent is the User-Agent unless WithUserAgent says otherwise.
	defaultUserAgent = "go-batchdata"
)

// Client defaults and limits.
const (
	// DefaultTimeout bounds one call unless WithTimeout says otherwise: dial,
	// send and the full body read.
	DefaultTimeout = 60 * time.Second

	// DefaultMaxResponseBytes is the default response body limit.
	DefaultMaxResponseBytes = 8 << 20

	// MaxResponseBytesLimit is the largest limit WithMaxResponseBytes accepts.
	MaxResponseBytesLimit = 64 << 20

	// MaxRequestBytes bounds an encoded request body. A larger request fails
	// locally with ErrInvalidRequest.
	MaxRequestBytes = 1 << 20
)

// handle is the sealed bridge to service packages. Embedding it under this
// unexported name promotes its unexported method, so a *Client satisfies
// call.Holder without exporting anything.
type handle = call.Handle[Warning]

// Client is an authenticated connection to the BatchData API, shared by the
// service packages such as skiptrace. It is immutable after NewClient and safe
// for concurrent use.
//
// Both methods have value receivers, so printing a Client or a *Client with
// fmt goes through String or GoString. The API key, the URL and the HTTP
// client exist only inside a function the Client holds: fmt prints a func as
// an address with every verb, even through unexported fields, and
// reflection-based dumpers cannot see what a function captures.
//
// The zero Client is unusable: a service built from it, or from a nil
// *Client, reports ErrInvalidConfig on every call and makes none.
type Client struct {
	handle

	desc string
}

// NewClient returns a Client that authenticates with apiKey, sent as a bearer
// token. apiKey must be 1 to 4096 bytes of visible ASCII (no spaces or control
// characters); the SDK never reads it from the environment.
//
// NewClient performs no I/O. An invalid argument or option returns a
// *ValidationError matching ErrInvalidConfig; its message never repeats the
// rejected value.
//
// Create one Client per token and share it. Without WithHTTPClient, each
// Client owns a connection pool that cannot be closed; idle connections
// close on their own after 90 seconds. An application that holds many tokens
// should pass the same *http.Client to every Client with WithHTTPClient, so
// that they share one pool.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	if !httpx.ValidCredential(apiKey) {
		return nil, invalid(ErrInvalidConfig, "apiKey", "must be 1 to 4096 bytes of visible ASCII, without spaces or control characters")
	}
	cfg := config{timeout: DefaultTimeout, maxBytes: DefaultMaxResponseBytes, userAgent: defaultUserAgent}
	for _, o := range opts {
		if o != nil {
			o(&cfg)
		}
	}
	host, err := cfg.check()
	if err != nil {
		return nil, err
	}
	send := newSender(httpx.NewClient(cfg.httpClient), httpx.Config{
		Scheme:        "https",
		Host:          host,
		Authorization: "Bearer " + apiKey,
		UserAgent:     cfg.userAgent,
		Timeout:       cfg.timeout,
		MaxBytes:      cfg.maxBytes,
	})
	desc := "batchdata.Client{timeout: " + cfg.timeout.String() +
		", maxResponseBytes: " + strconv.FormatInt(cfg.maxBytes, 10) + "}"
	return &Client{handle: call.NewHandle(send), desc: desc}, nil
}

// String returns a description of the Client's settings, such as
// `batchdata.Client{timeout: 1m0s, maxResponseBytes: 8388608}`. It never
// includes the API key, the URL or the User-Agent.
func (c Client) String() string {
	if c.desc == "" {
		return "batchdata.Client{}"
	}
	return c.desc
}

// GoString returns the same redacted form as String, so %#v cannot reveal the
// API key or the URL either.
func (c Client) GoString() string { return c.String() }
