package skiptrace_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/testutil"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

// wantFullRecord is fixtureFullRecord decoded.
func wantFullRecord() fixtureWireRecord {
	return fixtureWireRecord{
		Input: skiptrace.Input{
			RequestID: "req-0001",
			PropertyAddress: &skiptrace.InputAddress{
				Hash:              "hash-input-0001",
				AddressValidity:   skiptrace.AddressValid,
				Street:            "100 Example Ave Apt 1",
				City:              fixtureCity,
				State:             "ZZ",
				Zip:               "00000",
				ZipPlus4:          "0001",
				County:            "Example County",
				CountyFIPSCode:    "00000",
				HouseNumber:       "100",
				UnitType:          "Apt",
				UnitNumber:        "1",
				StreetNoUnit:      fixtureStreet,
				FormattedStreet:   "100 EXAMPLE AVE APT 1",
				DeliveryPointCode: "01",
				DPVMatchCode:      "Y",
				DPVFootnotes:      "AAZZ",
				GeoStatus:         "TEST",
				GeoStatusCode:     "T1",
				CityAliases:       []string{fixtureCity, "Test Village"},
				OldHashes:         []string{"hash-old-0001", "hash-old-0002"},
				Localities:        []string{"Testville Heights", "Testville Center"},
				Latitude:          new(0.5),
				Longitude:         new(-0.25),
				Normalized:        new(true),
			},
			MailingAddress: &skiptrace.InputAddress{
				Street: "200 Sample St", City: fixtureCity, State: "ZZ", Zip: "00000", Normalized: new(false),
			},
			Name:           &skiptrace.Name{First: "Sentinel", Last: "Person"},
			APN:            "000-000-0000",
			CountyFIPSCode: "00000",
			County:         "Example County",
			State:          "ZZ",
		},
		Property: &skiptrace.Property{
			ID: "prop-0001",
			Address: &skiptrace.PostalAddress{
				Hash:            "hash-prop-0001",
				AddressValidity: skiptrace.AddressValid,
				FullAddress:     "100 Example Ave Apt 1, Testville, ZZ 00000",
				Street:          "100 Example Ave Apt 1",
				City:            fixtureCity,
				County:          "Example County",
				State:           "ZZ",
				Zip:             "00000",
				ZipPlus4:        "0001",
			},
			MailingAddress: &skiptrace.PostalAddress{
				Hash:            "hash-mail-0001",
				AddressValidity: skiptrace.AddressPartiallyValid,
				FullAddress:     "200 Sample St, Testville, ZZ 00000",
				Street:          "200 Sample St",
				City:            fixtureCity,
				County:          "Example County",
				State:           "ZZ",
				Zip:             "00000",
				ZipPlus4:        "0002",
			},
			Owners: []skiptrace.Owner{
				{Name: skiptrace.PersonName{
					First: "Sentinel", Middle: "Q", Last: "Person", Full: "Sentinel Q Person",
					AKAs: []skiptrace.AKA{{First: "Senti", Last: "Person", Full: "Senti Person", Rank: 1}},
				}},
				{Name: skiptrace.PersonName{Full: "Example Holdings Trust"}},
			},
		},
		Persons: []skiptrace.Person{
			{
				PropertyOwner: new(true),
				Name: skiptrace.PersonName{
					First: "Sentinel", Middle: "Q", Last: "Person", Full: "Sentinel Q Person",
					AKAs: []skiptrace.AKA{
						{First: "Senti", Last: "Person", Full: "Senti Person", Rank: 1},
						{First: "Sentinel", Middle: "Quill", Last: "Persona", Full: "Sentinel Quill Persona", Rank: 2},
					},
				},
				Addresses: []skiptrace.PersonAddress{
					{
						PostalAddress: skiptrace.PostalAddress{
							Hash:            "hash-addr-0001",
							AddressValidity: skiptrace.AddressValid,
							FullAddress:     "100 Example Ave Apt 1, Testville, ZZ 00000",
							Street:          "100 Example Ave Apt 1",
							City:            fixtureCity,
							County:          "Example County",
							State:           "ZZ",
							Zip:             "00000",
							ZipPlus4:        "0001",
						},
						Rank:                   1,
						PropertyMailingAddress: new(false),
					},
					{
						PostalAddress: skiptrace.PostalAddress{
							Hash:            "hash-addr-0002",
							AddressValidity: skiptrace.AddressInvalid,
							FullAddress:     "200 Sample St, Testville, ZZ 00000",
							Street:          "200 Sample St",
							City:            fixtureCity,
							County:          "Example County",
							State:           "ZZ",
							Zip:             "00000",
							ZipPlus4:        "0002",
						},
						Rank:                   2,
						PropertyMailingAddress: new(true),
					},
				},
				Phones: []skiptrace.Phone{
					{
						Rank: 1, Number: "2025550101", Type: skiptrace.PhoneTypeMobile, Carrier: "Example Wireless",
						Tested: new(true), Reachable: new(true), DNC: new(false), TCPA: new(false),
					},
					{
						Rank: 2, Number: "2025550102", Type: skiptrace.PhoneTypeLandLine, Carrier: "Example Telco",
						Tested: new(false), Reachable: new(false), DNC: new(true), TCPA: new(true),
					},
					{
						Rank: 3, Number: "2025550103", Type: skiptrace.PhoneTypeVoIP, Carrier: "Example Voice",
						Tested: new(true), Reachable: new(false), DNC: new(false),
					},
				},
				Emails: []skiptrace.Email{
					{Rank: 1, Email: "sentinel.person@example.com", Tested: new(true)},
					{Rank: 2, Email: "sentinel.q@example.test", Tested: new(false)},
				},
				Litigator: new(false),
				Deceased:  new(false),
				DOB:       new("1901-02-03T00:00:00.000Z"),
			},
			{
				PropertyOwner: new(false),
				Name:          skiptrace.PersonName{First: "Sample", Last: "Resident", Full: "Sample Resident"},
				Addresses: []skiptrace.PersonAddress{{
					PostalAddress: skiptrace.PostalAddress{
						FullAddress: "300 Placeholder Rd, Testville, ZZ 00000",
						Street:      "300 Placeholder Rd",
						City:        fixtureCity,
						State:       "ZZ",
						Zip:         "00000",
					},
					Rank: 1,
				}},
				Phones: []skiptrace.Phone{{
					Rank: 1, Number: "2025550104", Type: skiptrace.PhoneTypeMobile, Carrier: "Example Wireless",
					Tested: new(true), Reachable: new(true), DNC: new(false),
				}},
				Emails:    []skiptrace.Email{{Rank: 1, Email: "sample.resident@example.com"}},
				Litigator: new(true),
				Deceased:  new(true),
				DOB:       new("1902-03-04T00:00:00.000Z"),
			},
		},
		Meta: skiptrace.RecordMeta{Matched: new(true), Error: new(false)},
	}
}

