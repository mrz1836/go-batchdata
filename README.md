<div align="center">

# 🏠&nbsp;&nbsp;go-batchdata

**A small, typed Go SDK for the BatchData V3 API: synchronous property skip tracing.**

<br/>

<a href="https://github.com/mrz1836/go-batchdata/releases"><img src="https://img.shields.io/github/release-pre/mrz1836/go-batchdata.svg?include_prereleases&style=flat-square&logo=github&color=black" alt="Release"></a>
<a href="https://golang.org/"><img src="https://img.shields.io/github/go-mod/go-version/mrz1836/go-batchdata?style=flat-square&logo=go&color=00ADD8" alt="Go Version"></a>
<a href="https://github.com/mrz1836/go-batchdata/blob/master/LICENSE"><img src="https://img.shields.io/github/license/mrz1836/go-batchdata.svg?style=flat-square&color=blue" alt="License"></a>

<br/>

<table align="center" border="0">
  <tr>
    <td align="right">
       <code>CI / CD</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/mrz1836/go-batchdata/actions"><img src="https://img.shields.io/github/actions/workflow/status/mrz1836/go-batchdata/fortress.yml?branch=master&label=build&logo=github&style=flat-square" alt="Build"></a>
       <a href="https://github.com/mrz1836/go-batchdata/actions"><img src="https://img.shields.io/github/last-commit/mrz1836/go-batchdata?style=flat-square&logo=git&logoColor=white&label=last%20update" alt="Last Commit"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Quality</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://codecov.io/gh/mrz1836/go-batchdata"><img src="https://codecov.io/gh/mrz1836/go-batchdata/branch/master/graph/badge.svg?style=flat-square" alt="Coverage"></a>
    </td>
  </tr>

  <tr>
    <td align="right">
       <code>Security</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://scorecard.dev/viewer/?uri=github.com/mrz1836/go-batchdata"><img src="https://api.scorecard.dev/projects/github.com/mrz1836/go-batchdata/badge?style=flat-square" alt="Scorecard"></a>
       <a href=".github/SECURITY.md"><img src="https://img.shields.io/badge/policy-active-success?style=flat-square&logo=security&logoColor=white" alt="Security"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Community</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/mrz1836/go-batchdata/graphs/contributors"><img src="https://img.shields.io/github/contributors/mrz1836/go-batchdata?style=flat-square&color=orange" alt="Contributors"></a>
       <a href="https://mrz1818.com/"><img src="https://img.shields.io/badge/donate-bitcoin-ff9900?style=flat-square&logo=bitcoin" alt="Bitcoin"></a>
    </td>
  </tr>
</table>

</div>

<br/>
<br/>

<div align="center">

### <code>Project Navigation</code>

</div>

<table align="center">
  <tr>
    <td align="center" width="20%">
       🚀&nbsp;<a href="#-installation"><code>Installation</code></a>
    </td>
    <td align="center" width="20%">
       🏁&nbsp;<a href="#-quick-start"><code>Quick&nbsp;Start</code></a>
    </td>
    <td align="center" width="20%">
       💡&nbsp;<a href="#-usage"><code>Usage</code></a>
    </td>
    <td align="center" width="20%">
       ⚙️&nbsp;<a href="#%EF%B8%8F-behavior"><code>Behavior</code></a>
    </td>
    <td align="center" width="20%">
       📚&nbsp;<a href="#-documentation"><code>Documentation</code></a>
    </td>
  </tr>
  <tr>
    <td align="center">
       🧪&nbsp;<a href="#-examples--tests"><code>Examples&nbsp;&&nbsp;Tests</code></a>
    </td>
    <td align="center">
      ⚡&nbsp;<a href="#-benchmarks"><code>Benchmarks</code></a>
    </td>
    <td align="center">
      🛠️&nbsp;<a href="#%EF%B8%8F-code-standards"><code>Code&nbsp;Standards</code></a>
    </td>
    <td align="center">
      🤖&nbsp;<a href="#-ai-usage--assistant-guidelines"><code>AI&nbsp;Usage</code></a>
    </td>
    <td align="center">
       🤝&nbsp;<a href="#-contributing"><code>Contributing</code></a>
    </td>
  </tr>
