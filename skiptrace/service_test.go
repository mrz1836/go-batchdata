package skiptrace_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/testutil"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

const (
	testKey       = "SENTINEL-KEY-0000"
	skipTracePath = "/api/v3/property/skip-trace"
)

// Static errors the fakes return.
var (
	errReplacedTransport = errors.New("SENTINEL replaced transport")
	errWrongRecords      = errors.New("wrong records")
)

// envelope wraps record JSON values in a documented success envelope.
func envelope(records ...string) string {
	return `{"status":{"code":200,"text":"OK","message":"Success"},"result":{"data":[` + strings.Join(records, ",") +
		`],"options":{"includeTCPABlacklistedPhones":true,"dateFormat":"iso-date"},` +
		`"meta":{"apiVersion":"3.1.0","results":{"requestCount":2,"matchCount":1,"noMatchCount":1,"errorCount":0},` +
		`"requestId":"01SYNTHETICCALLID000000000"}}}`
}

// matched returns a matched record echoing id, with one person and phone.
func matched(id string) string {
	return `{"input":{"requestId":"` + id + `","propertyAddress":{"street":"100 Example Ave","city":"Testville","state":"ZZ","zip":"00000"}},` +
		`"property":{"id":"SYNTHETIC-PROPERTY-0001","address":{"street":"100 Example Ave","zip":"00000","addressValidity":"Valid"}},` +
		`"persons":[{"propertyOwner":true,"name":{"first":"Sentinel","last":"Person","full":"Sentinel Person"},` +
		`"phones":[{"rank":1,"number":"2025550100","type":"Mobile","tested":true,"reachable":true,"dnc":false,"tcpa":false}],` +
		`"emails":[{"rank":1,"email":"sentinel@example.test","tested":true}],"litigator":false,"deceased":null,"dob":"1970-01-01"}],` +
		`"meta":{"matched":true,"error":false}}`
}

// noMatch returns a no-match record echoing id.
func noMatch(id string) string {
	return `{"input":{"requestId":"` + id + `","apn":"000-000-0000","countyFipsCode":"00000"},"persons":[],` +
		`"meta":{"matched":false,"error":false}}`
}

// manyRequest is a two-item request with explicit options.
func manyRequest() skiptrace.Request {
	return skiptrace.Request{
		Items: []skiptrace.Item{
			{RequestID: "row-1", PropertyAddress: skiptrace.Address{Street: "100 Example Ave", City: "Testville", State: "ZZ"}},
			{RequestID: "row-2", APN: "000-000-0000", CountyFIPSCode: "00000"},
		},
		Options: skiptrace.Options{IncludeTCPABlacklistedPhones: new(true), DateFormat: skiptrace.DateFormatISODate},
	}
}

// oneRequest is a one-item request without a request ID or options.
func oneRequest() skiptrace.Request {
	return skiptrace.Request{Items: []skiptrace.Item{{
		PropertyAddress: skiptrace.Address{Street: "100 Example Ave", Zip: "00000"},
	}}}
}

// serveOK returns a handler answering 200 with body as JSON.
func serveOK(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}
}

// serverTracer returns a Service calling srv through a transport that counts
// attempts.
func serverTracer(t *testing.T, srv *testutil.Server, opts ...batchdata.Option) (*skiptrace.Service, *testutil.CountingTransport) {
	t.Helper()
	hc := srv.Client()
	ct := &testutil.CountingTransport{Next: hc.Transport}
	hc.Transport = ct
	opts = append([]batchdata.Option{batchdata.WithHTTPClient(hc), batchdata.WithTestEndpoint(srv.URL())}, opts...)
	c, err := batchdata.NewClient(testKey, opts...)
	require.NoError(t, err)
	return skiptrace.New(c), ct
}

// memoryTracer returns a Service whose calls go to rt in memory, through a
// transport that counts attempts.
func memoryTracer(tb testing.TB, rt testutil.RoundTripFunc) (*skiptrace.Service, *testutil.CountingTransport) {
	tb.Helper()
	ct := &testutil.CountingTransport{Next: rt}
	c, err := batchdata.NewClient(testKey,
		batchdata.WithHTTPClient(&http.Client{Transport: ct}),
		batchdata.WithTestEndpoint("https://127.0.0.1"),
	)
	require.NoError(tb, err)
	return skiptrace.New(c), ct
}

