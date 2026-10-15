package batchdata_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

const (
	testKey       = "SENTINEL-KEY-0000"
	fieldEndpoint = "WithTestEndpoint"
)

func TestNewClientInvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"empty", ""},
		{"too long", strings.Repeat("k", 4097)},
		{"inner space", "SENTINEL KEY"},
		{"leading space", " SENTINEL-KEY"},
		{"tab", "SENTINEL\tKEY"},
		{"newline", "SENTINEL-KEY\n"},
		{"carriage return", "SENTINEL\rKEY"},
		{"delete", "SENTINEL\x7fKEY"},
		{"non ascii", "SENTINEL-KÉY"},
		{"nul", "SENTINEL\x00KEY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := batchdata.NewClient(tt.key)
			assert.Nil(t, c)
			ve := requireValidation(t, err, batchdata.ErrInvalidConfig)
			assert.Equal(t, "apiKey", ve.Field)
			if tt.key != "" {
				assert.NotContains(t, err.Error(), tt.key)
			}
			assert.NotContains(t, err.Error(), "SENTINEL")
		})
	}
}

func TestNewClientKeyLengthBoundary(t *testing.T) {
	c, err := batchdata.NewClient(strings.Repeat("k", 4096))
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestNewClientInvalidOptions(t *testing.T) {
	tests := []struct {
		name  string
		opts  []batchdata.Option
		field string
	}{
		{"nil http client", []batchdata.Option{batchdata.WithHTTPClient(nil)}, "WithHTTPClient"},
		{"zero timeout", []batchdata.Option{batchdata.WithTimeout(0)}, "WithTimeout"},
		{"negative timeout", []batchdata.Option{batchdata.WithTimeout(-time.Second)}, "WithTimeout"},
		{"zero max bytes", []batchdata.Option{batchdata.WithMaxResponseBytes(0)}, "WithMaxResponseBytes"},
		{"negative max bytes", []batchdata.Option{batchdata.WithMaxResponseBytes(-1)}, "WithMaxResponseBytes"},
		{"max bytes over limit", []batchdata.Option{batchdata.WithMaxResponseBytes(batchdata.MaxResponseBytesLimit + 1)}, "WithMaxResponseBytes"},
		{"empty user agent", []batchdata.Option{batchdata.WithUserAgent("")}, "WithUserAgent"},
		{"long user agent", []batchdata.Option{batchdata.WithUserAgent(strings.Repeat("u", 257))}, "WithUserAgent"},
		{"user agent leading space", []batchdata.Option{batchdata.WithUserAgent(" SENTINEL-UA")}, "WithUserAgent"},
		{"user agent trailing space", []batchdata.Option{batchdata.WithUserAgent("SENTINEL-UA ")}, "WithUserAgent"},
		{"user agent newline", []batchdata.Option{batchdata.WithUserAgent("SENTINEL-UA\r\nX-Injected: 1")}, "WithUserAgent"},
		{"user agent non ascii", []batchdata.Option{batchdata.WithUserAgent("SENTINEL-ÜA")}, "WithUserAgent"},
		{"endpoint empty", []batchdata.Option{batchdata.WithTestEndpoint("")}, fieldEndpoint},
		{"endpoint http", []batchdata.Option{batchdata.WithTestEndpoint("http://127.0.0.1:8443")}, fieldEndpoint},
		{"endpoint localhost", []batchdata.Option{batchdata.WithTestEndpoint("https://localhost:8443")}, fieldEndpoint},
		{"endpoint public ip", []batchdata.Option{batchdata.WithTestEndpoint("https://192.0.2.1")}, fieldEndpoint},
		{"endpoint production", []batchdata.Option{batchdata.WithTestEndpoint("https://api.batchdata.com")}, fieldEndpoint},
		{"endpoint userinfo", []batchdata.Option{batchdata.WithTestEndpoint("https://SENTINEL:pass@127.0.0.1")}, fieldEndpoint},
		{"endpoint query", []batchdata.Option{batchdata.WithTestEndpoint("https://127.0.0.1/?SENTINEL=1")}, fieldEndpoint},
		{"endpoint fragment", []batchdata.Option{batchdata.WithTestEndpoint("https://127.0.0.1/#SENTINEL")}, fieldEndpoint},
		{"endpoint path", []batchdata.Option{batchdata.WithTestEndpoint("https://127.0.0.1/api/v3")}, fieldEndpoint},
		{"endpoint port zero", []batchdata.Option{batchdata.WithTestEndpoint("https://127.0.0.1:0")}, fieldEndpoint},
		{"last invalid option wins", []batchdata.Option{batchdata.WithTimeout(time.Second), batchdata.WithTimeout(0)}, "WithTimeout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := batchdata.NewClient(testKey, tt.opts...)
			assert.Nil(t, c)
			ve := requireValidation(t, err, batchdata.ErrInvalidConfig)
			assert.Equal(t, tt.field, ve.Field)
			assert.NotContains(t, err.Error(), "SENTINEL")
			assert.NotContains(t, err.Error(), "127.0.0.1")
		})
	}
}