// assertNoZeroFields checks that every field of the struct v points to or
// holds is set, so a fixture keeps covering a type when the type gains a
// field.
func assertNoZeroFields(t *testing.T, name string, v any) {
	t.Helper()
	rv := reflect.Indirect(reflect.ValueOf(v))
	require.Equal(t, reflect.Struct, rv.Kind(), name)
	for i := range rv.NumField() {
		assert.False(t, rv.Field(i).IsZero(), "%s.%s is not set by the fixture", name, rv.Type().Field(i).Name)
	}
}

func TestRecordDecodeFullFixture(t *testing.T) {
	got := decodeFixtureRecord(t, fixtureFullRecord)
	assert.Equal(t, wantFullRecord(), got)

	assert.Equal(t, 25, reflect.TypeFor[skiptrace.InputAddress]().NumField())
	require.NotNil(t, got.Property)
	require.NotEmpty(t, got.Persons)
	person := got.Persons[0]
	require.NotEmpty(t, person.Addresses)
	require.NotEmpty(t, person.Phones)
	require.NotEmpty(t, person.Emails)
	require.Len(t, person.Name.AKAs, 2)

	for name, v := range map[string]any{
		"Input":                    got.Input,
		"Input.PropertyAddress":    got.Input.PropertyAddress,
		"Property":                 got.Property,
		"Property.Address":         got.Property.Address,
		"Property.MailingAddress":  got.Property.MailingAddress,
		"Person":                   person,
		"Person.Name":              person.Name,
		"Person.Name.AKAs[1]":      person.Name.AKAs[1],
		"Person.Addresses[0]":      person.Addresses[0],
		"Person.Addresses[0].Post": person.Addresses[0].PostalAddress,
		"Person.Phones[0]":         person.Phones[0],
		"Person.Emails[0]":         person.Emails[0],
		"Meta":                     got.Meta,
	} {
		assertNoZeroFields(t, name, v)
	}
}

