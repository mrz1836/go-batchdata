// Package skiptrace provides synchronous forward property skip tracing on the
// BatchData V3 API: POST /api/v3/property/skip-trace.
//
// Construct a Service from a *batchdata.Client and call Trace:
//
//	tracer := skiptrace.New(client)
//	res, err := tracer.Trace(ctx, skiptrace.Request{Items: []skiptrace.Item{{
//		PropertyAddress: skiptrace.Address{Street: "100 Example Ave", Zip: "00000"},
//	}}})
//
// Decode decodes the kept body of a response, with the request's item IDs,
// into the Result or the error Trace returned for it, without a call:
//
//	res, err := skiptrace.Decode(body, req)
//
// Importing this package pulls in only github.com/mrz1836/go-batchdata and the
// standard library.
package skiptrace
