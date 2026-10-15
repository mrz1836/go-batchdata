package skiptrace

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/call"
)

// path is the synchronous property skip-trace endpoint, relative to the API
// root.
const path = "/property/skip-trace"

// Service makes synchronous property skip-trace calls. It is immutable and
// safe for concurrent use.
type Service struct {
	send call.Sender[batchdata.Warning]
}

// New returns a Service that calls through client. A nil or zero client yields
// a Service whose every call fails with batchdata.ErrInvalidConfig and makes
// no request.
func New(client *batchdata.Client) *Service {
	if client == nil {
		return &Service{}
	}
	return &Service{send: call.FuncOf[batchdata.Warning](client)}
}

// Trace makes one synchronous skip-trace call for req and returns the decoded
// records.
//
// Before any request it checks, in order: the Service and its client
// (batchdata.ErrInvalidConfig), ctx (batchdata.ErrInvalidRequest when nil),
// req.Validate, and the size of the encoded request (batchdata.ErrInvalidRequest
// above batchdata.MaxRequestBytes). A context that is already done fails with
// its error as a *batchdata.Error. None of these make a call.
//
// Otherwise Trace makes exactly one HTTP attempt: it never retries, follows a
// redirect, or splits the request. Every other error is a *batchdata.Error,
// and the call may already have been processed and billed: do not resend
// blindly. Records with OutcomeNoMatch, OutcomeFailed, OutcomeUnreported or
// OutcomeUndecodable are part of a successful Result, never an error.
func (s *Service) Trace(ctx context.Context, req Request) (*Result, error) {
	if s == nil || s.send == nil {
		return nil, &batchdata.ValidationError{
			Err: batchdata.ErrInvalidConfig, Field: "Service",
			Reason: "is not initialized; create it with skiptrace.New and a client from batchdata.NewClient",
		}
	}
	if ctx == nil {
		return nil, invalidRequest("ctx", "must not be nil")
	}
	// A private copy of the items: the caller may reuse its slice once Trace
	// has returned, or even while it runs, without changing what is validated,
	// sent or linked.
	req.Items = slices.Clone(req.Items)
	if err := req.Validate(); err != nil {
		return nil, err
	}
	ids := make([]string, len(req.Items))
	for i := range req.Items {
		ids[i] = req.Items[i].RequestID
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, invalidRequest("Request", "cannot be encoded")
	}
	if len(body) > batchdata.MaxRequestBytes {
		return nil, invalidRequest("Request", "encodes to more than MaxRequestBytes (1 MiB)")
	}
	var res *Result
	err = s.send(ctx, call.Call[batchdata.Warning]{
		Path: path,
		Body: body,
		Decode: func(result []byte, warnings []batchdata.Warning) error {
			r, derr := decodeResult(result, ids)
			if derr != nil {
				return derr
			}
			r.Warnings = warnings
			res = r
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