// reply returns an in-memory round trip answering status and body.
func reply(status int, body string) testutil.RoundTripFunc {
	return func(r *http.Request) (*http.Response, error) {
		testutil.Drain(r)
		return &http.Response{
			StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Request: r,
		}, nil
	}
}

func TestTraceWireRequest(t *testing.T) {
	const golden = `{"requests":[{"requestId":"row-1","propertyAddress":{"street":"100 Example Ave","city":"Testville","state":"ZZ"}},` +
		`{"requestId":"row-2","apn":"000-000-0000","countyFipsCode":"00000"}],` +
		`"options":{"includeTCPABlacklistedPhones":true,"dateFormat":"iso-date"}}`
	for _, proto := range testutil.Protocols() {
		t.Run(proto.Name(), func(t *testing.T) {
			srv := testutil.NewServerProto(t, proto, serveOK(envelope(matched("row-1"), noMatch("row-2"))))
			tracer, ct := serverTracer(t, srv, batchdata.WithUserAgent("example-app/1.0"))
			_, err := tracer.Trace(t.Context(), manyRequest())
			require.NoError(t, err)
			assert.Equal(t, 1, ct.Attempts())
			reqs := srv.Requests()
			require.Len(t, reqs, 1)
			r := reqs[0]
			assert.Equal(t, string(proto), r.Proto)
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, skipTracePath, r.URL)
			assert.Equal(t, []string{"Bearer " + testKey}, r.Header.Values("Authorization"))
			assert.Equal(t, []string{"application/json"}, r.Header.Values("Content-Type"))
			assert.Equal(t, []string{"application/json"}, r.Header.Values("Accept"))
			assert.Equal(t, []string{"example-app/1.0"}, r.Header.Values("User-Agent"))
			assert.Empty(t, r.Header.Values("Idempotency-Key"))
			assert.Empty(t, r.Header.Values("Cookie"))
			assert.Equal(t, golden, string(r.Body)) //nolint:testifylint // the golden body is compared byte for byte, not as JSON
		})
	}
}

func TestTraceDefaultUserAgent(t *testing.T) {
	srv := testutil.NewServer(t, serveOK(envelope(noMatch("x"))))
	tracer, _ := serverTracer(t, srv)
	_, err := tracer.Trace(t.Context(), oneRequest())
	require.NoError(t, err)
	reqs := srv.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "go-batchdata", reqs[0].Header.Get("User-Agent"))
	assert.JSONEq(t, `{"requests":[{"propertyAddress":{"street":"100 Example Ave","zip":"00000"}}]}`, string(reqs[0].Body))
}

