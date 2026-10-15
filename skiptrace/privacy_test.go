package skiptrace_test

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/testutil"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

// Secrets planted in every input of a call. None may ever be printed by an
// error, a client, or anything an error chain reaches.
const (
	plantedToken    = "SENTINEL-KEY-0001"
	secretMessage   = "SENTINEL-PROVIDER-MESSAGE"
	secretWarning   = "SENTINEL-PROVIDER-WARNING"
	secretBody      = "SENTINEL-RESPONSE-BODY"
	secretTransport = "SENTINEL-TRANSPORT-ERROR"
	secretProxy     = "SENTINEL-PROXY-PASSWORD"
	secretName      = "SENTINEL-PERSON-NAME"
	secretStreet    = "SENTINEL-STREET-ADDRESS"
	secretID        = "SENTINEL-REQUEST-ID"
)

// errPlantedTransport is a transport error quoting a proxy address and its
// password, as some transports do.
var errPlantedTransport = errors.New(secretTransport + " dial tcp 192.0.2.10:3128: " + secretProxy)

// secrets returns every planted secret, plus the parts of a URL a leak would
// show.
func secrets(extra ...string) []string {
	return append([]string{
		plantedToken, secretMessage, secretWarning, secretBody, secretTransport, secretProxy,
		secretName, secretStreet, secretID, "Bearer", "api.batchdata.com", "/api/v3",
	}, extra...)
}

// privateRequest carries secrets in every caller-supplied value.
func privateRequest() skiptrace.Request {
	return skiptrace.Request{Items: []skiptrace.Item{{
		RequestID:       secretID,
		PropertyAddress: skiptrace.Address{Street: secretStreet, Zip: "00000"},
		Name:            skiptrace.Name{First: secretName, Last: secretName},
	}}}
}

// chain returns err and everything its Unwrap methods reach, depth first.
func chain(err error) []error {
	if err == nil {
		return nil
	}
	out := []error{err}
	switch u := err.(type) { //nolint:errorlint // walking the chain by hand is the point
	case interface{ Unwrap() []error }:
		for _, e := range u.Unwrap() {
			out = append(out, chain(e)...)
		}
	case interface{ Unwrap() error }:
		out = append(out, chain(u.Unwrap())...)
	}
	return out
}

// assertErrorRedacted checks err, its chain, and every way of holding and
// printing it.
func assertErrorRedacted(t *testing.T, err error, extra ...string) {
	t.Helper()
	require.Error(t, err)
	s := secrets(extra...)
	for _, e := range chain(err) {
		testutil.AssertRedacted(t, e.Error(), s, "Error() of %T", e)
		testutil.AssertNeverPrints(t, e, s...)
	}
	testutil.AssertShapesNeverPrint(t, &err, s...)
	if e, ok := errors.AsType[*batchdata.Error](err); ok {
		testutil.AssertRedacted(t, e.GoString(), s)
		testutil.AssertShapesNeverPrint(t, e, s...)
	}
}

func TestErrorsNeverRevealSecrets(t *testing.T) {
	rejectBody := `{"status":{"code":401,"message":"` + secretMessage + ` ` + secretName + `",` +
		`"warnings":[{"code":"dataset_not_allowed","message":"` + secretWarning + `"}]},` +
		`"warnings":[{"code":"x","message":"` + secretWarning + `"}]}`
	tests := []struct {
		name   string
		status int
		header http.Header
		body   string
		kind   batchdata.Kind
	}{
		{"http rejection", 401, http.Header{"X-Echo": {plantedToken}}, rejectBody, batchdata.KindRejected},
		{"embedded rejection", 200, nil, rejectBody, batchdata.KindRejected},
		{"malformed body", 200, nil, `{"result":"` + secretBody + `"}`, batchdata.KindMalformedResponse},
		{"not json", 200, nil, secretBody, batchdata.KindMalformedResponse},
		{"gateway page", 502, nil, `<html>` + secretBody + `</html>`, batchdata.KindRejected},
		{"unexpected 2xx", 206, nil, secretBody, batchdata.KindMalformedResponse},
		{"redirect", 307, http.Header{"Location": {"https://" + plantedToken + ".example.test/"}}, secretBody, batchdata.KindRejected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := func(r *http.Request) (*http.Response, error) {
				testutil.Drain(r)
				h := http.Header{}
				for k, v := range tt.header {
					h[k] = v
				}
				return &http.Response{
					StatusCode: tt.status, Header: h, Body: io.NopCloser(strings.NewReader(tt.body)),
					ContentLength: int64(len(tt.body)), Request: r,
				}, nil
			}
			c, err := batchdata.NewClient(plantedToken,
				batchdata.WithHTTPClient(&http.Client{Transport: testutil.RoundTripFunc(rt)}),
				batchdata.WithTestEndpoint("https://127.0.0.1:8443"))
			require.NoError(t, err)
			_, err = skiptrace.New(c).Trace(t.Context(), privateRequest())
			e := testutil.RequireErrorAs[*batchdata.Error](t, err)
			assert.Equal(t, tt.kind, e.Kind)
			assertErrorRedacted(t, err, "127.0.0.1", "8443")
		})
	}
}