func TestRecordDecodeFixtures(t *testing.T) {
	streetZip := &skiptrace.InputAddress{Street: fixtureStreet, Zip: "00000"}
	nullsOrAbsent := fixtureWireRecord{
		Input: skiptrace.Input{PropertyAddress: streetZip},
		Persons: []skiptrace.Person{{
			Name:   skiptrace.PersonName{First: "Sentinel", Last: "Person"},
			Phones: []skiptrace.Phone{{Number: "2025550105"}},
		}},
		Meta: skiptrace.RecordMeta{Matched: new(true)},
	}
	tests := []struct {
		name    string
		fixture string
		want    fixtureWireRecord
	}{
		{
			name:    "no match",
			fixture: fixtureNoMatchRecord,
			want: fixtureWireRecord{
				Input: skiptrace.Input{RequestID: "req-0002", PropertyAddress: &skiptrace.InputAddress{
					Street: "999 Nowhere Ln", City: fixtureCity, State: "ZZ", Zip: "00000",
				}},
				Persons: []skiptrace.Person{},
				Meta:    skiptrace.RecordMeta{Matched: new(false), Error: new(false)},
			},
		},
		{
			name:    "error",
			fixture: fixtureErrorRecord,
			want: fixtureWireRecord{
				Input: skiptrace.Input{RequestID: "req-0003", APN: "000-000-0000", CountyFIPSCode: "00000"},
				Meta:  skiptrace.RecordMeta{Matched: new(false), Error: new(true)},
			},
		},
		{
			name:    "matched flag absent",
			fixture: fixtureUnreportedRecord,
			want: fixtureWireRecord{
				Input:   skiptrace.Input{RequestID: "req-0004", PropertyAddress: streetZip},
				Persons: []skiptrace.Person{{Name: skiptrace.PersonName{First: "Sentinel", Last: "Person"}}},
				Meta:    skiptrace.RecordMeta{Error: new(false)},
			},
		},
		{name: "explicit nulls", fixture: fixtureNullsRecord, want: nullsOrAbsent},
		{name: "absent fields", fixture: fixtureAbsentRecord, want: nullsOrAbsent},
		{
			name:    "unknown enum values",
			fixture: fixtureUnknownEnumsRecord,
			want: fixtureWireRecord{
				Input: skiptrace.Input{PropertyAddress: &skiptrace.InputAddress{
					AddressValidity: "Unknown", Street: fixtureStreet, Zip: "00000",
				}},
				Property: &skiptrace.Property{
					Address: &skiptrace.PostalAddress{AddressValidity: "Unknown", Street: fixtureStreet},
				},
				Persons: []skiptrace.Person{{
					Addresses: []skiptrace.PersonAddress{{
						PostalAddress: skiptrace.PostalAddress{AddressValidity: "Unknown", Street: fixtureStreet},
					}},
					Phones: []skiptrace.Phone{{Rank: 1, Number: "2025550106", Type: "Satellite"}},
				}},
				Meta: skiptrace.RecordMeta{Matched: new(true), Error: new(false)},
			},
		},
		{
			name:    "more entries than documented",
			fixture: fixtureOverMaximaRecord,
			want: fixtureWireRecord{
				Input: skiptrace.Input{PropertyAddress: &skiptrace.InputAddress{
					Street:      fixtureStreet,
					Zip:         "00000",
					CityAliases: []string{"Alias One", "Alias Two", "Alias Three", "Alias Four"},
					OldHashes:   []string{"old-1", "old-2", "old-3"},
					Localities:  []string{"Locality 1", "Locality 2", "Locality 3"},
				}},
				Persons: []skiptrace.Person{
					{
						Name: skiptrace.PersonName{Full: "Sentinel Person", AKAs: []skiptrace.AKA{
							{Full: "Aka One", Rank: 1},
							{Full: "Aka Two", Rank: 2},
							{Full: "Aka Three", Rank: 3},
							{Full: "Aka Four", Rank: 4},
							{Full: "Aka Five", Rank: 5},
						}},
						Addresses: []skiptrace.PersonAddress{
							{PostalAddress: skiptrace.PostalAddress{Street: "101 Example Ave"}, Rank: 1},
							{PostalAddress: skiptrace.PostalAddress{Street: "102 Example Ave"}, Rank: 2},
							{PostalAddress: skiptrace.PostalAddress{Street: "103 Example Ave"}, Rank: 3},
							{PostalAddress: skiptrace.PostalAddress{Street: "104 Example Ave"}, Rank: 4},
							{PostalAddress: skiptrace.PostalAddress{Street: "105 Example Ave"}, Rank: 5},
						},
						Phones: []skiptrace.Phone{
							{Rank: 1, Number: "2025550111"},
							{Rank: 2, Number: "2025550112"},
							{Rank: 3, Number: "2025550113"},
							{Rank: 4, Number: "2025550114"},
							{Rank: 5, Number: "2025550115"},
							{Rank: 6, Number: "2025550116"},
							{Rank: 7, Number: "2025550117"},
						},
						Emails: []skiptrace.Email{
							{Rank: 1, Email: "one@example.com"},
							{Rank: 2, Email: "two@example.com"},
							{Rank: 3, Email: "three@example.test"},
							{Rank: 4, Email: "four@example.test"},
							{Rank: 5, Email: "five@example.com"},
						},
					},
					{Name: skiptrace.PersonName{Full: "Second Person"}},
					{Name: skiptrace.PersonName{Full: "Third Person"}},
					{Name: skiptrace.PersonName{Full: "Fourth Person"}},
				},
				Meta: skiptrace.RecordMeta{Matched: new(true), Error: new(false)},
			},
		},
		{
			name:    "unknown fields are ignored",
			fixture: fixtureUnknownFieldsRecord,
			want: fixtureWireRecord{
				Input:    skiptrace.Input{RequestID: "req-0007"},
				Property: &skiptrace.Property{ID: "prop-0007"},
				Persons: []skiptrace.Person{{
					Name:   skiptrace.PersonName{First: "Sentinel"},
					Phones: []skiptrace.Phone{{Number: "2025550107"}},
				}},
				Meta: skiptrace.RecordMeta{Matched: new(true)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, decodeFixtureRecord(t, tt.fixture))
		})
	}
}

