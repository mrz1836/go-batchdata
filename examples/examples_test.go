package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

func TestRunPrintsTheTrace(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, run(t.Context(), &out))
	assert.Equal(t, `row-1: matched (linked by request_id)
  phone rank 1, Mobile, DNC false, TCPA false
  phone rank 2, Land Line, DNC true, TCPA not reported
row-2: no_match (linked by request_id)
billed items: 1, call ID: 01EXAMPLECALLID00000000000
counts: {"requestCount":2,"matchCount":1,"noMatchCount":1,"errorCount":0}
`, out.String())
}

func TestDescribe(t *testing.T) {
	// Trace fails with a context that is already done before making a request.
	client, err := batchdata.NewClient("EXAMPLE-KEY-0000")
	require.NoError(t, err)
	tracer := skiptrace.New(client)
	req := skiptrace.Request{Items: []skiptrace.Item{{RequestID: "row-1", APN: "000-000-0000", CountyFIPSCode: "00000"}}}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	_, errCanceled := tracer.Trace(canceled, req)
	require.Error(t, errCanceled)
	expired, cancelExpired := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancelExpired()
	_, errExpired := tracer.Trace(expired, req)
	require.Error(t, errExpired)

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			"invalid config", &batchdata.ValidationError{Err: batchdata.ErrInvalidConfig, Field: "apiKey"},
			"fix the request; no call was made",
		},
		{
			"invalid request", &batchdata.ValidationError{Err: batchdata.ErrInvalidRequest, Field: "Items"},
			"fix the request; no call was made",
		},
		{
			"unauthorized", &batchdata.Error{Kind: batchdata.KindRejected, StatusCode: 401},
			"check the token, its permissions and the account balance",
		},
		{
			"forbidden", &batchdata.Error{Kind: batchdata.KindRejected, StatusCode: 403},
			"check the token, its permissions and the account balance",
		},
		{
			"rate limited with a hint",
			&batchdata.Error{Kind: batchdata.KindRejected, StatusCode: 429, RetryAfter: 30 * time.Second, HasRetryAfter: true},
			"rate limited; the provider hints 30s, which is not permission to repeat the call",
		},
		{
			"rate limited without a hint", &batchdata.Error{Kind: batchdata.KindRejected, StatusCode: 429},
			"rate limited; slow down before calling again",
		},
		{"canceled", errCanceled, "stopped; the call may already have been processed and billed"},
		{"deadline exceeded", errExpired, "stopped; the call may already have been processed and billed"},
		{
			"rejected", &batchdata.Error{Kind: batchdata.KindRejected, StatusCode: 500},
			"rejected or unusable response; it may already have been billed, so do not resend blindly",
		},
		{
			"malformed response", &batchdata.Error{Kind: batchdata.KindMalformedResponse, StatusCode: 200},
			"rejected or unusable response; it may already have been billed, so do not resend blindly",
		},
		{
			"transport", &batchdata.Error{Kind: batchdata.KindTransport},
			"transport failure; the outcome is unknown, so do not resend blindly",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			got := describe(&out, tt.err)
			assert.Same(t, tt.err, got, "describe returns the error unchanged")
			assert.Equal(t, tt.want+"\n", out.String())
		})
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		name string
		n    *int
		want string
	}{
		{"not reported", nil, "not reported"},
		{"zero", new(0), "0"},
		{"positive", new(3), "3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, count(tt.n))
		})
	}
}
