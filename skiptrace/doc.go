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
// Importing this package pulls in only github.com/mrz1836/go-batchdata and the
// standard library.
package skiptrace