func TestRecordDecodePersonsEmptyVersusAbsent(t *testing.T) {
	t.Run("empty array is empty and non-nil", func(t *testing.T) {
		got := decodeFixtureRecord(t, fixtureNoMatchRecord)
		assert.NotNil(t, got.Persons)
		assert.Empty(t, got.Persons)
		assert.Nil(t, got.Property)
	})
	t.Run("absent is nil", func(t *testing.T) {
		got := decodeFixtureRecord(t, fixtureErrorRecord)
		assert.Nil(t, got.Persons)
	})
	t.Run("null is nil", func(t *testing.T) {
		got := decodeFixtureRecord(t, `{"persons": null}`)
		assert.Nil(t, got.Persons)
	})
}

func TestWarningsDecodeKeepsUnknownCode(t *testing.T) {
	var got []batchdata.Warning
	require.NoError(t, json.Unmarshal([]byte(fixtureUnknownWarnings), &got))
	assert.Equal(t, []batchdata.Warning{
		{Code: "warning_new_code", Message: "Synthetic warning for tests."},
	}, got)
}

func TestOptionsDecode(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    skiptrace.Options
	}{
		{
			name:    "every option",
			fixture: fixtureResultOptions,
			want: skiptrace.Options{
				IncludeTCPABlacklistedPhones: new(false),
				PrioritizeMobilePhones:       new(true),
				DateFormat:                   skiptrace.DateFormatISODate,
			},
		},
		{
			name:    "unknown date format kept",
			fixture: `{"dateFormat": "unix-epoch"}`,
			want:    skiptrace.Options{DateFormat: "unix-epoch"},
		},
		{name: "empty object", fixture: `{}`, want: skiptrace.Options{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got skiptrace.Options
			require.NoError(t, json.Unmarshal([]byte(tt.fixture), &got))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMetaDecode(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    skiptrace.Meta
	}{
		{
			name:    "every count",
			fixture: fixtureResultMeta,
			want: skiptrace.Meta{
				APIVersion:    "9.9.9-test",
				CallRequestID: "call-0000-test",
				Results: skiptrace.Counts{
					RequestCount: new(3), MatchCount: new(1), NoMatchCount: new(1), ErrorCount: new(1),
				},
			},
		},
		{
			name:    "absent counts are nil and zero is kept",
			fixture: fixtureResultMetaPartial,
			want: skiptrace.Meta{
				CallRequestID: "call-0001-test",
				Results:       skiptrace.Counts{RequestCount: new(1), MatchCount: new(0)},
			},
		},
		{name: "empty object", fixture: `{}`, want: skiptrace.Meta{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got skiptrace.Meta
			require.NoError(t, json.Unmarshal([]byte(tt.fixture), &got))
			assert.Equal(t, tt.want, got)
		})
	}
}

// richResult returns a Result with every Outcome and Association, set and nil
// ItemIndex values, nil and empty slices, false and nil flags, and warnings.
func richResult(t *testing.T) skiptrace.Result {
	t.Helper()
	full := decodeFixtureRecord(t, fixtureFullRecord)
	return skiptrace.Result{
		Records: []skiptrace.Record{
			{
				ItemIndex:   new(0),
				Association: skiptrace.AssociationRequestID,
				Outcome:     skiptrace.OutcomeMatched,
				Input:       full.Input,
				Property:    full.Property,
				Persons:     full.Persons,
				Meta:        full.Meta,
			},
			{
				ItemIndex:   new(1),
				Association: skiptrace.AssociationSingleItem,
				Outcome:     skiptrace.OutcomeNoMatch,
				Input:       skiptrace.Input{RequestID: "req-0002"},
				Persons:     []skiptrace.Person{},
				Meta:        skiptrace.RecordMeta{Matched: new(false), Error: new(false)},
			},
			{
				Association: skiptrace.AssociationAmbiguous,
				Outcome:     skiptrace.OutcomeFailed,
				Meta:        skiptrace.RecordMeta{Error: new(true)},
			},
			{
				Association: skiptrace.AssociationNone,
				Outcome:     skiptrace.OutcomeUnreported,
				Property:    &skiptrace.Property{Owners: []skiptrace.Owner{}},
				Persons: []skiptrace.Person{{
					PropertyOwner: new(false),
					Name:          skiptrace.PersonName{First: "Sentinel", AKAs: []skiptrace.AKA{}},
					Addresses:     []skiptrace.PersonAddress{},
					Phones:        []skiptrace.Phone{{Number: "2025550108", TCPA: new(false)}},
					Emails:        []skiptrace.Email{},
				}},
			},
			{
				Association: skiptrace.AssociationNone,
				Outcome:     skiptrace.OutcomeUndecodable,
			},
		},
		Options: skiptrace.Options{
			IncludeTCPABlacklistedPhones: new(false),
			DateFormat:                   skiptrace.DateFormatISODateTime,
		},
		Meta: skiptrace.Meta{
			APIVersion:    "9.9.9-test",
			CallRequestID: "call-0000-test",
			Results:       skiptrace.Counts{RequestCount: new(2), MatchCount: new(0)},
		},
		Warnings: []batchdata.Warning{
			{Code: batchdata.WarningDeprecatedField, Message: "Synthetic deprecation."},
			{Code: "warning_new_code"},
		},
	}
}

func TestResultJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		res  skiptrace.Result
	}{
		{name: "rich", res: richResult(t)},
		{name: "empty", res: skiptrace.Result{}},
		{name: "empty slices", res: skiptrace.Result{Records: []skiptrace.Record{}, Warnings: []batchdata.Warning{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.RequireJSONRoundTrip(t, tt.res)
		})
	}
}

// recordsForResult returns a Result whose records are linked to items 0, 2
// and 4, plus two records with no ItemIndex.
func recordsForResult() *skiptrace.Result {
	rec := func(id string, idx *int, a skiptrace.Association) skiptrace.Record {
		return skiptrace.Record{
			ItemIndex:   idx,
			Association: a,
			Outcome:     skiptrace.OutcomeMatched,
			Input:       skiptrace.Input{RequestID: id},
			Persons:     []skiptrace.Person{{Name: skiptrace.PersonName{Full: id}}},
		}
	}
	return &skiptrace.Result{Records: []skiptrace.Record{
		rec("a0", new(0), skiptrace.AssociationRequestID),
		rec("a1", new(0), skiptrace.AssociationRequestID),
		rec("none", nil, skiptrace.AssociationNone),
		rec("b0", new(2), skiptrace.AssociationRequestID),
		rec("ambiguous", nil, skiptrace.AssociationAmbiguous),
		rec("c0", new(4), skiptrace.AssociationRequestID),
		rec("b1", new(2), skiptrace.AssociationRequestID),
		rec("a2", new(0), skiptrace.AssociationRequestID),
	}}
}

// recordIDs returns the echoed RequestID of each record, or nil for nil.
func recordIDs(records []skiptrace.Record) []string {
	if records == nil {
		return nil
	}
	ids := make([]string, 0, len(records))
	for _, r := range records {
		ids = append(ids, r.Input.RequestID)
	}
	return ids
}

func TestResultRecordsFor(t *testing.T) {
	tests := []struct {
		name  string
		index int
		want  []string
	}{
		{name: "several records in provider order", index: 0, want: []string{"a0", "a1", "a2"}},
		{name: "two records apart", index: 2, want: []string{"b0", "b1"}},
		{name: "one record", index: 4, want: []string{"c0"}},
		{name: "no records", index: 1, want: nil},
		{name: "no records for later item", index: 3, want: nil},
		{name: "out of range", index: skiptrace.MaxItems, want: nil},
		{name: "negative", index: -1, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := recordsForResult()
			got := r.RecordsFor(tt.index)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, tt.want, recordIDs(got))
			}
			assert.Equal(t, recordsForResult(), r, "RecordsFor must not modify the Result")
		})
	}
}

