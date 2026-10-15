// Package batchdata is the foundation of a small Go SDK for the BatchData V3
// API.
//
// It provides the Client, an authenticated and immutable connection, and the
// error surface every service shares. Typed access to each API lives in its
// own package, such as github.com/mrz1836/go-batchdata/skiptrace, so importing
// one service pulls in only that service, this package and the standard
// library.
//
// Construct a client once and hand it to a service:
//
//	client, err := batchdata.NewClient(apiKey)
//	if err != nil {
//		return err
//	}
//	tracer := skiptrace.New(client)
//	res, err := tracer.Trace(ctx, req)
//
// The SDK makes at most one HTTP attempt per call. It never retries, follows
// redirects, replays a request or reads the environment for credentials. Every
// call may be billed, and a failed call may already have been processed.
package batchdata
