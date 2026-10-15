package skiptrace_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/testutil"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

func TestRequestMarshalGolden(t *testing.T) {
	streetZip := skiptrace.Address{Street: fixtureStreet, Zip: fixtureZip}
	tests := []struct {
		name string
		req  skiptrace.Request
		want string
	}{
		{
			name: "minimal street zip without options",
			req:  oneItem(skiptrace.Item{PropertyAddress: streetZip}),
			want: `{"requests":[{"propertyAddress":{"street":"100 Example Ave","zip":"00000"}}]}`,
		},
		{
			name: "apn and fips without property address",
			req:  oneItem(skiptrace.Item{APN: fixtureAPN, CountyFIPSCode: fixtureFIPS}),
			want: `{"requests":[{"apn":"000-000-0000","countyFipsCode":"00000"}]}`,
		},
		{
			name: "full item with every option",
			req: skiptrace.Request{
				Items: []skiptrace.Item{fixtureFullItem()},
				Options: skiptrace.Options{
					IncludeTCPABlacklistedPhones: new(true),
					PrioritizeMobilePhones:       new(false),
					DateFormat:                   skiptrace.DateFormatISODate,
				},
			},
			want: `{"requests":[{"requestId":"req-0001",` +
				`"propertyAddress":{"street":"100 Example Ave","city":"Testville","state":"ZZ","zip":"00000"},` +
				`"apn":"000-000-0000","county":"Example County","countyFipsCode":"00000","state":"ZZ",` +
				`"name":{"first":"Sentinel","last":"Person"},` +
				`"mailingAddress":{"street":"200 Sample St","city":"Testville","state":"ZZ","zip":"00000"}}],` +
				`"options":{"includeTCPABlacklistedPhones":true,"prioritizeMobilePhones":false,"dateFormat":"iso-date"}}`,
		},
		{
			name: "explicit false options are kept",
			req: skiptrace.Request{
				Items: []skiptrace.Item{{PropertyAddress: streetZip}},
				Options: skiptrace.Options{
					IncludeTCPABlacklistedPhones: new(false),
					PrioritizeMobilePhones:       new(false),
				},
			},
			want: `{"requests":[{"propertyAddress":{"street":"100 Example Ave","zip":"00000"}}],` +
				`"options":{"includeTCPABlacklistedPhones":false,"prioritizeMobilePhones":false}}`,
		},
		{
			name: "nil option is omitted",
			req: skiptrace.Request{
				Items:   []skiptrace.Item{{PropertyAddress: streetZip}},
				Options: skiptrace.Options{PrioritizeMobilePhones: new(true)},
			},
			want: `{"requests":[{"propertyAddress":{"street":"100 Example Ave","zip":"00000"}}],` +
				`"options":{"prioritizeMobilePhones":true}}`,
		},
		{
			name: "date format only",
			req: skiptrace.Request{
				Items:   []skiptrace.Item{{PropertyAddress: streetZip}},
				Options: skiptrace.Options{DateFormat: skiptrace.DateFormatISODateTime},
			},
			want: `{"requests":[{"propertyAddress":{"street":"100 Example Ave","zip":"00000"}}],` +
				`"options":{"dateFormat":"iso-date-time"}}`,
		},
		{
			name: "zero name and mailing address are omitted",
			req: oneItem(skiptrace.Item{
				RequestID:       "req-1",
				PropertyAddress: streetZip,
				Name:            skiptrace.Name{},
				MailingAddress:  skiptrace.Address{},
			}),
			want: `{"requests":[{"requestId":"req-1","propertyAddress":{"street":"100 Example Ave","zip":"00000"}}]}`,
		},
		{
			name: "partial objects keep only set fields",
			req: oneItem(skiptrace.Item{
				PropertyAddress: skiptrace.Address{Street: fixtureStreet, City: fixtureCity, State: fixtureState},
				Name:            skiptrace.Name{Last: fixtureLastName},
				MailingAddress:  skiptrace.Address{Street: fixtureMailingStreet},
			}),
			want: `{"requests":[{"propertyAddress":{"street":"100 Example Ave","city":"Testville","state":"ZZ"},` +
				`"name":{"last":"Person"},"mailingAddress":{"street":"200 Sample St"}}]}`,
		},
		{
			name: "padded values are sent as given",
			req: oneItem(skiptrace.Item{
				PropertyAddress: skiptrace.Address{Street: "  100 Example Ave\t", Zip: " 00000"},
			}),
			want: `{"requests":[{"propertyAddress":{"street":"  100 Example Ave\t","zip":" 00000"}}]}`,
		},
		{
			name: "items keep their order",
			req: skiptrace.Request{Items: []skiptrace.Item{
				{RequestID: "b", PropertyAddress: streetZip},
				{RequestID: "a", APN: fixtureAPN, County: fixtureCounty, State: fixtureState},
			}},
			want: `{"requests":[{"requestId":"b","propertyAddress":{"street":"100 Example Ave","zip":"00000"}},` +
				`{"requestId":"a","apn":"000-000-0000","county":"Example County","state":"ZZ"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.req)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestRequestMarshalOnlyEscapes(t *testing.T) {
	// A quote, a backslash, HTML characters, a control character, a tab,
	// non-ASCII text, a character outside the BMP and U+2028.
	const value = "q\"b\\s<g>a&c\x01t\tn\u00fc\U0001F600l\u2028e"

	t.Run("golden", func(t *testing.T) {
		req := oneItem(skiptrace.Item{
			PropertyAddress: skiptrace.Address{Street: fixtureStreet, Zip: fixtureZip},
			Name:            skiptrace.Name{First: value},
		})
		require.NoError(t, req.Validate())
		got, err := json.Marshal(req)
		require.NoError(t, err)
		want := `{"requests":[{"propertyAddress":{"street":"100 Example Ave","zip":"00000"},` +
			`"name":{"first":"q\"b\\s\u003cg\u003ea\u0026c\u0001t\tn` + "\u00fc\U0001F600" + `l\u2028e"}}]}`
		assert.Equal(t, want, string(got))
	})

	for _, f := range itemStringFields() {
		t.Run("round trip "+f.path, func(t *testing.T) {
			req := fixtureRequest(1)
			f.set(&req.Items[0], value)
			testutil.RequireJSONRoundTrip(t, req)
		})
	}
}

func TestRequestMarshalRoundTrip(t *testing.T) {
	req := fixtureRequest(3)
	req.Items[1] = skiptrace.Item{APN: fixtureAPN, CountyFIPSCode: fixtureFIPS}
	req.Options = skiptrace.Options{
		IncludeTCPABlacklistedPhones: new(false),
		PrioritizeMobilePhones:       new(true),
		DateFormat:                   skiptrace.DateFormatISODate,
	}
	testutil.RequireJSONRoundTrip(t, req)
}

func BenchmarkEncode100(b *testing.B) {
	req := fixtureRequest(skiptrace.MaxItems)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := json.Marshal(req); err != nil {
			b.Fatal(err)
		}
	}
}