</table>
<br/>

go-batchdata calls [BatchData](https://batchdata.com)'s V3 property skip-trace endpoint: you send up to
100 properties, by address or by assessor's parcel number (APN), and get back the persons BatchData links
to each property, with their phones, emails and addresses. Use it when a Go service needs owner contact
data and has to stay in control of every billable call.

- **One attempt per call.** No retries, redirects, replays, chunking or polling: the SDK never repeats a
  billable request on its own.
- **Typed results that keep BatchData's meaning.** An absent flag is `nil`, not `false`; an absent count
  is `nil`, not `0`; unknown enum values are kept as sent.
- **No guessing.** A returned record is linked to your request item only by evidence, never by position,
  name or address.
- **Nothing implicit.** No environment reads for the token, no injected option defaults, no globals.
- **Private by construction.** Errors and printed clients never contain the token, URLs, bodies or
  personal data.
- **Zero runtime dependencies** and bounded memory, time and response size.

Version 1 covers **synchronous forward skip tracing** (`POST /api/v3/property/skip-trace`) only; see
[what is not covered](#what-is-not-covered).

<br/>

## 📦 Installation

**go-batchdata** requires a [supported release of Go](https://golang.org/doc/devel/release.html#policy) (Go 1.26 or newer).
```shell script
go get github.com/mrz1836/go-batchdata
```

The library has **zero third-party runtime dependencies**: the root package and `skiptrace` use only the
standard library. `testify` is used only by the tests and their internal helper package.

<br/>

### Credentials

You need a BatchData API token, sent as a bearer token; see BatchData's guide to
[authentication and tokens](https://help.batchdata.io/en/articles/13238212-set-up-authentication-and-tokens-to-start-using-batchdata-apis).
go-batchdata never reads an environment variable or a configuration file for it: your code passes the
token to `batchdata.NewClient`, from wherever it keeps secrets.

| Token | Use |
|---|---|
| Production token | Real data, billed per matched record. |
| [Sandbox token](https://help.batchdata.io/en/articles/12606410-sandbox-api-tokens) | Same host, free mock data that varies and may lack attributes. The cheapest way to try the SDK against the real API. |

<br/>

## 🏁 Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

func main() {
	// The application chooses where the token comes from; the SDK reads no environment.
	client, err := batchdata.NewClient(os.Getenv("BATCHDATA_API_TOKEN"))
	if err != nil {
		log.Fatal(err) // ErrInvalidConfig: no call was made
	}
	tracer := skiptrace.New(client)

	res, err := tracer.Trace(context.Background(), skiptrace.Request{
		Items: []skiptrace.Item{{
			PropertyAddress: skiptrace.Address{Street: "100 Example Ave", City: "Testville", State: "ZZ", Zip: "00000"},
		}},
	})
	if err != nil {
		log.Fatal(err) // the call may already have been processed and billed: do not resend blindly
	}
	for _, rec := range res.RecordsFor(0) {
		fmt.Println(rec.Outcome, len(rec.Persons))
	}
}
```

### Next steps

- [Set up a client once](#set-up-once) and send [one property or a batch](#one-property-or-many).
- Learn how to [read results](#reading-results) and [link records to items](#linking-records-to-items).
- Handle [errors](#errors) without resending billed calls.
- Run the [offline example](examples): `go run ./examples`.

<br/>

## 💡 Usage

### Set up once

Create one `Client` per token and one `skiptrace.Service`, and share them: both are immutable and safe for
concurrent use.

```go
client, err := batchdata.NewClient(apiKey,
	batchdata.WithTimeout(45*time.Second),      // the whole call, including the body read
	batchdata.WithUserAgent("example-app/1.0"), // replaces "go-batchdata"
)
if err != nil {
	return err // errors.Is(err, batchdata.ErrInvalidConfig); no call was made
}
tracer := skiptrace.New(client)
```

| Option | Default | Accepted values |
|---|---|---|
| `WithTimeout(d)` | 60s (`DefaultTimeout`) | `d > 0`. An earlier deadline on the call's context wins. |
| `WithMaxResponseBytes(n)` | 8 MiB (`DefaultMaxResponseBytes`) | 1 byte to 64 MiB. A larger response fails as `KindResponseTooLarge`. |
| `WithUserAgent(ua)` | `go-batchdata` | 1 to 256 bytes of visible ASCII or spaces, starting and ending with a visible character. |
| `WithHTTPClient(hc)` | a transport the SDK builds | A non-nil `*http.Client`; see [the transport](#the-transport). |
| `WithTestEndpoint(url)` | `https://api.batchdata.com` | Tests only: `https` on `127.0.0.0/8` or `[::1]`, an optional port, and no path, query, fragment or userinfo. |

Nil options are skipped, options apply in order, and the last option of each kind wins. `NewClient`
validates the token (1 to 4096 bytes of visible ASCII) and every option before returning, and performs no
I/O. Its errors name the argument or option, never the rejected value.

> **Holding many tokens?** Without `WithHTTPClient`, each `Client` owns a connection pool that cannot be
> closed (idle connections close on their own after 90 seconds). Pass the same `*http.Client` to every
> `Client` so that they share one pool.

### Inputs and limits

A request carries 1 to 100 items (`skiptrace.MaxItems`). Each item must satisfy at least one of the
combinations BatchData documents:

| Lookup | Fields |
|---|---|
| Address | `PropertyAddress` with `Street`, `City`, `State` and `Zip` |
| Address | `PropertyAddress` with `Street` and `Zip` |
| Address | `PropertyAddress` with `Street`, `City` and `State` |
| Parcel | `APN`, `County` and `State` |
| Parcel | `APN` and `CountyFIPSCode` (exactly 5 ASCII digits) |

`Name` and `MailingAddress` are optional extras, never lookup keys, and extra fields alongside a satisfied
combination are allowed. `Request.Validate`, which `Trace` runs before any request, also rejects a string
that is invalid UTF-8 or only whitespace, an address set without a `Street`, and a `DateFormat` other than
the two constants. Values are sent as given: the SDK never trims, normalizes or rewrites them. The encoded
request must fit in 1 MiB (`batchdata.MaxRequestBytes`).

A request that fails any check returns a `*batchdata.ValidationError` matching
`batchdata.ErrInvalidRequest`, naming the first problem by field path, such as
`Items[3].PropertyAddress.Street`. No call is made.

### One property, or many

A single property is a one-item request; a batch is up to 100 items in one call. BatchData notes that
large synchronous batches respond more slowly.

```go
many := skiptrace.Request{
	Items: []skiptrace.Item{
		{RequestID: "row-1", PropertyAddress: skiptrace.Address{Street: "100 Example Ave", City: "Testville", State: "ZZ"}},
		{RequestID: "row-2", APN: "000-000-0000", CountyFIPSCode: "00000"},
	},
	Options: skiptrace.Options{IncludeTCPABlacklistedPhones: new(true), DateFormat: skiptrace.DateFormatISODate},
}
res, err := tracer.Trace(ctx, many)
```

### Options

An option you leave unset is omitted from the request, so BatchData's own default applies. The SDK never
injects a default.

| Option | Meaning |
|---|---|
| `IncludeTCPABlacklistedPhones` | `true` includes phones with TCPA restrictions, each flagged by `Phone.TCPA`; `false` filters them out. |
| `PrioritizeMobilePhones` | `true` (BatchData's default) lists mobile numbers first; `false` leaves the order alone. |
| `DateFormat` | `DateFormatISODateTime` (BatchData's default) or `DateFormatISODate`, for `Person.DOB`. |

> **Set `IncludeTCPABlacklistedPhones` explicitly.** BatchData's documentation contradicts itself: the
> option is described as defaulting to `true` in V3, but the `tcpa` field is described as appearing only when
> the option is `true`, with TCPA-restricted phones excluded when it is not specified.

### Reading results

`Result.Records` holds one entry per element of BatchData's data array, in BatchData's order: never
reordered, deduplicated or truncated. Each record has an `Outcome`:

| Outcome | Meaning |
|---|---|
| `OutcomeMatched` | BatchData set `meta.matched` to true: it found data for the property. |
| `OutcomeNoMatch` | `meta.matched` is false. A valid answer, not an error; `Persons` is usually an empty slice. |
| `OutcomeFailed` | BatchData set `meta.error` to true. This wins over `meta.matched`, and BatchData documents no detail for it. |
| `OutcomeUnreported` | No `meta.matched` flag was sent. |
| `OutcomeUndecodable` | The element was null, not an object, or held a value of the wrong type. Nothing of it is kept: its `Association` is `AssociationNone` and every other field is zero. |

A failed or undecodable record never turns into an error: `Trace` returns the `Result` with every paid
record in it.

```go
for _, rec := range res.Records {
	switch rec.Outcome {
	case skiptrace.OutcomeMatched:
		for _, p := range rec.Persons {
			for _, ph := range p.Phones {
				if ph.TCPA == nil {
					// not reported: absent is not false
				}
				_ = ph.Rank // presentation order, not a probability
			}
		}
	case skiptrace.OutcomeNoMatch: // a valid answer
	case skiptrace.OutcomeFailed, skiptrace.OutcomeUnreported, skiptrace.OutcomeUndecodable:
	}
}
billed := res.Meta.Results.MatchCount // *int; nil means not reported
supportID := res.Meta.CallRequestID   // identifies the call, not an item
```

Good to know about the data:

- **A match is about the property, not the name you sent.** `Name` is not a lookup key, and a matched
  record can describe whoever BatchData links to the address, such as a later resident. Compare each
  person's `Name.Full` and `Name.AKAs` with the name you hold, as BatchData's own field documentation
  advises.
- **Flags are pointers.** `nil` means BatchData did not send the flag. `PropertyOwner` is a name match
  against the owner of record, not proof of ownership; `nil` means it could not be evaluated. `Deceased`
  and `DOB` are `nil` when no value is on file.
- **Ranks are presentation order,** starting at 1, not measured confidence; 0 means not reported.
- **Documented maxima are documentation only.** BatchData documents at most 3 persons, 5 phones, 3
  emails, 3 addresses and 3 AKAs; extra entries are kept.
- **Unknown values are kept as sent:** `PhoneType`, `AddressValidity` and `batchdata.WarningCode`.
- **`DOB` stays a string,** in whichever format `DateFormat` selected; the SDK never parses it.
- **Flags are provider data, not consent or compliance decisions.** `DNC`, `TCPA` and `Litigator` describe
  what BatchData knows; whether and how you contact anyone is your decision and your responsibility.

### Linking records to items

`Record.Association` says what links a record to a request item, and `Result.RecordsFor(i)` returns the
records linked to `Items[i]`. The rule is strict: **never by position, name or address.**

| Association | When |
|---|---|
| `AssociationRequestID` | The record echoes an `input.requestId` equal to the `RequestID` of exactly one item. Every record echoing it links to that item. |
| `AssociationSingleItem` | The request had one item and the response one decodable record, and either the item had no `RequestID` or the record echoed none (a provider-generated echo is covered too). |
| `AssociationAmbiguous` | The echo matches an item's `RequestID`, but more than one item has it, or another item's differs only in case or surrounding whitespace. |
| `AssociationNone` | Anything else: no echo in a multi-item request, an echo that matches no item, a single item whose ID differs from the echo, or an undecodable record. |

> **Give every item of a batch a distinct `RequestID`, and expect `AssociationNone` until BatchData
> confirms the echo.** The V3 request schema documents `requestId` for correlation, but the V3 response
> schema does not list it; only BatchData's
> [request ID correlation article](https://help.batchdata.io/en/articles/13376439-async-api-request-id-correlation-january-2026)
> describes the echo, with examples in an older shape. BatchData does not document whether `data[i]`
> answers `requests[i]`, so the SDK never assumes it. When `RecordsFor` returns nothing, each record's
> echoed `Input` is still there to inspect yourself.

### Errors

Every error from `NewClient` and `Trace` falls into exactly one row of the
[error reference](#error-reference). Match the classes with `errors.Is`, and use
`errors.AsType[*batchdata.Error]` for the details of a call that passed validation.

```go
res, err := tracer.Trace(ctx, req)
switch {
case errors.Is(err, batchdata.ErrInvalidConfig), errors.Is(err, batchdata.ErrInvalidRequest):
	// local: no call was made
case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
	// stopped: the call may already have been processed and billed
case errors.Is(err, batchdata.ErrUnauthorized), errors.Is(err, batchdata.ErrForbidden):
	// token, permissions or balance
case errors.Is(err, batchdata.ErrRateLimited):
	if e, ok := errors.AsType[*batchdata.Error](err); ok && e.HasRetryAfter {
		_ = e.RetryAfter // a hint, not permission to repeat the call
	}
case errors.Is(err, batchdata.ErrRejected), errors.Is(err, batchdata.ErrMalformedResponse):
	// refused or unusable: it may already have been billed
case err != nil:
	// transport failure: the outcome is unknown, so do not resend blindly
}
```

`*batchdata.Error` carries `Kind`, `StatusCode` (HTTP), `ProviderCode` (the body's `status.code` when it
could be decoded), and `RetryAfter`/`HasRetryAfter` from a standard `Retry-After` header, clamped to 24
hours. A 403 means insufficient permissions **or** insufficient balance; the SDK does not guess which.

Error strings use a fixed vocabulary, such as `batchdata: rejected (HTTP 429)`. BatchData's own text is
available only on request, through `ProviderMessage()` and `Warnings()`, because it may echo your input.

### Timeouts, billing and repeat calls

- BatchData bills each matched property record with at least one person: `Meta.Results.MatchCount` is
  the billed count.
- The SDK never retries, hedges, falls back, splits a request or polls, and never follows a redirect. It
  sends the request without a rewindable body, so Go's HTTP/1.1 and HTTP/2 clients cannot resend it either.
- **A canceled or timed-out call may still be processed and billed.** Cancellation stops the SDK from
  waiting, not BatchData from working.
- **`Retry-After` is not proof that a repeat is safe or free,** and a malformed response may already have
  been billed. BatchData documents no idempotency mechanism, so the SDK sends none.
- BatchData allows 3,000 requests per minute per account, shared by all of its tokens
  ([rate and request limits](https://help.batchdata.io/en/articles/11880437-batchdata-api-rate-and-request-limits)),
  and documents no rate-limit headers.
- `DefaultTimeout` (60 seconds) covers dialing, sending and reading the whole body. For large batches,
  raise it or send fewer items per call.

### In a net/http handler

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := h.Tracer.Trace(ctx, buildRequest(r))
	if err != nil {
		// map err as above; never echo ProviderMessage to the browser
		http.Error(w, "skip trace failed", http.StatusBadGateway)
		return
	}
	render(w, res.Records)
}
```

### Testing your application

Depend on a small interface of your own and fake it:

```go
type SkipTracer interface {
	Trace(ctx context.Context, req skiptrace.Request) (*skiptrace.Result, error)
}

var _ SkipTracer = (*skiptrace.Service)(nil)

type fakeTracer struct {
	res *skiptrace.Result
	err error
}

func (f fakeTracer) Trace(context.Context, skiptrace.Request) (*skiptrace.Result, error) {
	return f.res, f.err
}
```

To exercise the real SDK offline, serve a synthetic response from `httptest.NewTLSServer` and pass
`batchdata.WithHTTPClient(srv.Client())` and `batchdata.WithTestEndpoint(srv.URL)`, as the
[examples](examples) do.

### Storing results

`skiptrace.Result` round-trips through `encoding/json` unchanged (`reflect.DeepEqual`), keeping nil and
empty slices, and absent and `false` flags, apart. `encoding/json` decodes a JSON `null` and an absent
field alike, which matches BatchData's meaning for every nullable field. The SDK keeps no raw body.

<br/>

## ⚙️ Behavior

### The transport

Without `WithHTTPClient`, each `Client` builds its own `*http.Transport` with the settings of
`http.DefaultTransport` (HTTP/2, and the standard dial, TLS and idle timeouts), plus a 64 KiB limit on
response headers. Later changes to `http.DefaultTransport` do not reach it. Like the default transport, it
takes a proxy from the `HTTPS_PROXY` and `NO_PROXY` environment variables; every call is https, so
`HTTP_PROXY` is never used. The SDK's own code reads no environment variable.

With `WithHTTPClient(hc)`, the SDK uses a copy of `hc` taken at `NewClient`, with redirects disabled and no
cookie jar. `hc` itself is never modified, later changes to it have no effect, its `Timeout` still
applies, and its `Transport` is shared. If `hc.Transport` is nil, Go uses `http.DefaultTransport`, and the
64 KiB header limit does not apply. **An injected transport is trusted code:** one that retries, follows
redirects or replays requests on its own defeats the one-attempt guarantee.

Every request is a `POST` to `https://api.batchdata.com/api/v3/property/skip-trace` with
`Authorization: Bearer <token>`, `Content-Type: application/json`, `Accept: application/json` and the
User-Agent, and never an idempotency key. The SDK never sets `Accept-Encoding`: Go's transport may request
gzip on its own and then decompresses it, and the size limit applies to the decompressed bytes. A response
body is always closed and never read more than one byte past the limit.

### Limits

| Bound | Value |
|---|---|
| Items per request | 1 to 100 |
| Encoded request | 1 MiB |
| Response body | 8 MiB by default, up to 64 MiB |
| Rejection body read | 64 KiB, best-effort |
| Response headers (SDK-built transport) | 64 KiB |
| JSON objects and arrays in a 200 body | 100,000 |
| JSON values in a 200 body | 1,000,000 |
| Call duration | 60 seconds by default, including the whole body read |

Decoding turns small JSON values into much larger Go values, so the byte limit alone would not bound
memory. A full answer to 100 items holds a few thousand objects and arrays, and a response over either
JSON bound fails as `ErrMalformedResponse` before it is decoded.

### Error reference

<details>
<summary><strong><code>Every error, and what it matches</code></strong></summary>
<br/>

| What happened | `Kind` | `errors.Is` matches |
|---|---|---|
| Invalid token or option, or a nil or zero client | (`*ValidationError`) | `ErrInvalidConfig` |
| Nil context, or a request failing validation | (`*ValidationError`) | `ErrInvalidRequest` |
| Context already done before the call (no attempt) | `canceled` / `deadline_exceeded` | `context.Canceled` / `context.DeadlineExceeded` |
| No response: caller canceled | `canceled` | `context.Canceled` |
| No response: caller deadline | `deadline_exceeded` | `context.DeadlineExceeded` |
| No response: `WithTimeout` or the `http.Client` timeout | `timeout` | `context.DeadlineExceeded` |
| No response: any other failure | `transport` | none |
| HTTP 1xx, 3xx, 4xx or 5xx (redirects included) | `rejected` | `ErrRejected`, plus `ErrUnauthorized` (401), `ErrForbidden` (403) or `ErrRateLimited` (429) by **HTTP status** |
| HTTP 2xx other than 200 | `malformed_response` | `ErrMalformedResponse` |
| HTTP 200, body over the size limit | `response_too_large` | `ErrMalformedResponse` |
| HTTP 200, body read interrupted | `canceled` / `deadline_exceeded` / `timeout` / `transport` | as the rows above |
| HTTP 200 with a `Content-Encoding` the transport did not decode | `malformed_response` | `ErrMalformedResponse` |
| HTTP 200, body over the JSON bounds or not the documented envelope | `malformed_response` | `ErrMalformedResponse` |
| HTTP 200 whose `status.code` is present and not 200 | `rejected` | `ErrRejected`, plus the 401/403/429 sentinel by **`ProviderCode`** |

The body of a rejection is read best-effort, up to 64 KiB, and dropped if it cannot be decoded: the
status alone classifies it. `Unwrap` returns only `context.Canceled` or `context.DeadlineExceeded`; the
underlying transport error is never kept.

</details>

### Privacy

- Error strings, `GoString`, every `fmt` verb on a `Client` or an error, and every error in an `Unwrap`
  chain never contain the token, URLs, request or response bodies, names, addresses, phones, emails,
  BatchData's messages, or network addresses. The token and URL live only inside a function the `Client`
  holds, which `fmt` and reflection-based dumpers cannot see into.
- The SDK has no logger, telemetry, cache or persistence.
- **Results are personal data.** Access control, logging, retention, and lawful and permissible use are
  your responsibility. A hashed field is not anonymous.

### Contract uncertainties

<details>
<summary><strong><code>What BatchData's V3 documentation leaves open, and what the SDK does</code></strong></summary>
<br/>

| Question | SDK behavior |
|---|---|
| Is each item's `requestId` echoed, and where? | Read from `input.requestId` when present; otherwise no link by ID. |
| Does `data[i]` answer `requests[i]`, and are the counts equal? | Never assumed. |
| How does an item that "couldn't be processed" (`errorCount`) appear? | `meta.error: true` becomes `OutcomeFailed`; a record that is missing is simply missing. |
| What does the 422 for more than 100 items look like? | Requests over 100 items fail locally; any rejection body is read best-effort. |
| Are there rate-limit headers? | A standard `Retry-After` is reported if present. |
| What is the TCPA default? | Never injected; set the option explicitly. |

The SDK is strict about the response envelope: a type change in `status`, `result.options` or
`result.meta` fails the call with `ErrMalformedResponse`, even though BatchData may have billed it.
`encoding/json` matches keys without regard to case; for a duplicate key, the last scalar or array wins,
while duplicate objects merge field by field.

</details>

### What is not covered

The asynchronous skip-trace endpoint, reverse skip tracing (by phone or email) and BatchData's other V3
APIs. New services can be added as sibling packages without changing the root package; see
[adding a service](docs/adding-a-service.md).

<br/>

## 📚 Documentation

View the generated [documentation](https://pkg.go.dev/github.com/mrz1836/go-batchdata?tab=doc) and
[BatchData's API documentation](https://developer.batchdata.com).

| Package | Purpose |
|---|---|
| [`batchdata`](https://pkg.go.dev/github.com/mrz1836/go-batchdata) | The `Client`, its options, and the errors every service shares |
| [`skiptrace`](https://pkg.go.dev/github.com/mrz1836/go-batchdata/skiptrace) | Synchronous forward property skip tracing |

> **Heads up!** `go-batchdata` is intentionally light on dependencies. The only external package it
uses is the excellent `testify` suite—and that's just for our tests. You can drop this library into
your projects without dragging along extra baggage.

<br/>

<details>
<summary><strong><code>Development Setup (Getting Started)</code></strong></summary>
<br/>

Install [MAGE-X](https://github.com/mrz1836/mage-x) build tool for development:

```bash
# Install MAGE-X for development and building
go install github.com/mrz1836/mage-x/cmd/magex@latest
magex update:install
```
</details>

<details>
<summary><strong><code>Library Deployment</code></strong></summary>
<br/>

This project uses [goreleaser](https://github.com/goreleaser/goreleaser) for streamlined library releases to GitHub. To get started, install it via:

```bash
brew install goreleaser
```

The release process is defined in the [.goreleaser.yml](.goreleaser.yml) configuration file.

Then create and push a new Git tag using:

```bash
magex version:bump bump=patch push=true branch=master
```

</details>

<details>
<summary><strong><code>Build Commands</code></strong></summary>
<br/>

View all build commands

```bash script
magex help
```

</details>

<details>
<summary><strong><code>GitHub Workflows</code></strong></summary>
<br/>

All workflows are driven by modular configuration in [`.github/env/`](.github/env/README.md) — no YAML editing required.

**[View all workflows and the control center →](.github/docs/workflows.md)**

</details>

<details>
<summary><strong><code>Updating Dependencies</code></strong></summary>
<br/>

To update all dependencies (Go modules, linters, and related tools), run:

```bash
magex deps:update
```

</details>

<br/>

## 🧪 Examples & Tests

All unit tests, fuzz tests and examples run via [GitHub Actions](https://github.com/mrz1836/go-batchdata/actions) on Go 1.26.x (the supported baseline) and the current Go release. View the [configuration file](.github/workflows/fortress.yml).

Run all tests (fast):

```bash script
magex test
```

Run all tests with race detector (slower):
```bash script
magex test:race
```

Runnable examples live in the `Example*` functions of both packages and in [examples/](examples), which
serves a synthetic response from a local TLS server:

```bash
go run ./examples # offline: no token, no network, no cost
```

> **Note:** tests never touch the network. They use in-memory transports and local TLS servers over
HTTP/1.1 and HTTP/2, with synthetic data only.

<br/>

## ⚡ Benchmarks

Run the Go benchmarks:

```bash script
magex bench
```

<br/>

### 📊 Performance Results

| Benchmark | Iterations | ns/op | B/op | allocs/op |
|---|---:|---:|---:|---:|
| [Validate100](skiptrace/validate_test.go) | 49,106 | 24,547 | 0 | 0 |
| [Encode100](skiptrace/request_test.go) | 13,681 | 87,719 | 41,172 | 3 |
| [Trace100](skiptrace/benchmark_test.go) | 1,125 | 1,069,554 | 468,237 | 2,173 |
| [Do](internal/httpx/benchmark_test.go) | 763,564 | 1,570 | 2,720 | 27 |

> Validate100 and Encode100 check and encode a 100-item request; Trace100 makes a full call against an
> in-memory transport and decodes a 100-record response; Do is one transport round trip in memory.
> They measure local CPU work only, executed on an Apple M1 Max (ARM64), and make no claim about network
> or BatchData latency, which dominates real calls.

<br/>

## 🛠️ Code Standards
Read more about this Go project's [code standards](.github/CODE_STANDARDS.md).

<br/>

## 🤖 AI Usage & Assistant Guidelines
Read the [AI Usage & Assistant Guidelines](.github/tech-conventions/ai-compliance.md) for details on how AI is used in this project and how to interact with the AI assistants.

<br/>

## 👥 Maintainers
| [<img src="https://github.com/mrz1836.png" height="50" alt="MrZ" />](https://github.com/mrz1836) |
|:------------------------------------------------------------------------------------------------:|
|                                [MrZ](https://github.com/mrz1836)                                 |

<br/>

## 🤝 Contributing
View the [contributing guidelines](.github/CONTRIBUTING.md) and please follow the [code of conduct](.github/CODE_OF_CONDUCT.md).

### How can I help?
All kinds of contributions are welcome :raised_hands:!
The most basic way to show your support is to star :star2: the project, or to raise issues :speech_balloon:.
You can also support this project by [becoming a sponsor on GitHub](https://github.com/sponsors/mrz1836) :clap:
or by making a [**bitcoin donation**](https://mrz1818.com/?tab=tips&utm_source=github&utm_medium=sponsor-link&utm_campaign=go-batchdata&utm_term=go-batchdata&utm_content=go-batchdata) to ensure this journey continues indefinitely! :rocket:


[![Stars](https://img.shields.io/github/stars/mrz1836/go-batchdata?label=Please%20like%20us&style=social)](https://github.com/mrz1836/go-batchdata/stargazers)

<br/>

## 📝 License

[![License](https://img.shields.io/github/license/mrz1836/go-batchdata.svg?style=flat&v=1)](LICENSE)
