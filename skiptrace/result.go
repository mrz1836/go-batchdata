package skiptrace

import "github.com/mrz1836/go-batchdata"

// Result is the decoded response of a successful call. Records keep the
// provider's order and are never reordered, deduplicated or truncated.
//
// A Result holds personal data. Access control, logging, retention and lawful
// use are the caller's responsibility.
type Result struct {
	// Records holds one entry per element of the provider's data array.
	Records []Record `json:"records"`

	// Options are the options the provider reports it applied.
	Options Options `json:"options,omitzero"`

	// Meta is the provider's metadata for the whole call.
	Meta Meta `json:"meta,omitzero"`

	// Warnings are the envelope's status.warnings, then its top-level
	// warnings, in order.
	Warnings []batchdata.Warning `json:"warnings,omitzero"`
}

// Record is the provider's answer for one property.
type Record struct {
	// ItemIndex is the index in Request.Items of the item this record is
	// linked to, or nil unless Association is AssociationRequestID or
	// AssociationSingleItem.
	ItemIndex *int `json:"itemIndex,omitempty"`

	// Association reports what links this record to an item.
	Association Association `json:"association"`

	// Outcome summarizes the record's meta flags.
	Outcome Outcome `json:"outcome"`

	// Input is the provider's echo of the request item.
	Input Input `json:"input,omitzero"`

	// Property is the matched property, or nil when none was returned.
	Property *Property `json:"property,omitempty"`

	// Persons are the persons found for the property. A no-match record
	// keeps an empty, non-nil slice when the provider sent one.
	Persons []Person `json:"persons,omitzero"`

	// Meta holds the provider's per-record flags.
	Meta RecordMeta `json:"meta,omitzero"`
}

// Outcome summarizes a record. The empty string is never a valid outcome.
type Outcome string

// Record outcomes. OutcomeUndecodable is decided first; the others follow
// the precedence error, then matched, then an absent flag.
const (
	// OutcomeFailed means the provider set meta.error to true, whatever
	// meta.matched says. BatchData documents no detail for it.
	OutcomeFailed Outcome = "failed"

	// OutcomeMatched means the provider set meta.matched to true: it found
	// data for the property. It does not mean the persons are the one named
	// in Item.Name, which is not a lookup key; compare each person's
	// Name, its AKAs included, with the name you hold.
	OutcomeMatched Outcome = "matched"

	// OutcomeNoMatch means the provider set meta.matched to false: a valid
	// answer, not an error.
	OutcomeNoMatch Outcome = "no_match"

	// OutcomeUnreported means the provider sent no meta.matched flag.
	OutcomeUnreported Outcome = "unreported"

	// OutcomeUndecodable means the record was null, not an object, or held a
	// value of the wrong type. Nothing of it is kept: its Association is
	// AssociationNone and every other field is zero.
	OutcomeUndecodable Outcome = "undecodable"
)

// Association reports what links a record to a request item. Records are
// never linked by position, name or address.
type Association string

// Associations.
const (
	// AssociationRequestID means the record's echoed Input.RequestID equals
	// the RequestID of exactly one item, and no other item's RequestID equals
	// it after trimming surrounding whitespace and folding case.
	AssociationRequestID Association = "request_id"

	// AssociationSingleItem means the request had one item and the response
	// one decodable record, and either the item had no RequestID or the
	// record echoed none.
	AssociationSingleItem Association = "single_item"

	// AssociationAmbiguous means the echoed RequestID equals the RequestID
	// of an item, but more than one item has that RequestID, or another
	// item's differs from it only in case or surrounding whitespace.
	AssociationAmbiguous Association = "ambiguous"

	// AssociationNone means there is no evidence linking the record to an
	// item.
	AssociationNone Association = "none"
)

// RecordMeta holds a record's flags. Nil means the provider did not send the
// flag.
type RecordMeta struct {
	Matched *bool `json:"matched,omitempty"`
	Error   *bool `json:"error,omitempty"`
}

// Meta is the provider's metadata for a call.
type Meta struct {
	// APIVersion is the version the provider reports, such as "3.1.0".
	APIVersion string `json:"apiVersion,omitempty"`

	// CallRequestID is the provider's identifier for the whole call, for
	// support requests. It identifies the call, not an item.
	CallRequestID string `json:"requestId,omitempty"`

	// Results holds the provider's counts.
	Results Counts `json:"results,omitzero"`
}

