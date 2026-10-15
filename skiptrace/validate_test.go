package skiptrace_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

// wantFieldItem0 is the field path of the first item as a whole.
const wantFieldItem0 = "Items[0]"

// Expected validation reasons, pinned so a change to the wording is deliberate.
const (
	wantReasonNoItems        = "must contain at least 1 item"
	wantReasonTooManyItems   = "must contain at most 100 items"
	wantReasonInvalidUTF8    = "must be valid UTF-8"
	wantReasonBlank          = "must not be only whitespace"
	wantReasonStreetRequired = "is required when PropertyAddress is set"
	wantReasonMailingStreet  = "is required when MailingAddress is set"
	wantReasonCountyFIPSCode = "must be exactly 5 ASCII digits"
	wantReasonNoLookupKey    = "must include one of: PropertyAddress.Street and PropertyAddress.Zip; " +
		"PropertyAddress.Street, PropertyAddress.City and PropertyAddress.State; " +
		"APN and CountyFIPSCode; APN, County and State"
	wantReasonDateFormat = "must be empty, DateFormatISODateTime or DateFormatISODate"
)

// itemStringField names one string field of an Item and sets it.
type itemStringField struct {
	path string
	set  func(it *skiptrace.Item, v string)
}

// itemStringFields returns every string field of an Item, in declaration
// order, with its path relative to the item.
func itemStringFields() []itemStringField {
	return []itemStringField{
		{"RequestID", func(it *skiptrace.Item, v string) { it.RequestID = v }},
		{"PropertyAddress.Street", func(it *skiptrace.Item, v string) { it.PropertyAddress.Street = v }},
		{"PropertyAddress.City", func(it *skiptrace.Item, v string) { it.PropertyAddress.City = v }},
		{"PropertyAddress.State", func(it *skiptrace.Item, v string) { it.PropertyAddress.State = v }},
		{"PropertyAddress.Zip", func(it *skiptrace.Item, v string) { it.PropertyAddress.Zip = v }},
		{"APN", func(it *skiptrace.Item, v string) { it.APN = v }},
		{"County", func(it *skiptrace.Item, v string) { it.County = v }},
		{"CountyFIPSCode", func(it *skiptrace.Item, v string) { it.CountyFIPSCode = v }},
		{"State", func(it *skiptrace.Item, v string) { it.State = v }},
		{"Name.First", func(it *skiptrace.Item, v string) { it.Name.First = v }},
		{"Name.Last", func(it *skiptrace.Item, v string) { it.Name.Last = v }},
		{"MailingAddress.Street", func(it *skiptrace.Item, v string) { it.MailingAddress.Street = v }},
		{"MailingAddress.City", func(it *skiptrace.Item, v string) { it.MailingAddress.City = v }},
		{"MailingAddress.State", func(it *skiptrace.Item, v string) { it.MailingAddress.State = v }},
		{"MailingAddress.Zip", func(it *skiptrace.Item, v string) { it.MailingAddress.Zip = v }},
	}
}

// oneItem returns a request holding only it.
func oneItem(it skiptrace.Item) skiptrace.Request {
	return skiptrace.Request{Items: []skiptrace.Item{it}}
}

// requireInvalidRequest checks that err is exactly a *batchdata.ValidationError
// for ErrInvalidRequest with the given field and reason.
func requireInvalidRequest(t *testing.T, err error, field, reason string) {
	t.Helper()
	require.Error(t, err)
	require.ErrorIs(t, err, batchdata.ErrInvalidRequest)
	var ve *batchdata.ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Same(t, ve, err, "the error must be the *ValidationError itself, not a wrapper")
	assert.Same(t, batchdata.ErrInvalidRequest, ve.Err)
	assert.Equal(t, field, ve.Field)
	assert.Equal(t, reason, ve.Reason)
	want := &batchdata.ValidationError{Err: batchdata.ErrInvalidRequest, Field: field, Reason: reason}
	assert.Equal(t, want.Error(), err.Error())
}