func TestNewClientValidOptions(t *testing.T) {
	tests := []struct {
		name string
		opts []batchdata.Option
		want string
	}{
		{"no options", nil, "batchdata.Client{timeout: 1m0s, maxResponseBytes: 8388608}"},
		{"nil options skipped", []batchdata.Option{nil, batchdata.WithTimeout(time.Second), nil}, "batchdata.Client{timeout: 1s, maxResponseBytes: 8388608}"},
		{"timeout", []batchdata.Option{batchdata.WithTimeout(45 * time.Second)}, "batchdata.Client{timeout: 45s, maxResponseBytes: 8388608}"},
		{"min max bytes", []batchdata.Option{batchdata.WithMaxResponseBytes(1)}, "batchdata.Client{timeout: 1m0s, maxResponseBytes: 1}"},
		{"max max bytes", []batchdata.Option{batchdata.WithMaxResponseBytes(batchdata.MaxResponseBytesLimit)}, "batchdata.Client{timeout: 1m0s, maxResponseBytes: 67108864}"},
		{"user agent", []batchdata.Option{batchdata.WithUserAgent("example-app/1.0 (+test)")}, "batchdata.Client{timeout: 1m0s, maxResponseBytes: 8388608}"},
		{"user agent 256 bytes", []batchdata.Option{batchdata.WithUserAgent(strings.Repeat("u", 256))}, "batchdata.Client{timeout: 1m0s, maxResponseBytes: 8388608}"},
		{"http client", []batchdata.Option{batchdata.WithHTTPClient(&http.Client{})}, "batchdata.Client{timeout: 1m0s, maxResponseBytes: 8388608}"},
		{"ipv4 endpoint", []batchdata.Option{batchdata.WithTestEndpoint("https://127.0.0.1:8443")}, "batchdata.Client{timeout: 1m0s, maxResponseBytes: 8388608}"},
		{"ipv6 endpoint", []batchdata.Option{batchdata.WithTestEndpoint("https://[::1]:8443/")}, "batchdata.Client{timeout: 1m0s, maxResponseBytes: 8388608}"},
		{"last timeout wins", []batchdata.Option{batchdata.WithTimeout(0), batchdata.WithTimeout(2 * time.Second)}, "batchdata.Client{timeout: 2s, maxResponseBytes: 8388608}"},
		{"last max bytes wins", []batchdata.Option{batchdata.WithMaxResponseBytes(0), batchdata.WithMaxResponseBytes(10)}, "batchdata.Client{timeout: 1m0s, maxResponseBytes: 10}"},
		{"last http client wins", []batchdata.Option{batchdata.WithHTTPClient(nil), batchdata.WithHTTPClient(&http.Client{})}, "batchdata.Client{timeout: 1m0s, maxResponseBytes: 8388608}"},
		{"last endpoint wins", []batchdata.Option{batchdata.WithTestEndpoint("http://bad"), batchdata.WithTestEndpoint("https://127.0.0.1")}, "batchdata.Client{timeout: 1m0s, maxResponseBytes: 8388608}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := batchdata.NewClient(testKey, tt.opts...)
			require.NoError(t, err)
			require.NotNil(t, c)
			assert.Equal(t, tt.want, c.String())
			assert.Equal(t, c.String(), c.GoString())
		})
	}
}

func TestNewClientDoesNotModifyHTTPClient(t *testing.T) {
	tr := &http.Transport{}
	hc := &http.Client{Transport: tr, Timeout: 3 * time.Second}
	_, err := batchdata.NewClient(testKey, batchdata.WithHTTPClient(hc))
	require.NoError(t, err)
	assert.Same(t, tr, hc.Transport)
	assert.Nil(t, hc.CheckRedirect)
	assert.Nil(t, hc.Jar)
	assert.Equal(t, 3*time.Second, hc.Timeout)
}

// requireValidation asserts that err is a *batchdata.ValidationError matching
// sentinel and returns it.
func requireValidation(t *testing.T, err, sentinel error) *batchdata.ValidationError {
	t.Helper()
	ve := testutil.RequireErrorAs[*batchdata.ValidationError](t, err)
	require.ErrorIs(t, err, sentinel)
	return ve
}
