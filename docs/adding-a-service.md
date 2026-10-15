# Adding a Service Package

This is the recipe for wrapping another BatchData V3 endpoint, such as property lookup or address
verification. The `skiptrace` package is the reference implementation: when in doubt, copy its shape.

## Architecture in one breath

- **Small root** (`package batchdata`): the `Client` (immutable, functional options), the public error
  surface (`Error`, `Kind`, `ValidationError`, the sentinels, `Warning`) and the one place that turns an
  HTTP exchange into those errors. It exports no generic `Do` or `Post`.
- **One package per service** (`skiptrace`, ...). Each imports only the root, `internal/call`,
  `internal/jsonx` and the standard library. A service never imports another service. Types live in
  the service package, so `skiptrace.Address` and a future `property.Address` never collide.
- **`internal/call`**: the sealed bridge. The root `Client` embeds a `call.Handle` under an unexported
  alias, and a service gets the client's `call.Sender` with `call.FuncOf(client)`. Code outside this
  module can neither import the package nor reach the sender.
- **`internal/httpx`**: the single transport. One bounded POST attempt, no redirects, no replays, and a
  `*Failure` that holds no cause. It never imports the root.
- **`internal/jsonx`**: the shared byte-level JSON checks. `Present`, `IsObject` and `IsArray` test the
  shape of a raw value, and `Count` measures a document against the root's shape budget. Decoders call
  them instead of keeping their own copies. It never imports the root.
- **`internal/testutil`**: test-only fakes and leak assertions. Only `_test.go` files import it; the
  `depguard` linter enforces this.

The root does not change when a service is added.

## Files per service

```
<service>/
  doc.go        package doc comment: the endpoint, and a short usage example
  service.go    Service, New(client), the call methods
  request.go    request types and their JSON tags
  validate.go   Request.Validate: local checks, first problem only
  result.go     result types and their JSON tags
  decode.go     strict decoding of the result object
  *_test.go     validation tables, golden request JSON, decoding fixtures, association,
                end-to-end tests over internal/testutil servers, fuzz targets, benchmarks
```

## Step by step

1. **Read the contract.** Use BatchData's OpenAPI export for the endpoint, and record the export's
   SHA-256 in your notes. Mark each point verified, contradictory or unresolved, and give every
   unresolved point a conservative behavior. Never commit the export: it is proprietary.
2. **`New` and `Service`.** Copy `skiptrace.New`: return a `Service` whose sender is nil for a nil or
   zero client, so that every call fails with `batchdata.ErrInvalidConfig` and makes none.
3. **Types.** Every field carries an explicit tag equal to the documented camelCase wire name
   (`musttag` checks this). Rules:
   - A response boolean is a `*bool`: an absent flag is not `false`.
   - A count is a `*int`: an absent count is not `0`.
   - Slices use `omitzero`, so nil is omitted and an empty array is kept; pointers and strings use
     `omitempty`.
   - A documented enum is a named string type with constants, and unknown values are kept as sent.
   - Documented maxima are documentation: extra items are kept, never truncated.
   - A type that another type embeds must never gain a custom JSON marshaler: it would be promoted.
4. **Validate.** Check everything the provider documents as required, before any I/O. Report the first
   problem as a `*batchdata.ValidationError` matching `batchdata.ErrInvalidRequest`, with a Go field path
   (`Items[3].PropertyAddress.Street`) and a fixed reason. Never quote a value. Never trim or rewrite
   one: reject it instead.
5. **The call method.** Follow `skiptrace.Service.Trace` in order: the service, the context, a private
   copy of caller slices, `Validate`, encoding, the `batchdata.MaxRequestBytes` check, then one
   `send(ctx, call.Call{Path, Body, Decode})`. The path is relative to `/api/v3`.
6. **Decode.** `Decode` receives the envelope's `result` object, already checked to be an object, and
   the envelope's warnings. Be strict on the result's own shape (return any error, and the root reports
   `ErrMalformedResponse`), and decode each element of a data array on its own, so that one bad element
   becomes an undecodable entry instead of failing a billed call.
7. **Tests.** Cover the wire request byte for byte, every zero-call path with an exact attempt count,
   validation tables, decoding fixtures with nulls, absent fields, unknown enums and oversized arrays,
   a JSON round trip of the result, the HTTP status table end to end, and fuzz targets for validation
   and decoding. Fixtures are synthetic: `555-01xx` numbers, `example.com` addresses, invented streets
   with ZIP `00000`.
8. **Wire up.** Add the package to the README's package table and to `examples/`.

## Invariants a service must keep

- **One attempt.** No retries, fallback, hedging, chunking or polling: every call may be billed.
- **Nothing implicit.** No environment reads, globals, `init`, injected defaults or hidden repeat calls.
- **No secrets or payloads in errors.** Error strings come from fixed vocabularies; provider text is
  reachable only through `Error.ProviderMessage` and `Error.Warnings`.
- **No positional guessing.** Never link a response element to a request element by position, name or
  address: only by evidence the provider documents.