func TestRequestValidateAcceptsDocumentedCombinations(t *testing.T) {
	tests := []struct {
		name string
		item skiptrace.Item
	}{
		{"street city state zip", skiptrace.Item{PropertyAddress: skiptrace.Address{
			Street: fixtureStreet, City: fixtureCity, State: fixtureState, Zip: fixtureZip,
		}}},
		{"street zip", skiptrace.Item{PropertyAddress: skiptrace.Address{Street: fixtureStreet, Zip: fixtureZip}}},
		{"street city state", skiptrace.Item{PropertyAddress: skiptrace.Address{
			Street: fixtureStreet, City: fixtureCity, State: fixtureState,
		}}},
		{"apn county state", skiptrace.Item{APN: fixtureAPN, County: fixtureCounty, State: fixtureState}},
		{"apn fips", skiptrace.Item{APN: fixtureAPN, CountyFIPSCode: fixtureFIPS}},
		{"every field set", fixtureFullItem()},
		{"street zip with name and mailing address", skiptrace.Item{
			PropertyAddress: skiptrace.Address{Street: fixtureStreet, Zip: fixtureZip},
			Name:            skiptrace.Name{First: fixtureFirstName, Last: fixtureLastName},
			MailingAddress:  skiptrace.Address{Street: fixtureMailingStreet},
		}},
		{"apn fips with county", skiptrace.Item{APN: fixtureAPN, CountyFIPSCode: fixtureFIPS, County: fixtureCounty}},
		{"apn fips with partial property address", skiptrace.Item{
			APN: fixtureAPN, CountyFIPSCode: fixtureFIPS,
			PropertyAddress: skiptrace.Address{Street: fixtureStreet, City: fixtureCity},
		}},
		{"street zip with apn only", skiptrace.Item{
			PropertyAddress: skiptrace.Address{Street: fixtureStreet, Zip: fixtureZip}, APN: fixtureAPN,
		}},
		{"request id with combination", skiptrace.Item{
			RequestID: "req-1", APN: fixtureAPN, County: fixtureCounty, State: fixtureState,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, oneItem(tt.item).Validate())
		})
	}
}

func TestRequestValidateRejectsPartialCombinations(t *testing.T) {
	tests := []struct {
		name   string
		item   skiptrace.Item
		field  string
		reason string
	}{
		{
			"empty item",
			skiptrace.Item{},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"street only",
			skiptrace.Item{PropertyAddress: skiptrace.Address{Street: fixtureStreet}},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"street city",
			skiptrace.Item{PropertyAddress: skiptrace.Address{Street: fixtureStreet, City: fixtureCity}},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"street state",
			skiptrace.Item{PropertyAddress: skiptrace.Address{Street: fixtureStreet, State: fixtureState}},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"street with item level state",
			skiptrace.Item{
				PropertyAddress: skiptrace.Address{Street: fixtureStreet, City: fixtureCity}, State: fixtureState,
			},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"city state zip without street",
			skiptrace.Item{PropertyAddress: skiptrace.Address{
				City: fixtureCity, State: fixtureState, Zip: fixtureZip,
			}},
			"Items[0].PropertyAddress.Street", wantReasonStreetRequired,
		},
		{
			"zip only",
			skiptrace.Item{PropertyAddress: skiptrace.Address{Zip: fixtureZip}},
			"Items[0].PropertyAddress.Street", wantReasonStreetRequired,
		},
		{
			"apn only",
			skiptrace.Item{APN: fixtureAPN},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"apn county without state",
			skiptrace.Item{APN: fixtureAPN, County: fixtureCounty},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"apn state without county",
			skiptrace.Item{APN: fixtureAPN, State: fixtureState},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"apn county with state only on property address",
			skiptrace.Item{
				APN: fixtureAPN, County: fixtureCounty,
				PropertyAddress: skiptrace.Address{Street: fixtureStreet, State: fixtureState},
			},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"county state without apn",
			skiptrace.Item{County: fixtureCounty, State: fixtureState},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"fips without apn",
			skiptrace.Item{CountyFIPSCode: fixtureFIPS},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"name and mailing address only",
			skiptrace.Item{
				Name: skiptrace.Name{First: fixtureFirstName, Last: fixtureLastName},
				MailingAddress: skiptrace.Address{
					Street: fixtureMailingStreet, City: fixtureCity, State: fixtureState, Zip: fixtureZip,
				},
			},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"name only",
			skiptrace.Item{Name: skiptrace.Name{First: fixtureFirstName}},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"request id only",
			skiptrace.Item{RequestID: "req-1"},
			wantFieldItem0, wantReasonNoLookupKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireInvalidRequest(t, oneItem(tt.item).Validate(), tt.field, tt.reason)
		})
	}
}