func TestTraceZeroCalls(t *testing.T) {
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	expired, cancelExpired := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancelExpired()

	huge := skiptrace.Request{Items: make([]skiptrace.Item, skiptrace.MaxItems)}
	for i := range huge.Items {
		huge.Items[i] = skiptrace.Item{PropertyAddress: skiptrace.Address{Street: strings.Repeat("s", 11<<10), Zip: "00000"}}
	}
	tooMany := skiptrace.Request{Items: make([]skiptrace.Item, skiptrace.MaxItems+1)}

	tests := []struct {
		name    string
		ctx     context.Context //nolint:containedctx // table input
		req     skiptrace.Request
		want    error
		field   string
		kind    batchdata.Kind
		tracerF func(*skiptrace.Service) *skiptrace.Service
	}{
		{
			name: "nil service", ctx: t.Context(), req: oneRequest(), want: batchdata.ErrInvalidConfig, field: "Service",
			tracerF: func(*skiptrace.Service) *skiptrace.Service { return nil },
		},
		{
			name: "nil client", ctx: t.Context(), req: oneRequest(), want: batchdata.ErrInvalidConfig, field: "Service",
			tracerF: func(*skiptrace.Service) *skiptrace.Service { return skiptrace.New(nil) },
		},
		{
			name: "zero client", ctx: t.Context(), req: oneRequest(), want: batchdata.ErrInvalidConfig, field: "Service",
			tracerF: func(*skiptrace.Service) *skiptrace.Service { return skiptrace.New(&batchdata.Client{}) },
		},
		{
			name: "zero service", ctx: t.Context(), req: oneRequest(), want: batchdata.ErrInvalidConfig, field: "Service",
			tracerF: func(*skiptrace.Service) *skiptrace.Service { return &skiptrace.Service{} },
		},
		{name: "nil context", ctx: nil, req: oneRequest(), want: batchdata.ErrInvalidRequest, field: "ctx"},
		{name: "no items", ctx: t.Context(), req: skiptrace.Request{}, want: batchdata.ErrInvalidRequest, field: "Items"},
		{name: "too many items", ctx: t.Context(), req: tooMany, want: batchdata.ErrInvalidRequest, field: "Items"},
		{
			name: "invalid item", ctx: t.Context(), req: skiptrace.Request{Items: []skiptrace.Item{{APN: "000-000-0000"}}},
			want: batchdata.ErrInvalidRequest, field: "Items[0]",
		},
		{
			name: "invalid date format", ctx: t.Context(),
			req:  skiptrace.Request{Items: oneRequest().Items, Options: skiptrace.Options{DateFormat: "SENTINEL"}},
			want: batchdata.ErrInvalidRequest, field: "Options.DateFormat",
		},
		{name: "oversized body", ctx: t.Context(), req: huge, want: batchdata.ErrInvalidRequest, field: "Request"},
		{name: "context canceled", ctx: canceled, req: oneRequest(), want: context.Canceled, kind: batchdata.KindCanceled},
		{name: "deadline passed", ctx: expired, req: oneRequest(), want: context.DeadlineExceeded, kind: batchdata.KindDeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := testutil.NewServer(t, serveOK(envelope()))
			tracer, ct := serverTracer(t, srv)
			if tt.tracerF != nil {
				tracer = tt.tracerF(tracer)
			}
			res, err := tracer.Trace(tt.ctx, tt.req)
			assert.Nil(t, res)
			require.ErrorIs(t, err, tt.want)
			if tt.kind != "" {
				assert.Equal(t, tt.kind, testutil.RequireErrorAs[*batchdata.Error](t, err).Kind)
			} else {
				ve := testutil.RequireErrorAs[*batchdata.ValidationError](t, err)
				assert.Equal(t, tt.field, ve.Field)
				assert.NotContains(t, err.Error(), "SENTINEL")
			}
			assert.Zero(t, ct.Attempts(), "no attempt may be made")
			assert.Zero(t, srv.Hits(), "no request may arrive")
		})
	}
}

func TestTraceDecodesResult(t *testing.T) {
	body := `{"status":{"code":200,"text":"OK","warnings":[{"code":"warning_deprecated_field","message":"a"}]},` +
		`"result":{"data":[` + noMatch("row-2") + `,` + matched("row-1") + `,null],` +
		`"meta":{"requestId":"01SYNTHETICCALLID000000000","results":{"requestCount":2,"matchCount":1}}},` +
		`"warnings":[{"code":"warning_rate_limit_approaching","message":"b"}]}`
	tracer, ct := memoryTracer(t, reply(http.StatusOK, body))
	res, err := tracer.Trace(t.Context(), manyRequest())
	require.NoError(t, err)
	assert.Equal(t, 1, ct.Attempts())
	require.Len(t, res.Records, 3)

	nm := res.Records[0]
	assert.Equal(t, skiptrace.OutcomeNoMatch, nm.Outcome)
	assert.Equal(t, skiptrace.AssociationRequestID, nm.Association)
	assert.Equal(t, new(1), nm.ItemIndex)
	assert.NotNil(t, nm.Persons, "an empty persons array is kept")
	assert.Empty(t, nm.Persons)
	assert.Nil(t, nm.Property)

	m := res.Records[1]
	assert.Equal(t, skiptrace.OutcomeMatched, m.Outcome)
	assert.Equal(t, new(0), m.ItemIndex)
	require.Len(t, m.Persons, 1)
	p := m.Persons[0]
	assert.Equal(t, new(true), p.PropertyOwner)
	assert.Nil(t, p.Deceased, "null is no value on file")
	assert.Equal(t, new("1970-01-01"), p.DOB)
	require.Len(t, p.Phones, 1)
	assert.Equal(t, skiptrace.PhoneTypeMobile, p.Phones[0].Type)
	assert.Equal(t, new(false), p.Phones[0].TCPA)
	require.NotNil(t, m.Property)
	assert.Equal(t, skiptrace.AddressValid, m.Property.Address.AddressValidity)

	bad := res.Records[2]
	assert.Equal(t, skiptrace.OutcomeUndecodable, bad.Outcome)
	assert.Equal(t, skiptrace.AssociationNone, bad.Association)

	assert.Equal(t, "01SYNTHETICCALLID000000000", res.Meta.CallRequestID)
	assert.Equal(t, new(1), res.Meta.Results.MatchCount)
	assert.Nil(t, res.Meta.Results.ErrorCount)
	assert.Equal(t, []batchdata.Warning{
		{Code: batchdata.WarningDeprecatedField, Message: "a"},
		{Code: batchdata.WarningRateLimitApproaching, Message: "b"},
	}, res.Warnings)

	assert.Equal(t, []skiptrace.Record{m}, res.RecordsFor(0))
	assert.Equal(t, []skiptrace.Record{nm}, res.RecordsFor(1))
	assert.Nil(t, res.RecordsFor(2))

	testutil.RequireJSONRoundTrip(t, *res)
}

