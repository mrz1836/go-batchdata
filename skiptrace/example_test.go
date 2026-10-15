package skiptrace_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

// exampleServer serves body from a local TLS server, standing in for
// BatchData so that the examples run offline.
func exampleServer(body string) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
}

// One property: a request with a single item. With one item and one record,
// the record is linked to the item by elimination.
func ExampleService_Trace() {
	srv := exampleServer(`{"status":{"code":200},"result":{"data":[{"persons":[` +
		`{"name":{"full":"Sample Person"},"phones":[{"rank":1,"number":"2025550100","type":"Mobile","dnc":false}]}],` +
		`"meta":{"matched":true,"error":false}}],"meta":{"results":{"matchCount":1}}}}`)
	defer srv.Close()

	client, err := batchdata.NewClient("EXAMPLE-KEY-0000",
		batchdata.WithHTTPClient(srv.Client()), batchdata.WithTestEndpoint(srv.URL))
	if err != nil {
		fmt.Println(err)
		return
	}
	res, err := skiptrace.New(client).Trace(context.Background(), skiptrace.Request{Items: []skiptrace.Item{{
		PropertyAddress: skiptrace.Address{Street: "100 Example Ave", Zip: "00000"},
	}}})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, rec := range res.RecordsFor(0) {
		fmt.Println(rec.Outcome, rec.Association)
		for _, p := range rec.Persons {
			for _, ph := range p.Phones {
				// Flags are pointers: nil means BatchData did not report it.
				dnc := "not reported"
				if ph.DNC != nil {
					dnc = fmt.Sprint(*ph.DNC)
				}
				fmt.Println(p.Name.Full, ph.Rank, ph.Type, "DNC:", dnc, "TCPA reported:", ph.TCPA != nil)
			}
		}
	}
	if n := res.Meta.Results.MatchCount; n != nil {
		fmt.Println("billed:", *n)
	}
	// Output:
	// matched single_item
	// Sample Person 1 Mobile DNC: false TCPA reported: false
	// billed: 1
}

// Several properties: records are linked to items only by an echoed request
// ID that is unique among the items, never by position.
func ExampleResult_RecordsFor() {
	srv := exampleServer(`{"status":{"code":200},"result":{"data":[` +
		`{"input":{"requestId":"row-2"},"persons":[],"meta":{"matched":false,"error":false}},` +
		`{"input":{"requestId":"row-1"},"persons":[{"name":{"full":"Sample Person"}}],"meta":{"matched":true,"error":false}}]}}`)
	defer srv.Close()

	client, err := batchdata.NewClient("EXAMPLE-KEY-0000",
		batchdata.WithHTTPClient(srv.Client()), batchdata.WithTestEndpoint(srv.URL))
	if err != nil {
		fmt.Println(err)
		return
	}
	req := skiptrace.Request{Items: []skiptrace.Item{
		{RequestID: "row-1", PropertyAddress: skiptrace.Address{Street: "100 Example Ave", City: "Testville", State: "ZZ"}},
		{RequestID: "row-2", APN: "000-000-0000", CountyFIPSCode: "00000"},
	}}
	res, err := skiptrace.New(client).Trace(context.Background(), req)
	if err != nil {
		fmt.Println(err)
		return
	}
	for i, item := range req.Items {
		for _, rec := range res.RecordsFor(i) {
			fmt.Println(item.RequestID, rec.Outcome, len(rec.Persons))
		}
	}
	// Output:
	// row-1 matched 1
	// row-2 no_match 0
}