func TestRequestValidateRejectsBlankStrings(t *testing.T) {
	blanks := []struct {
		name  string
		value string
	}{
		{"space", " "},
		{"tab newline", "\t\n"},
		{"ideographic space", "\u3000"},
		{"no break space", "\u00a0"},
		{"carriage return vertical tab form feed", "\r\v\f"},
		{"mixed", " \u00a0\u3000 "},
	}
	for _, f := range itemStringFields() {
		for _, b := range blanks {
			t.Run(f.path+" "+b.name, func(t *testing.T) {
				it := fixtureFullItem()
				f.set(&it, b.value)
				requireInvalidRequest(t, oneItem(it).Validate(), "Items[0]."+f.path, wantReasonBlank)
			})
		}
	}
}

func TestRequestValidateRejectsInvalidUTF8(t *testing.T) {
	invalid := []struct {
		name  string
		value string
	}{
		{"lone byte", "\xff"},
		{"inside text", "100 Example\xffAve"},
		{"truncated sequence", "Testville\xc3"},
		{"surrogate half", "\xed\xa0\x80"},
		{"between spaces", " \xff "},
	}
	for _, f := range itemStringFields() {
		for _, v := range invalid {
			t.Run(f.path+" "+v.name, func(t *testing.T) {
				it := fixtureFullItem()
				f.set(&it, v.value)
				requireInvalidRequest(t, oneItem(it).Validate(), "Items[0]."+f.path, wantReasonInvalidUTF8)
			})
		}
	}
}

func TestRequestValidateAcceptsEmptyAndPaddedStrings(t *testing.T) {
	t.Run("empty optional fields", func(t *testing.T) {
		for _, f := range itemStringFields() {
			switch f.path {
			case "PropertyAddress.Street", "PropertyAddress.Zip", "MailingAddress.Street":
				continue // required by the remaining fields
			}
			it := fixtureFullItem()
			f.set(&it, "")
			require.NoError(t, oneItem(it).Validate(), f.path)
		}
	})
	t.Run("padded values", func(t *testing.T) {
		for _, f := range itemStringFields() {
			if f.path == "CountyFIPSCode" {
				continue // must be exactly 5 digits
			}
			it := fixtureFullItem()
			f.set(&it, " \tpadded value\u3000")
			require.NoError(t, oneItem(it).Validate(), f.path)
		}
	})
}

func TestRequestValidateRequiresStreetOnSetAddress(t *testing.T) {
	// Every item below has APN and CountyFIPSCode, so only the address is wrong.
	base := func(it skiptrace.Item) skiptrace.Item {
		it.APN = fixtureAPN
		it.CountyFIPSCode = fixtureFIPS
		return it
	}
	tests := []struct {
		name   string
		item   skiptrace.Item
		field  string
		reason string
	}{
		{
			"property address city only", base(skiptrace.Item{PropertyAddress: skiptrace.Address{City: fixtureCity}}),
			"Items[0].PropertyAddress.Street", wantReasonStreetRequired,
		},
		{
			"property address state only", base(skiptrace.Item{PropertyAddress: skiptrace.Address{State: fixtureState}}),
			"Items[0].PropertyAddress.Street", wantReasonStreetRequired,
		},
		{
			"property address zip only", base(skiptrace.Item{PropertyAddress: skiptrace.Address{Zip: fixtureZip}}),
			"Items[0].PropertyAddress.Street", wantReasonStreetRequired,
		},
		{
			"mailing address city only", base(skiptrace.Item{MailingAddress: skiptrace.Address{City: fixtureCity}}),
			"Items[0].MailingAddress.Street", wantReasonMailingStreet,
		},
		{
			"mailing address without street", base(skiptrace.Item{MailingAddress: skiptrace.Address{
				City: fixtureCity, State: fixtureState, Zip: fixtureZip,
			}}),
			"Items[0].MailingAddress.Street", wantReasonMailingStreet,
		},
		{
			"both addresses without street", base(skiptrace.Item{
				PropertyAddress: skiptrace.Address{Zip: fixtureZip},
				MailingAddress:  skiptrace.Address{Zip: fixtureZip},
			}),
			"Items[0].PropertyAddress.Street", wantReasonStreetRequired,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireInvalidRequest(t, oneItem(tt.item).Validate(), tt.field, tt.reason)
		})
	}
}

func TestRequestValidateCountyFIPSCode(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{"zeros", "00000", true},
		{"digits", "12345", true},
		{"four digits", "1234", false},
		{"six digits", "123456", false},
		{"letter", "1234a", false},
		{"fullwidth digits", "\uff11\uff12\uff13\uff14\uff15", false},
		{"arabic indic digits", "\u0660\u0661\u0662\u0663\u0664", false},
		{"sign", "+1234", false},
		{"inner space", "12 45", false},
		{"leading space", " 12345", false},
		{"trailing newline", "12345\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Street and Zip satisfy a combination, so the code is checked even
			// when it is not needed.
			for _, it := range []skiptrace.Item{
				{APN: fixtureAPN, CountyFIPSCode: tt.value},
				{PropertyAddress: skiptrace.Address{Street: fixtureStreet, Zip: fixtureZip}, CountyFIPSCode: tt.value},
			} {
				err := oneItem(it).Validate()
				if tt.valid {
					require.NoError(t, err)
					continue
				}
				requireInvalidRequest(t, err, "Items[0].CountyFIPSCode", wantReasonCountyFIPSCode)
			}
		})
	}
}