func TestTraceSingleItemAssociation(t *testing.T) {
	tracer, _ := memoryTracer(t, reply(http.StatusOK, envelope(matched("01SYNTHETICGENERATEDID0000"))))
	res, err := tracer.Trace(t.Context(), oneRequest())
	require.NoError(t, err)
	recs := res.RecordsFor(0)
	require.Len(t, recs, 1)
	assert.Equal(t, skiptrace.AssociationSingleItem, recs[0].Association)
}

func TestTraceSnapshotsRequestIDs(t *testing.T) {
	req := manyRequest()
	// The round trip runs on the goroutine that called Trace, after Trace
	// copied the items: rewriting the caller's items here must change
	// neither what was sent nor how the records are linked.
	var sent []byte
	tracer, _ := memoryTracer(t, func(r *http.Request) (*http.Response, error) {
		sent, _ = io.ReadAll(r.Body)
		req.Items[0].RequestID = "row-2"
		req.Items[1].RequestID = "row-1"
		return reply(http.StatusOK, envelope(matched("row-1"), noMatch("row-2")))(r)
	})
	res, err := tracer.Trace(t.Context(), req)
	require.NoError(t, err)
	assert.Contains(t, string(sent), `"requestId":"row-1","propertyAddress"`)
	require.Len(t, res.Records, 2)
	assert.Equal(t, new(0), res.Records[0].ItemIndex)
	assert.Equal(t, new(1), res.Records[1].ItemIndex)
}

