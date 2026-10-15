// Package testutil holds the test plumbing go-batchdata's packages share:
// in-memory and recording HTTP fakes over HTTP/1.1 and HTTP/2, fmt leak
// assertions, and checks of error types and JSON round trips.
//
// It is imported only by tests. It imports testify and the standard library,
// and never a package of this module, so every test package can use it.
package testutil
