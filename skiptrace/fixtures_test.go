package skiptrace_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/skiptrace"
)

// Shared, synthetic test data. Every value is invented: phone numbers are in
// the 202-555-01xx fiction range, emails use example.com or example.test, and
// addresses, APNs and FIPS codes are placeholders. Nothing is copied from
// BatchData's documentation.

// Synthetic request values.
const (
	fixtureStreet        = "100 Example Ave"
	fixtureMailingStreet = "200 Sample St"
	fixtureCity          = "Testville"
	fixtureState         = "ZZ"
	fixtureZip           = "00000"
	fixtureAPN           = "000-000-0000"
	fixtureCounty        = "Example County"
	fixtureFIPS          = "00000"
	fixtureFirstName     = "Sentinel"
	fixtureLastName      = "Person"
)

// fixtureFullRecord is one data[i] object for a matched property: an input
// echo with every InputAddress field, a property with addresses and owners,
// and two persons with every documented field and flag.
const fixtureFullRecord = `{
  "input": {
    "requestId": "req-0001",
    "propertyAddress": {
      "hash": "hash-input-0001",
      "addressValidity": "Valid",
      "street": "100 Example Ave Apt 1",
      "city": "Testville",
      "state": "ZZ",
      "zip": "00000",
      "zipPlus4": "0001",
      "county": "Example County",
      "countyFipsCode": "00000",
      "houseNumber": "100",
      "unitType": "Apt",
      "unitNumber": "1",
      "streetNoUnit": "100 Example Ave",
      "formattedStreet": "100 EXAMPLE AVE APT 1",
      "deliveryPointCode": "01",
      "dpvMatchCode": "Y",
      "dpvFootnotes": "AAZZ",
      "geoStatus": "TEST",
      "geoStatusCode": "T1",
      "cityAliases": ["Testville", "Test Village"],
      "oldHashes": ["hash-old-0001", "hash-old-0002"],
      "localities": ["Testville Heights", "Testville Center"],
      "latitude": 0.5,
      "longitude": -0.25,
      "normalized": true
    },
    "mailingAddress": {
      "street": "200 Sample St",
      "city": "Testville",
      "state": "ZZ",
      "zip": "00000",
      "normalized": false
    },
    "name": {"first": "Sentinel", "last": "Person"},
    "apn": "000-000-0000",
    "countyFipsCode": "00000",
    "county": "Example County",
    "state": "ZZ"
  },
  "property": {
    "id": "prop-0001",
    "address": {
      "hash": "hash-prop-0001",
      "addressValidity": "Valid",
      "fullAddress": "100 Example Ave Apt 1, Testville, ZZ 00000",
      "street": "100 Example Ave Apt 1",
      "city": "Testville",
      "county": "Example County",
      "state": "ZZ",
      "zip": "00000",
      "zipPlus4": "0001"
    },
    "mailingAddress": {
      "hash": "hash-mail-0001",
      "addressValidity": "Partially Valid",
      "fullAddress": "200 Sample St, Testville, ZZ 00000",
      "street": "200 Sample St",
      "city": "Testville",
      "county": "Example County",
      "state": "ZZ",
      "zip": "00000",
      "zipPlus4": "0002"
    },
    "owners": [
      {
        "name": {
          "first": "Sentinel",
          "middle": "Q",
          "last": "Person",
          "full": "Sentinel Q Person",
          "akas": [{"first": "Senti", "last": "Person", "full": "Senti Person", "rank": 1}]
        }
      },
      {"name": {"full": "Example Holdings Trust"}}
    ]
  },
  "persons": [
    {
      "propertyOwner": true,
      "name": {
        "first": "Sentinel",
        "middle": "Q",
        "last": "Person",
        "full": "Sentinel Q Person",
        "akas": [
          {"first": "Senti", "last": "Person", "full": "Senti Person", "rank": 1},
          {"first": "Sentinel", "middle": "Quill", "last": "Persona", "full": "Sentinel Quill Persona", "rank": 2}
        ]
      },
      "addresses": [
        {
          "hash": "hash-addr-0001",
          "addressValidity": "Valid",
          "fullAddress": "100 Example Ave Apt 1, Testville, ZZ 00000",
          "street": "100 Example Ave Apt 1",
          "city": "Testville",
          "county": "Example County",
          "state": "ZZ",
          "zip": "00000",
          "zipPlus4": "0001",
          "rank": 1,
          "propertyMailingAddress": false
        },
        {
          "hash": "hash-addr-0002",
          "addressValidity": "Invalid",
          "fullAddress": "200 Sample St, Testville, ZZ 00000",
          "street": "200 Sample St",
          "city": "Testville",
          "county": "Example County",
          "state": "ZZ",
          "zip": "00000",
          "zipPlus4": "0002",
          "rank": 2,
          "propertyMailingAddress": true
        }
      ],
      "phones": [
        {"rank": 1, "number": "2025550101", "type": "Mobile", "carrier": "Example Wireless",
          "tested": true, "reachable": true, "dnc": false, "tcpa": false},
        {"rank": 2, "number": "2025550102", "type": "Land Line", "carrier": "Example Telco",
          "tested": false, "reachable": false, "dnc": true, "tcpa": true},
        {"rank": 3, "number": "2025550103", "type": "VoIP", "carrier": "Example Voice",
          "tested": true, "reachable": false, "dnc": false}
      ],
      "emails": [
        {"rank": 1, "email": "sentinel.person@example.com", "tested": true},
        {"rank": 2, "email": "sentinel.q@example.test", "tested": false}
      ],
      "litigator": false,
      "deceased": false,
      "dob": "1901-02-03T00:00:00.000Z"
    },
    {
      "propertyOwner": false,
      "name": {"first": "Sample", "last": "Resident", "full": "Sample Resident"},
      "addresses": [
        {"fullAddress": "300 Placeholder Rd, Testville, ZZ 00000", "street": "300 Placeholder Rd",
          "city": "Testville", "state": "ZZ", "zip": "00000", "rank": 1}
      ],
      "phones": [
        {"rank": 1, "number": "2025550104", "type": "Mobile", "carrier": "Example Wireless",
          "tested": true, "reachable": true, "dnc": false}
      ],
      "emails": [{"rank": 1, "email": "sample.resident@example.com"}],
      "litigator": true,
      "deceased": true,
      "dob": "1902-03-04T00:00:00.000Z"
    }
  ],
  "meta": {"matched": true, "error": false}
}`

