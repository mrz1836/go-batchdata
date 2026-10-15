// Command examples shows the go-batchdata SDK end to end, offline: it serves a
// synthetic skip-trace response from a local TLS server, so it needs no API
// key, makes no network call and costs nothing.
//
//	go run ./examples
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

// fakeResponse is a synthetic answer: one matched record and one no-match
// record, each echoing its item's request ID.
const fakeResponse = `{
  "status": {"code": 200, "text": "OK", "message": "Success"},
  "result": {
    "data": [
      {
        "input": {"requestId": "row-1", "propertyAddress": {"street": "100 Example Ave", "city": "Testville", "state": "ZZ"}},
        "persons": [{
          "propertyOwner": true,
          "name": {"first": "Sample", "last": "Person", "full": "Sample Person"},
          "phones": [
            {"rank": 1, "number": "2025550100", "type": "Mobile", "tested": true, "reachable": true, "dnc": false, "tcpa": false},
            {"rank": 2, "number": "2025550101", "type": "Land Line", "dnc": true}
          ],
          "emails": [{"rank": 1, "email": "sample.person@example.com", "tested": true}],
          "litigator": false
        }],
        "meta": {"matched": true, "error": false}
      },
      {
        "input": {"requestId": "row-2", "apn": "000-000-0000", "countyFipsCode": "00000"},
        "persons": [],
        "meta": {"matched": false, "error": false}
      }
    ],
    "meta": {
      "apiVersion": "3.1.0",
      "results": {"requestCount": 2, "matchCount": 1, "noMatchCount": 1, "errorCount": 0},
      "requestId": "01EXAMPLECALLID00000000000"
    }
  }
}`

func main() {
	if err := run(context.Background(), os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "examples:", err)
		os.Exit(1)
	}
}

// run starts the local fake, traces two properties against it, and writes
// what came back to w.
func run(ctx context.Context, w io.Writer) error {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(rw, fakeResponse)
	}))
	defer srv.Close()

	// A real application passes its own key, from wherever it keeps
	// secrets, and leaves out the two test options.
	client, err := batchdata.NewClient("EXAMPLE-KEY-0000",
		batchdata.WithTimeout(45*time.Second),
		batchdata.WithUserAgent("example-app/1.0"),
		batchdata.WithHTTPClient(srv.Client()),
		batchdata.WithTestEndpoint(srv.URL),
	)
	if err != nil {
		return err
	}
	tracer := skiptrace.New(client)

	req := skiptrace.Request{
		Items: []skiptrace.Item{
			{RequestID: "row-1", PropertyAddress: skiptrace.Address{Street: "100 Example Ave", City: "Testville", State: "ZZ"}},
			{RequestID: "row-2", APN: "000-000-0000", CountyFIPSCode: "00000"},
		},
		Options: skiptrace.Options{IncludeTCPABlacklistedPhones: new(true), DateFormat: skiptrace.DateFormatISODate},
	}
	res, err := tracer.Trace(ctx, req)
	if err != nil {
		return describe(w, err)
	}

	for i, item := range req.Items {
		recs := res.RecordsFor(i)
		if len(recs) != 1 {
			_, _ = fmt.Fprintf(w, "%s: no record linked by evidence\n", item.RequestID)
			continue
		}
		rec := recs[0]
		_, _ = fmt.Fprintf(w, "%s: %s (linked by %s)\n", item.RequestID, rec.Outcome, rec.Association)
		for _, p := range rec.Persons {
			for _, ph := range p.Phones {
				_, _ = fmt.Fprintf(w, "  phone rank %d, %s, DNC %s, TCPA %s\n", ph.Rank, ph.Type, flag(ph.DNC), flag(ph.TCPA))
			}
		}
	}
	_, _ = fmt.Fprintf(w, "billed items: %s, call ID: %s\n", count(res.Meta.Results.MatchCount), res.Meta.CallRequestID)

	b, err := json.Marshal(res.Meta.Results)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "counts: %s\n", b)
	return nil
}

// describe writes the class of a failed call. A call that was made may have
// been billed, so none of these branches resends it.
func describe(w io.Writer, err error) error {
	switch {
	case errors.Is(err, batchdata.ErrInvalidConfig), errors.Is(err, batchdata.ErrInvalidRequest):
		_, _ = fmt.Fprintln(w, "fix the request; no call was made")
	case errors.Is(err, batchdata.ErrUnauthorized), errors.Is(err, batchdata.ErrForbidden):
		_, _ = fmt.Fprintln(w, "check the token, its permissions and the account balance")
	case errors.Is(err, batchdata.ErrRateLimited):
		if e, ok := errors.AsType[*batchdata.Error](err); ok && e.HasRetryAfter {
			_, _ = fmt.Fprintf(w, "rate limited; the provider hints %s, which is not permission to repeat the call\n", e.RetryAfter)
		} else {
			_, _ = fmt.Fprintln(w, "rate limited; slow down before calling again")
		}
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		_, _ = fmt.Fprintln(w, "stopped; the call may already have been processed and billed")
	case errors.Is(err, batchdata.ErrRejected), errors.Is(err, batchdata.ErrMalformedResponse):
		_, _ = fmt.Fprintln(w, "rejected or unusable response; it may already have been billed, so do not resend blindly")
	default:
		_, _ = fmt.Fprintln(w, "transport failure; the outcome is unknown, so do not resend blindly")
	}
	return err
}

// flag prints a reported flag, or "not reported" for nil.
func flag(b *bool) string {
	if b == nil {
		return "not reported"
	}
	return fmt.Sprint(*b)
}

// count prints a reported count, or "not reported" for nil.
func count(n *int) string {
	if n == nil {
		return "not reported"
	}
	return fmt.Sprint(*n)
}