// Counts are the provider's counts for a call. Nil means the provider did not
// report the count, which is not the same as zero.
type Counts struct {
	RequestCount *int `json:"requestCount,omitempty"`

	// MatchCount is the number of items BatchData bills: matched property
	// records with at least one person.
	MatchCount *int `json:"matchCount,omitempty"`

	NoMatchCount *int `json:"noMatchCount,omitempty"`
	ErrorCount   *int `json:"errorCount,omitempty"`
}

// Input is the provider's echo of a request item.
type Input struct {
	// RequestID is the echoed Item.RequestID. The response schema does not
	// list it; BatchData's request ID correlation article describes the
	// echo.
	RequestID string `json:"requestId,omitempty"`

	PropertyAddress *InputAddress `json:"propertyAddress,omitempty"`
	MailingAddress  *InputAddress `json:"mailingAddress,omitempty"`
	Name            *Name         `json:"name,omitempty"`
	APN             string        `json:"apn,omitempty"`
	CountyFIPSCode  string        `json:"countyFipsCode,omitempty"`
	County          string        `json:"county,omitempty"`
	State           string        `json:"state,omitempty"`
}

// InputAddress is the provider's echo of a request address, with the
// standardization it applied.
type InputAddress struct {
	Hash              string          `json:"hash,omitempty"`
	AddressValidity   AddressValidity `json:"addressValidity,omitempty"`
	Street            string          `json:"street,omitempty"`
	City              string          `json:"city,omitempty"`
	State             string          `json:"state,omitempty"`
	Zip               string          `json:"zip,omitempty"`
	ZipPlus4          string          `json:"zipPlus4,omitempty"`
	County            string          `json:"county,omitempty"`
	CountyFIPSCode    string          `json:"countyFipsCode,omitempty"`
	HouseNumber       string          `json:"houseNumber,omitempty"`
	UnitType          string          `json:"unitType,omitempty"`
	UnitNumber        string          `json:"unitNumber,omitempty"`
	StreetNoUnit      string          `json:"streetNoUnit,omitempty"`
	FormattedStreet   string          `json:"formattedStreet,omitempty"`
	DeliveryPointCode string          `json:"deliveryPointCode,omitempty"`
	DPVMatchCode      string          `json:"dpvMatchCode,omitempty"`

	// DPVFootnotes is a single string in the provider's schema.
	DPVFootnotes  string `json:"dpvFootnotes,omitempty"`
	GeoStatus     string `json:"geoStatus,omitempty"`
	GeoStatusCode string `json:"geoStatusCode,omitempty"`

	// CityAliases, OldHashes and Localities are documented with at most two
	// entries; extra entries are kept.
	CityAliases []string `json:"cityAliases,omitzero"`
	OldHashes   []string `json:"oldHashes,omitzero"`
	Localities  []string `json:"localities,omitzero"`

	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
	Normalized *bool    `json:"normalized,omitempty"`
}

// Property is the property the provider matched.
type Property struct {
	ID             string         `json:"id,omitempty"`
	Address        *PostalAddress `json:"address,omitempty"`
	MailingAddress *PostalAddress `json:"mailingAddress,omitempty"`
	Owners         []Owner        `json:"owners,omitzero"`
}

// PostalAddress is a standardized address in a response.
//
// PersonAddress embeds it, so it must never gain a custom JSON marshaler or
// unmarshaler: one would be promoted and replace PersonAddress's own fields.
type PostalAddress struct {
	Hash            string          `json:"hash,omitempty"`
	AddressValidity AddressValidity `json:"addressValidity,omitempty"`
	FullAddress     string          `json:"fullAddress,omitempty"`
	Street          string          `json:"street,omitempty"`
	City            string          `json:"city,omitempty"`
	County          string          `json:"county,omitempty"`
	State           string          `json:"state,omitempty"`
	Zip             string          `json:"zip,omitempty"`
	ZipPlus4        string          `json:"zipPlus4,omitempty"`
}

// Owner is an owner of record on the property.
type Owner struct {
	Name PersonName `json:"name,omitzero"`
}