func TestTraceStatuses(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		header   http.Header
		body     string
		kind     batchdata.Kind
		code     int
		matches  []error
		message  string
		hasRetry bool
	}{
		{
			name: "401", status: 401, body: `{"status":{"code":401,"text":"Unauthorized","message":"SENTINEL-MSG token"}}`,
			kind: batchdata.KindRejected, code: 401, matches: []error{batchdata.ErrRejected, batchdata.ErrUnauthorized}, message: "SENTINEL-MSG token",
		},
		{
			name: "403", status: 403, body: `{"status":{"code":403}}`,
			kind: batchdata.KindRejected, code: 403, matches: []error{batchdata.ErrRejected, batchdata.ErrForbidden},
		},
		{
			name: "422 undocumented body", status: 422, body: `SENTINEL not json`,
			kind: batchdata.KindRejected, matches: []error{batchdata.ErrRejected},
		},
		{
			name: "429 retry-after", status: 429, header: http.Header{"Retry-After": {"60"}}, body: `{"status":{"code":429}}`,
			kind: batchdata.KindRejected, code: 429, matches: []error{batchdata.ErrRejected, batchdata.ErrRateLimited}, hasRetry: true,
		},
		{
			name: "500", status: 500, body: `{"status":{"code":500}}`,
			kind: batchdata.KindRejected, code: 500, matches: []error{batchdata.ErrRejected},
		},
		{
			name: "200 with body 401", status: 200, body: `{"status":{"code":401,"message":"SENTINEL-MSG"}}`,
			kind: batchdata.KindRejected, code: 401, matches: []error{batchdata.ErrRejected, batchdata.ErrUnauthorized}, message: "SENTINEL-MSG",
		},
		{
			name: "200 with body 429", status: 200, body: `{"status":{"code":429},"result":{"data":[]}}`,
			kind: batchdata.KindRejected, code: 429, matches: []error{batchdata.ErrRejected, batchdata.ErrRateLimited},
		},
		{
			name: "429 with body 200", status: 429, body: `{"status":{"code":200}}`,
			kind: batchdata.KindRejected, code: 200, matches: []error{batchdata.ErrRejected, batchdata.ErrRateLimited},
		},
		{name: "204", status: 204, body: ``, kind: batchdata.KindMalformedResponse, matches: []error{batchdata.ErrMalformedResponse}},
		{name: "206", status: 206, body: envelope(), kind: batchdata.KindMalformedResponse, matches: []error{batchdata.ErrMalformedResponse}},
		{
			name: "200 malformed", status: 200, body: `{"result":{"data":null}}`, kind: batchdata.KindMalformedResponse,
			matches: []error{batchdata.ErrMalformedResponse},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := testutil.NewServer(t, func(w http.ResponseWriter, _ *http.Request) {
				for k, v := range tt.header {
					w.Header()[k] = v
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			})
			tracer, ct := serverTracer(t, srv)
			res, err := tracer.Trace(t.Context(), manyRequest())
			assert.Nil(t, res)
			e := testutil.RequireErrorAs[*batchdata.Error](t, err)
			assert.Equal(t, tt.kind, e.Kind)
			assert.Equal(t, tt.status, e.StatusCode)
			assert.Equal(t, tt.code, e.ProviderCode)
			assert.Equal(t, tt.message, e.ProviderMessage())
			assert.Equal(t, tt.hasRetry, e.HasRetryAfter)
			for _, s := range tt.matches {
				require.ErrorIs(t, err, s)
			}
			assert.NotContains(t, err.Error(), "SENTINEL")
			assert.Equal(t, 1, ct.Attempts())
			assert.Equal(t, 1, srv.Hits())
		})
	}
}

func TestTraceNeverFollowsRedirects(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			target := testutil.NewServer(t, serveOK(envelope()))
			srv := testutil.NewServer(t, func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL()+skipTracePath, status)
			})
			tracer, ct := serverTracer(t, srv)
			_, err := tracer.Trace(t.Context(), oneRequest())
			e := testutil.RequireErrorAs[*batchdata.Error](t, err)
			assert.Equal(t, batchdata.KindRejected, e.Kind)
			assert.Equal(t, status, e.StatusCode)
			require.ErrorIs(t, err, batchdata.ErrRejected)
			assert.Equal(t, 1, ct.Attempts())
			assert.Zero(t, target.Hits(), "the redirect target must never be called")
		})
	}
}

func TestTraceTimeouts(t *testing.T) {
	block := func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}
	t.Run("client timeout", func(t *testing.T) {
		srv := testutil.NewServer(t, block)
		tracer, ct := serverTracer(t, srv, batchdata.WithTimeout(50*time.Millisecond))
		_, err := tracer.Trace(t.Context(), oneRequest())
		assert.Equal(t, batchdata.KindTimeout, testutil.RequireErrorAs[*batchdata.Error](t, err).Kind)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, 1, ct.Attempts())
	})
	t.Run("caller deadline", func(t *testing.T) {
		srv := testutil.NewServer(t, block)
		tracer, _ := serverTracer(t, srv)
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		_, err := tracer.Trace(ctx, oneRequest())
		assert.Equal(t, batchdata.KindDeadlineExceeded, testutil.RequireErrorAs[*batchdata.Error](t, err).Kind)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
	t.Run("http client timeout", func(t *testing.T) {
		srv := testutil.NewServer(t, block)
		hc := srv.Client()
		hc.Timeout = 50 * time.Millisecond
		c, err := batchdata.NewClient(testKey, batchdata.WithHTTPClient(hc), batchdata.WithTestEndpoint(srv.URL()))
		require.NoError(t, err)
		_, err = skiptrace.New(c).Trace(t.Context(), oneRequest())
		assert.Equal(t, batchdata.KindTimeout, testutil.RequireErrorAs[*batchdata.Error](t, err).Kind)
	})
}

func TestTraceIgnoresLaterHTTPClientChanges(t *testing.T) {
	srv := testutil.NewServer(t, serveOK(envelope(noMatch("x"))))
	hc := srv.Client()
	c, err := batchdata.NewClient(testKey, batchdata.WithHTTPClient(hc), batchdata.WithTestEndpoint(srv.URL()))
	require.NoError(t, err)
	hc.Timeout = time.Nanosecond
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }
	hc.Transport = testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errReplacedTransport
	})
	_, err = skiptrace.New(c).Trace(t.Context(), oneRequest())
	require.NoError(t, err)
	assert.Equal(t, 1, srv.Hits())
}

