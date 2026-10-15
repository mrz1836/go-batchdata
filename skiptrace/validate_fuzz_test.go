package skiptrace_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/testutil"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

// validationMessages returns every Error string Validate may return for a
// one-item request. They are built only from field paths and fixed reasons.
func validationMessages() []string {
	stringFields := itemStringFields()
	fields := make([]string, 0, 3+len(stringFields))
	fields = append(fields, "Items", wantFieldItem0, "Options.DateFormat")
	for _, f := range stringFields {
		fields = append(fields, "Items[0]."+f.path)
	}
	reasons := []string{
		wantReasonNoItems, wantReasonTooManyItems, wantReasonInvalidUTF8, wantReasonBlank,
		wantReasonStreetRequired, wantReasonMailingStreet, wantReasonCountyFIPSCode,
		wantReasonNoLookupKey, wantReasonDateFormat,
	}
	msgs := make([]string, 0, len(fields)*len(reasons))
	for _, field := range fields {
		for _, reason := range reasons {
			e := &batchdata.ValidationError{Err: batchdata.ErrInvalidRequest, Field: field, Reason: reason}
			msgs = append(msgs, e.Error())
		}
	}
	return msgs
}

// FuzzValidate checks that Validate never panics, that every error it returns
// is a *batchdata.ValidationError matching ErrInvalidRequest whose text comes
// from the fixed vocabulary and holds no caller value, and that a request it
// accepts encodes to JSON without loss.
func FuzzValidate(f *testing.F) {
	const s = fixtureStreet
	f.Add("req-1", s, fixtureCity, fixtureState, fixtureZip, "", "", "", "", "", "", "", "", "", "", "")
	f.Add("", "", "", "", "", fixtureAPN, "", fixtureFIPS, "", "", "", "", "", "", "", "iso-date")
	f.Add("", "", "", "", "", fixtureAPN, fixtureCounty, "", fixtureState, "", "", "", "", "", "", "iso-date-time")
	f.Add("", " ", "", "", fixtureZip, "", "", "", "", "", "", "", "", "", "", "")
	f.Add("", s, "\xff", "", fixtureZip, "", "", "", "", "", "", "", "", "", "", "")
	f.Add("", s, "", "", fixtureZip, "", "", "1234a", "", "", "", "", "", "", "", "")
	f.Add("", s, "", "", fixtureZip, "", "", "\uff11\uff12\uff13\uff14\uff15", "", "", "", "", "", "", "", "")
	f.Add("", s, "", "", fixtureZip, "", "", "", "", "", "", "", fixtureCity, "", "", "")
	f.Add("", s, "", "", fixtureZip, "", "", "", "", "", "", "", "", "", "", "SENTINEL-VALUE-DATE")
	f.Add("SENTINEL-VALUE-ID", "SENTINEL-VALUE-STREET", "SENTINEL-VALUE-CITY", "", "", "SENTINEL-VALUE-APN",
		"SENTINEL-VALUE-COUNTY", "", "", "SENTINEL-VALUE-FIRST", "SENTINEL-VALUE-LAST", "SENTINEL-VALUE-MAIL",
		"", "", "", "")
	f.Add("", "is required when PropertyAddress is set", "", "", "", "", "", "", "", "", "", "", "", "", "", "")
	f.Add("", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "")

	messages := validationMessages()
	fromVocabulary := func(v string) bool {
		for _, m := range messages {
			if strings.Contains(m, v) {
				return true
			}
		}
		return false
	}

	f.Fuzz(func(t *testing.T, requestID, street, city, state, zip, apn, county, fips, itemState,
		first, last, mStreet, mCity, mState, mZip, dateFormat string,
	) {
		req := skiptrace.Request{
			Items: []skiptrace.Item{{
				RequestID:       requestID,
				PropertyAddress: skiptrace.Address{Street: street, City: city, State: state, Zip: zip},
				APN:             apn,
				County:          county,
				CountyFIPSCode:  fips,
				State:           itemState,
				Name:            skiptrace.Name{First: first, Last: last},
				MailingAddress:  skiptrace.Address{Street: mStreet, City: mCity, State: mState, Zip: mZip},
			}},
			Options: skiptrace.Options{DateFormat: skiptrace.DateFormat(dateFormat)},
		}

		err := req.Validate()
		if err == nil {
			testutil.RequireJSONRoundTrip(t, req, "a valid request must encode without loss")
			return
		}

		require.ErrorIs(t, err, batchdata.ErrInvalidRequest)
		var ve *batchdata.ValidationError
		require.ErrorAs(t, err, &ve)
		msg := err.Error()
		assert.Contains(t, messages, msg, "the error must come from the fixed vocabulary")
		for _, v := range []string{
			requestID, street, city, state, zip, apn, county, fips, itemState,
			first, last, mStreet, mCity, mState, mZip, dateFormat,
		} {
			if len(v) < 8 || fromVocabulary(v) {
				continue
			}
			assert.NotContains(t, msg, v, "the error must not contain a caller value")
		}
	})
}