// Person is a person found for the property, with contact data.
//
// The flags are provider observations, not consent or compliance decisions.
type Person struct {
	// PropertyOwner is true when the person's name matched an owner of
	// record, which is not proof of ownership, and false when owner names
	// were compared and none matched. Nil means ownership could not be
	// evaluated.
	PropertyOwner *bool `json:"propertyOwner,omitempty"`

	Name      PersonName      `json:"name,omitzero"`
	Addresses []PersonAddress `json:"addresses,omitzero"`
	Phones    []Phone         `json:"phones,omitzero"`
	Emails    []Email         `json:"emails,omitzero"`

	// Litigator reports whether the person is on the known TCPA litigators
	// list. It is informational and filters nothing.
	Litigator *bool `json:"litigator,omitempty"`

	// Deceased is true when records show the person as deceased. Nil means
	// no value is on file.
	Deceased *bool `json:"deceased,omitempty"`

	// DOB is the date of birth as sent: an ISO 8601 date-time by default, or
	// a date with DateFormatISODate. The SDK never parses it. Nil means no
	// value is on file.
	DOB *string `json:"dob,omitempty"`
}

// PersonName is a person's name and the other names they are known by.
type PersonName struct {
	First  string `json:"first,omitempty"`
	Middle string `json:"middle,omitempty"`
	Last   string `json:"last,omitempty"`
	Full   string `json:"full,omitempty"`
	AKAs   []AKA  `json:"akas,omitzero"`
}

// AKA is another name a person is known by.
type AKA struct {
	First  string `json:"first,omitempty"`
	Middle string `json:"middle,omitempty"`
	Last   string `json:"last,omitempty"`
	Full   string `json:"full,omitempty"`

	// Rank orders the entries, starting at 1; 0 means not reported.
	Rank int `json:"rank,omitempty"`
}

// PersonAddress is an address associated with a person.
type PersonAddress struct {
	PostalAddress

	// Rank is the presentation order, starting at 1, not a measured
	// confidence; 0 means not reported.
	Rank int `json:"rank,omitempty"`

	// PropertyMailingAddress reports whether this is the mailing address of
	// the property.
	PropertyMailingAddress *bool `json:"propertyMailingAddress,omitempty"`
}

// Phone is a phone number associated with a person.
type Phone struct {
	// Rank is the presentation order, starting at 1, not a probability of
	// reaching the person; 0 means not reported.
	Rank int `json:"rank,omitempty"`

	// Number is the 10-digit number as sent.
	Number  string    `json:"number,omitempty"`
	Type    PhoneType `json:"type,omitempty"`
	Carrier string    `json:"carrier,omitempty"`

	// Tested reports a successful connection in the last six months.
	Tested *bool `json:"tested,omitempty"`

	// Reachable reports a successful reach in the last three months.
	Reachable *bool `json:"reachable,omitempty"`

	// DNC reports registration on the national Do Not Call Registry.
	DNC *bool `json:"dnc,omitempty"`

	// TCPA reports TCPA restrictions. BatchData sends it only when
	// IncludeTCPABlacklistedPhones is true; nil means not reported, which is
	// not the same as false.
	TCPA *bool `json:"tcpa,omitempty"`
}

// Email is an email address associated with a person.
type Email struct {
	// Rank is the presentation order, starting at 1; 0 means not reported.
	Rank  int    `json:"rank,omitempty"`
	Email string `json:"email,omitempty"`

	// Tested reports that the address is registered and unlikely to bounce.
	Tested *bool `json:"tested,omitempty"`
}

// PhoneType is the type of a phone line. Values other than the documented
// constants are kept as sent.
type PhoneType string

// Documented phone types.
const (
	PhoneTypeMobile   PhoneType = "Mobile"
	PhoneTypeLandLine PhoneType = "Land Line"
	PhoneTypeVoIP     PhoneType = "VoIP"
)

// AddressValidity is the result of address validation. Values other than the
// documented constants are kept as sent.
type AddressValidity string

// Documented address validity values.
const (
	AddressValid          AddressValidity = "Valid"
	AddressInvalid        AddressValidity = "Invalid"
	AddressPartiallyValid AddressValidity = "Partially Valid"
)

// RecordsFor returns the records linked to Items[itemIndex], in provider order:
// those whose ItemIndex equals itemIndex. Records with a nil ItemIndex are never
// returned. It returns nil when there are none, including for a negative or out
// of range index or a nil Result.
//
// The returned slice is new, so changing it leaves r.Records alone, but each
// element is a shallow copy that shares its pointers and slices with r.
func (r *Result) RecordsFor(itemIndex int) []Record {
	if r == nil || itemIndex < 0 {
		return nil
	}
	var out []Record
	for i := range r.Records {
		if idx := r.Records[i].ItemIndex; idx != nil && *idx == itemIndex {
			out = append(out, r.Records[i])
		}
	}
	return out
}