func TestTraceConcurrent(t *testing.T) {
	// The server echoes each item's request ID, so every caller can check it
	// got its own answer.
	srv := testutil.NewServerProto(t, testutil.HTTP2, func(w http.ResponseWriter, r *http.Request) {
		var req skiptrace.Request
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		recs := make([]string, len(req.Items))
		for i, it := range req.Items {
			recs[i] = noMatch(it.RequestID)
		}
		_, _ = io.WriteString(w, envelope(recs...))
	})
	tracer, ct := serverTracer(t, srv)
	const callers = 16
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, callers)
	for g := range callers {
		wg.Go(func() {
			<-start
			req := skiptrace.Request{Items: []skiptrace.Item{
				{RequestID: fmt.Sprintf("caller-%d-a", g), APN: "000-000-0000", CountyFIPSCode: "00000"},
				{RequestID: fmt.Sprintf("caller-%d-b", g), APN: "000-000-0001", CountyFIPSCode: "00000"},
			}}
			res, err := tracer.Trace(t.Context(), req)
			if err != nil {
				errs[g] = err
				return
			}
			for i := range req.Items {
				recs := res.RecordsFor(i)
				if len(recs) != 1 || recs[0].Input.RequestID != req.Items[i].RequestID {
					errs[g] = fmt.Errorf("caller %d item %d: %w", g, i, errWrongRecords)
				}
			}
		})
	}
	close(start)
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, callers, ct.Attempts())
	assert.Equal(t, callers, srv.Hits())
}

// echoServer answers every item with a no-match record echoing its request
// ID, and records each request body.
func echoServer(t *testing.T) *testutil.Server {
	t.Helper()
	return testutil.NewServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req skiptrace.Request
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		recs := make([]string, len(req.Items))
		for i, it := range req.Items {
			id, err := json.Marshal(it.RequestID)
			if !assert.NoError(t, err) {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			recs[i] = `{"input":{"requestId":` + string(id) + `},"persons":[],"meta":{"matched":false,"error":false}}`
		}
		_, _ = io.WriteString(w, envelope(recs...))
	})
}

func TestTraceOddRequestIDs(t *testing.T) {
	ids := []string{
		strings.Repeat("x", 4096),
		`quote " and backslash \ inside`,
		"<script>&amp;</script>",
		"\u65e5\u672c-ünïcödé",
		"line\u2028separator",
		"control\x01char",
		" leading and trailing ",
		"Row-1",
	}
	srv := echoServer(t)
	tracer, _ := serverTracer(t, srv)
	req := skiptrace.Request{Items: make([]skiptrace.Item, len(ids))}
	for i, id := range ids {
		req.Items[i] = skiptrace.Item{RequestID: id, APN: "000-000-0000", CountyFIPSCode: "00000"}
	}
	res, err := tracer.Trace(t.Context(), req)
	require.NoError(t, err)
	for i, id := range ids {
		recs := res.RecordsFor(i)
		require.Len(t, recs, 1, "item %d", i)
		assert.Equal(t, id, recs[0].Input.RequestID, "item %d: the ID must survive encoding unchanged", i)
		assert.Equal(t, skiptrace.AssociationRequestID, recs[0].Association)
	}
}

func TestTraceOptionsArePerCall(t *testing.T) {
	srv := echoServer(t)
	tracer, _ := serverTracer(t, srv)
	_, err := tracer.Trace(t.Context(), manyRequest())
	require.NoError(t, err)
	_, err = tracer.Trace(t.Context(), oneRequest())
	require.NoError(t, err)
	reqs := srv.Requests()
	require.Len(t, reqs, 2)
	assert.Contains(t, string(reqs[0].Body), `"options":{"includeTCPABlacklistedPhones":true,"dateFormat":"iso-date"}`)
	assert.NotContains(t, string(reqs[1].Body), "options", "a later call must not inherit an earlier call's options")
}