func TestTransportErrorsNeverRevealSecrets(t *testing.T) {
	failing := testutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
		testutil.Drain(r)
		return nil, &url.Error{
			Op:  "Post",
			URL: "https://user:" + secretProxy + "@127.0.0.1:8443/?" + plantedToken,
			Err: errPlantedTransport,
		}
	})
	c, err := batchdata.NewClient(plantedToken,
		batchdata.WithHTTPClient(&http.Client{Transport: failing}),
		batchdata.WithTestEndpoint("https://127.0.0.1:8443"))
	require.NoError(t, err)
	_, err = skiptrace.New(c).Trace(t.Context(), privateRequest())
	assert.Equal(t, batchdata.KindTransport, testutil.RequireErrorAs[*batchdata.Error](t, err).Kind)
	assertErrorRedacted(t, err, "192.0.2.10", "127.0.0.1", "3128")
}

func TestProxyCredentialsNeverRevealed(t *testing.T) {
	proxy := &url.URL{Scheme: "http", User: url.UserPassword("user", secretProxy), Host: "127.0.0.1:1"}
	hc := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}, Timeout: 5 * time.Second}
	c, err := batchdata.NewClient(plantedToken, batchdata.WithHTTPClient(hc), batchdata.WithTestEndpoint("https://127.0.0.1:9"))
	require.NoError(t, err)
	_, err = skiptrace.New(c).Trace(t.Context(), privateRequest())
	assert.Equal(t, batchdata.KindTransport, testutil.RequireErrorAs[*batchdata.Error](t, err).Kind)
	assertErrorRedacted(t, err, "127.0.0.1")
}

func TestValidationErrorsNeverRevealValues(t *testing.T) {
	req := privateRequest()
	req.Items[0].Name.Last = " \t "
	c, err := batchdata.NewClient(plantedToken)
	require.NoError(t, err)
	_, err = skiptrace.New(c).Trace(t.Context(), req)
	require.ErrorIs(t, err, batchdata.ErrInvalidRequest)
	assertErrorRedacted(t, err)
}

func TestProviderTextIsOptIn(t *testing.T) {
	body := `{"status":{"code":429,"message":"` + secretMessage + `","warnings":[{"code":"warning_rate_limit_approaching","message":"` +
		secretWarning + `"}]}}`
	c, err := batchdata.NewClient(plantedToken,
		batchdata.WithHTTPClient(&http.Client{Transport: reply(http.StatusTooManyRequests, body)}),
		batchdata.WithTestEndpoint("https://127.0.0.1"))
	require.NoError(t, err)
	_, err = skiptrace.New(c).Trace(t.Context(), privateRequest())
	e := testutil.RequireErrorAs[*batchdata.Error](t, err)
	assert.Equal(t, secretMessage, e.ProviderMessage())
	assert.Equal(t, []batchdata.Warning{{Code: batchdata.WarningRateLimitApproaching, Message: secretWarning}}, e.Warnings())

	other := *e
	other.StatusCode = 500
	testutil.AssertDiffRedacted(t, e, &other, secrets()...)
}

func TestClientNeverRevealsSecrets(t *testing.T) {
	c, err := batchdata.NewClient(plantedToken,
		batchdata.WithTestEndpoint("https://127.0.0.1:8443"),
		batchdata.WithUserAgent("example-app/1.0"))
	require.NoError(t, err)
	s := secrets("127.0.0.1", "8443", "example-app")
	testutil.AssertShapesNeverPrint(t, c, s...)
	testutil.AssertShapesNeverPrint(t, skiptrace.New(c), s...)
	other, err := batchdata.NewClient(plantedToken, batchdata.WithTimeout(time.Second))
	require.NoError(t, err)
	testutil.AssertDiffRedacted(t, c, other, s...)
}