// fixtureNoMatchRecord is a valid no-match answer: an empty persons array, no
// property, and meta.matched false.
const fixtureNoMatchRecord = `{
  "input": {
    "requestId": "req-0002",
    "propertyAddress": {"street": "999 Nowhere Ln", "city": "Testville", "state": "ZZ", "zip": "00000"}
  },
  "persons": [],
  "meta": {"matched": false, "error": false}
}`

// fixtureErrorRecord is a record the provider flagged with meta.error.
const fixtureErrorRecord = `{
  "input": {"requestId": "req-0003", "apn": "000-000-0000", "countyFipsCode": "00000"},
  "meta": {"matched": false, "error": true}
}`

// fixtureUnreportedRecord has a meta object without the matched flag.
const fixtureUnreportedRecord = `{
  "input": {"requestId": "req-0004", "propertyAddress": {"street": "100 Example Ave", "zip": "00000"}},
  "persons": [{"name": {"first": "Sentinel", "last": "Person"}}],
  "meta": {"error": false}
}`

// fixtureNullsRecord sends explicit nulls where fixtureAbsentRecord omits the
// same fields; both decode to nil.
const fixtureNullsRecord = `{
  "input": {
    "requestId": null,
    "propertyAddress": {"street": "100 Example Ave", "zip": "00000", "latitude": null, "normalized": null},
    "mailingAddress": null,
    "name": null
  },
  "property": null,
  "persons": [
    {
      "propertyOwner": null,
      "name": {"first": "Sentinel", "last": "Person", "akas": null},
      "addresses": null,
      "phones": [{"number": "2025550105", "tcpa": null, "dnc": null}],
      "emails": null,
      "litigator": null,
      "deceased": null,
      "dob": null
    }
  ],
  "meta": {"matched": true, "error": null}
}`

// fixtureAbsentRecord is fixtureNullsRecord with the null fields left out.
const fixtureAbsentRecord = `{
  "input": {"propertyAddress": {"street": "100 Example Ave", "zip": "00000"}},
  "persons": [
    {"name": {"first": "Sentinel", "last": "Person"}, "phones": [{"number": "2025550105"}]}
  ],
  "meta": {"matched": true}
}`

// fixtureUnknownEnumsRecord carries enum values the SDK does not declare; they
// are kept verbatim.
const fixtureUnknownEnumsRecord = `{
  "input": {"propertyAddress": {"addressValidity": "Unknown", "street": "100 Example Ave", "zip": "00000"}},
  "property": {"address": {"addressValidity": "Unknown", "street": "100 Example Ave"}},
  "persons": [
    {
      "addresses": [{"addressValidity": "Unknown", "street": "100 Example Ave"}],
      "phones": [{"rank": 1, "number": "2025550106", "type": "Satellite"}]
    }
  ],
  "meta": {"matched": true, "error": false}
}`

// fixtureUnknownWarnings is an envelope warnings array with an undeclared code.
const fixtureUnknownWarnings = `[{"code": "warning_new_code", "message": "Synthetic warning for tests."}]`