func TestRequestValidateDateFormat(t *testing.T) {
	tests := []struct {
		name   string
		format skiptrace.DateFormat
		valid  bool
	}{
		{"empty", "", true},
		{"iso date time", skiptrace.DateFormatISODateTime, true},
		{"iso date", skiptrace.DateFormatISODate, true},
		{"unknown", "unix-epoch", false},
		{"upper case", "ISO-DATE", false},
		{"trailing space", "iso-date ", false},
		{"prefix", "iso", false},
		{"blank", " ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := fixtureRequest(1)
			req.Options.DateFormat = tt.format
			err := req.Validate()
			if tt.valid {
				require.NoError(t, err)
				return
			}
			requireInvalidRequest(t, err, "Options.DateFormat", wantReasonDateFormat)
		})
	}
}

func TestRequestValidateItemCount(t *testing.T) {
	assert.Equal(t, 100, skiptrace.MaxItems)
	assert.Contains(t, wantReasonTooManyItems, strconv.Itoa(skiptrace.MaxItems))

	t.Run("nil items", func(t *testing.T) {
		requireInvalidRequest(t, skiptrace.Request{}.Validate(), "Items", wantReasonNoItems)
	})
	t.Run("empty items", func(t *testing.T) {
		req := skiptrace.Request{Items: []skiptrace.Item{}}
		requireInvalidRequest(t, req.Validate(), "Items", wantReasonNoItems)
	})
	t.Run("empty items with bad date format", func(t *testing.T) {
		req := skiptrace.Request{Options: skiptrace.Options{DateFormat: "bad"}}
		requireInvalidRequest(t, req.Validate(), "Items", wantReasonNoItems)
	})
	t.Run("one item", func(t *testing.T) {
		require.NoError(t, fixtureRequest(1).Validate())
	})
	t.Run("max items", func(t *testing.T) {
		require.NoError(t, fixtureRequest(skiptrace.MaxItems).Validate())
	})
	t.Run("too many items reports only the count", func(t *testing.T) {
		req := fixtureRequest(skiptrace.MaxItems + 1)
		req.Items[0] = skiptrace.Item{}
		req.Items[50].PropertyAddress.Street = " "
		req.Options.DateFormat = "bad"
		requireInvalidRequest(t, req.Validate(), "Items", wantReasonTooManyItems)
	})
}

func TestRequestValidateAllowsDuplicateRequestIDs(t *testing.T) {
	req := fixtureRequest(3)
	for i := range req.Items {
		req.Items[i].RequestID = "req-same"
	}
	require.NoError(t, req.Validate())
}

func TestRequestValidateReportsFirstProblem(t *testing.T) {
	tests := []struct {
		name   string
		build  func() skiptrace.Request
		field  string
		reason string
	}{
		{
			"two bad items report index 0",
			func() skiptrace.Request {
				req := fixtureRequest(2)
				req.Items[0] = skiptrace.Item{}
				req.Items[1].PropertyAddress.Street = " "
				return req
			},
			wantFieldItem0, wantReasonNoLookupKey,
		},
		{
			"first bad item after a good one",
			func() skiptrace.Request {
				req := fixtureRequest(3)
				req.Items[1].Name.Last = "\t"
				req.Items[2].CountyFIPSCode = "1"
				return req
			},
			"Items[1].Name.Last", wantReasonBlank,
		},
		{
			"bad item before bad date format",
			func() skiptrace.Request {
				req := fixtureRequest(2)
				req.Items[1].CountyFIPSCode = "1"
				req.Options.DateFormat = "bad"
				return req
			},
			"Items[1].CountyFIPSCode", wantReasonCountyFIPSCode,
		},
		{
			"fields in declaration order",
			func() skiptrace.Request {
				req := fixtureRequest(1)
				req.Items[0].MailingAddress.Zip = " "
				req.Items[0].RequestID = " "
				return req
			},
			"Items[0].RequestID", wantReasonBlank,
		},
		{
			"invalid utf8 before blank in one field",
			func() skiptrace.Request {
				req := fixtureRequest(1)
				req.Items[0].County = " \xff "
				return req
			},
			"Items[0].County", wantReasonInvalidUTF8,
		},
		{
			"string problems before a missing street",
			func() skiptrace.Request {
				return oneItem(skiptrace.Item{
					PropertyAddress: skiptrace.Address{City: fixtureCity},
					Name:            skiptrace.Name{Last: " "},
				})
			},
			"Items[0].Name.Last", wantReasonBlank,
		},
		{
			"missing street before bad fips",
			func() skiptrace.Request {
				return oneItem(skiptrace.Item{
					PropertyAddress: skiptrace.Address{City: fixtureCity},
					CountyFIPSCode:  "1",
				})
			},
			"Items[0].PropertyAddress.Street", wantReasonStreetRequired,
		},
		{
			"bad fips before missing combination",
			func() skiptrace.Request {
				return oneItem(skiptrace.Item{CountyFIPSCode: "1"})
			},
			"Items[0].CountyFIPSCode", wantReasonCountyFIPSCode,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireInvalidRequest(t, tt.build().Validate(), tt.field, tt.reason)
		})
	}
}