func TestResultRecordsForEdgeCases(t *testing.T) {
	t.Run("nil result", func(t *testing.T) {
		var r *skiptrace.Result
		assert.Nil(t, r.RecordsFor(0))
	})
	t.Run("empty result", func(t *testing.T) {
		assert.Nil(t, (&skiptrace.Result{}).RecordsFor(0))
	})
	t.Run("only records without item index", func(t *testing.T) {
		r := &skiptrace.Result{Records: []skiptrace.Record{
			{Association: skiptrace.AssociationNone, Outcome: skiptrace.OutcomeMatched},
			{Association: skiptrace.AssociationAmbiguous, Outcome: skiptrace.OutcomeNoMatch},
		}}
		assert.Nil(t, r.RecordsFor(0))
	})
	t.Run("single item", func(t *testing.T) {
		r := &skiptrace.Result{Records: []skiptrace.Record{
			{ItemIndex: new(0), Association: skiptrace.AssociationSingleItem, Outcome: skiptrace.OutcomeNoMatch},
		}}
		got := r.RecordsFor(0)
		require.Len(t, got, 1)
		assert.Equal(t, r.Records[0], got[0])
	})
}

func TestResultRecordsForReturnsCopy(t *testing.T) {
	t.Run("changing elements", func(t *testing.T) {
		r := recordsForResult()
		got := r.RecordsFor(0)
		require.Len(t, got, 3)

		got[0].Outcome = skiptrace.OutcomeFailed
		got[0].Input.RequestID = "changed"
		got[1] = skiptrace.Record{}
		assert.Equal(t, recordsForResult(), r)
	})
	t.Run("appending after a contiguous run", func(t *testing.T) {
		// The matches are r.Records[0:2]: a sub-slice of r.Records would let
		// the append overwrite r.Records[2].
		build := func() *skiptrace.Result {
			full := recordsForResult()
			return &skiptrace.Result{Records: []skiptrace.Record{full.Records[0], full.Records[1], full.Records[3]}}
		}
		r := build()
		got := r.RecordsFor(0)
		assert.Equal(t, []string{"a0", "a1"}, recordIDs(got))

		got = append(got, skiptrace.Record{Outcome: skiptrace.OutcomeUndecodable})
		require.Len(t, got, 3)
		assert.Equal(t, build(), r)
	})
}