// fixtureOverMaximaRecord has more entries than BatchData documents: 4
// persons, 7 phones, 5 emails, 5 AKAs, 5 addresses and 4 city aliases. Every
// entry is kept, in order.
const fixtureOverMaximaRecord = `{
  "input": {
    "propertyAddress": {
      "street": "100 Example Ave",
      "zip": "00000",
      "cityAliases": ["Alias One", "Alias Two", "Alias Three", "Alias Four"],
      "oldHashes": ["old-1", "old-2", "old-3"],
      "localities": ["Locality 1", "Locality 2", "Locality 3"]
    }
  },
  "persons": [
    {
      "name": {
        "full": "Sentinel Person",
        "akas": [
          {"full": "Aka One", "rank": 1},
          {"full": "Aka Two", "rank": 2},
          {"full": "Aka Three", "rank": 3},
          {"full": "Aka Four", "rank": 4},
          {"full": "Aka Five", "rank": 5}
        ]
      },
      "addresses": [
        {"street": "101 Example Ave", "rank": 1},
        {"street": "102 Example Ave", "rank": 2},
        {"street": "103 Example Ave", "rank": 3},
        {"street": "104 Example Ave", "rank": 4},
        {"street": "105 Example Ave", "rank": 5}
      ],
      "phones": [
        {"rank": 1, "number": "2025550111"},
        {"rank": 2, "number": "2025550112"},
        {"rank": 3, "number": "2025550113"},
        {"rank": 4, "number": "2025550114"},
        {"rank": 5, "number": "2025550115"},
        {"rank": 6, "number": "2025550116"},
        {"rank": 7, "number": "2025550117"}
      ],
      "emails": [
        {"rank": 1, "email": "one@example.com"},
        {"rank": 2, "email": "two@example.com"},
        {"rank": 3, "email": "three@example.test"},
        {"rank": 4, "email": "four@example.test"},
        {"rank": 5, "email": "five@example.com"}
      ]
    },
    {"name": {"full": "Second Person"}},
    {"name": {"full": "Third Person"}},
    {"name": {"full": "Fourth Person"}}
  ],
  "meta": {"matched": true, "error": false}
}`

// fixtureUnknownFieldsRecord carries fields the SDK does not declare, at every
// level; they are ignored.
const fixtureUnknownFieldsRecord = `{
  "futureTopLevel": "ignored",
  "input": {"requestId": "req-0007", "futureInput": {"nested": [1, 2, 3]}},
  "property": {"id": "prop-0007", "futureProperty": {"a": 1}},
  "persons": [
    {
      "name": {"first": "Sentinel", "futureName": true},
      "futurePerson": [1, 2],
      "phones": [{"number": "2025550107", "futureFlag": true}]
    }
  ],
  "meta": {"matched": true, "futureMeta": 1}
}`

// fixtureResultOptions is a result.options object with every documented
// option.
const fixtureResultOptions = `{
  "includeTCPABlacklistedPhones": false,
  "prioritizeMobilePhones": true,
  "dateFormat": "iso-date"
}`

// fixtureResultMeta is a result.meta object with every count reported.
const fixtureResultMeta = `{
  "apiVersion": "9.9.9-test",
  "requestId": "call-0000-test",
  "results": {"requestCount": 3, "matchCount": 1, "noMatchCount": 1, "errorCount": 1}
}`

// fixtureResultMetaPartial is a result.meta object with some counts absent
// and one reported as zero.
const fixtureResultMetaPartial = `{
  "requestId": "call-0001-test",
  "results": {"requestCount": 1, "matchCount": 0}
}`

// fixtureWireRecord mirrors the package's private decoding of one data[i]
// object, so the fixtures prove the exported types' JSON tags match the wire
// names.
type fixtureWireRecord struct {
	Input    skiptrace.Input      `json:"input"`
	Property *skiptrace.Property  `json:"property"`
	Persons  []skiptrace.Person   `json:"persons"`
	Meta     skiptrace.RecordMeta `json:"meta"`
}

// decodeFixtureRecord unmarshals a data[i] fixture into a fixtureWireRecord.
func decodeFixtureRecord(t *testing.T, fixture string) fixtureWireRecord {
	t.Helper()
	var rec fixtureWireRecord
	require.NoError(t, json.Unmarshal([]byte(fixture), &rec))
	return rec
}

// fixtureFullItem returns a valid item with every field set.
func fixtureFullItem() skiptrace.Item {
	return skiptrace.Item{
		RequestID: "req-0001",
		PropertyAddress: skiptrace.Address{
			Street: fixtureStreet, City: fixtureCity, State: fixtureState, Zip: fixtureZip,
		},
		APN:            fixtureAPN,
		County:         fixtureCounty,
		CountyFIPSCode: fixtureFIPS,
		State:          fixtureState,
		Name:           skiptrace.Name{First: fixtureFirstName, Last: fixtureLastName},
		MailingAddress: skiptrace.Address{
			Street: fixtureMailingStreet, City: fixtureCity, State: fixtureState, Zip: fixtureZip,
		},
	}
}

// fixtureRequest returns a request of n valid items, each with every field set
// and a unique RequestID.
func fixtureRequest(n int) skiptrace.Request {
	items := make([]skiptrace.Item, n)
	for i := range items {
		items[i] = fixtureFullItem()
		items[i].RequestID = "req-" + strconv.Itoa(i)
	}
	return skiptrace.Request{Items: items}
}