func TestRequestValidateErrorOmitsValues(t *testing.T) {
	tests := []struct {
		name  string
		build func() skiptrace.Request
	}{
		{
			"bad fips",
			func() skiptrace.Request {
				req := fixtureRequest(1)
				req.Items[0].RequestID = "SENTINEL-VALUE-ID"
				req.Items[0].CountyFIPSCode = "SENTINEL-VALUE-FIPS"
				return req
			},
		},
		{
			"invalid utf8",
			func() skiptrace.Request {
				req := fixtureRequest(1)
				req.Items[0].PropertyAddress.Street = "SENTINEL-VALUE-STREET\xff"
				return req
			},
		},
		{
			"missing combination",
			func() skiptrace.Request {
				return oneItem(skiptrace.Item{
					RequestID:       "SENTINEL-VALUE-ID",
					PropertyAddress: skiptrace.Address{Street: "SENTINEL-VALUE-STREET"},
					Name:            skiptrace.Name{First: "SENTINEL-VALUE-FIRST", Last: "SENTINEL-VALUE-LAST"},
				})
			},
		},
		{
			"missing street",
			func() skiptrace.Request {
				return oneItem(skiptrace.Item{
					APN: "SENTINEL-VALUE-APN", CountyFIPSCode: fixtureFIPS,
					MailingAddress: skiptrace.Address{City: "SENTINEL-VALUE-CITY"},
				})
			},
		},
		{
			"bad date format",
			func() skiptrace.Request {
				req := fixtureRequest(1)
				req.Items[0].Name.First = "SENTINEL-VALUE-FIRST"
				req.Options.DateFormat = "SENTINEL-VALUE-DATE"
				return req
			},
		},
		{
			"too many items",
			func() skiptrace.Request {
				req := fixtureRequest(skiptrace.MaxItems + 1)
				for i := range req.Items {
					req.Items[i].RequestID = "SENTINEL-VALUE-" + strconv.Itoa(i)
				}
				return req
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.build().Validate()
			require.Error(t, err)
			require.ErrorIs(t, err, batchdata.ErrInvalidRequest)
			assert.NotContains(t, err.Error(), "SENTINEL")
			var ve *batchdata.ValidationError
			require.ErrorAs(t, err, &ve)
			assert.NotContains(t, ve.Field, "SENTINEL")
			assert.NotContains(t, ve.Reason, "SENTINEL")
		})
	}
}

func TestRequestValidateDoesNotModifyRequest(t *testing.T) {
	req := fixtureRequest(2)
	req.Items[0].PropertyAddress.Street = "  " + fixtureStreet + "  "
	req.Items[1].Name.First = "\u3000Sentinel\u3000"
	req.Options.IncludeTCPABlacklistedPhones = new(false)

	want := fixtureRequest(2)
	want.Items[0].PropertyAddress.Street = "  " + fixtureStreet + "  "
	want.Items[1].Name.First = "\u3000Sentinel\u3000"
	want.Options.IncludeTCPABlacklistedPhones = new(false)

	require.NoError(t, req.Validate())
	assert.Equal(t, want, req)
}

func BenchmarkValidate100(b *testing.B) {
	req := fixtureRequest(skiptrace.MaxItems)
	b.ReportAllocs()
	for b.Loop() {
		if err := req.Validate(); err != nil {
			b.Fatal(err)
		}
	}
}
