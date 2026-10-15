package skiptrace

// MaxItems is the largest number of items one request may carry.
const MaxItems = 100

// Request is one synchronous skip-trace call: 1 to MaxItems items and the
// options that apply to all of them. Each item is one property lookup.
type Request struct {
	Items   []Item  `json:"requests"`
	Options Options `json:"options,omitzero"`
}

// Item identifies one property, by address or by assessor's parcel number
// (APN). An item must satisfy at least one documented combination:
//
//   - PropertyAddress with Street, City, State and Zip;
//   - PropertyAddress with Street and Zip;
//   - PropertyAddress with Street, City and State;
//   - APN, County and State;
//   - APN and CountyFIPSCode.
//
// Other fields may be set alongside a satisfied combination. Name and
// MailingAddress are optional extras, not lookup keys. Values are sent as
// given: the SDK never trims, normalizes or rewrites them. Instead,
// Request.Validate rejects a string that is not valid UTF-8 or is only
// whitespace, and a CountyFIPSCode that is not exactly 5 ASCII digits.
type Item struct {
	// RequestID is a caller-chosen identifier for the item. It is the only
	// evidence the SDK uses to link a returned record to a multi-item
	// request, and only when BatchData echoes it and it is unique among the
	// items (see Record.Association).
	RequestID string `json:"requestId,omitempty"`

	PropertyAddress Address `json:"propertyAddress,omitzero"`
	APN             string  `json:"apn,omitempty"`
	County          string  `json:"county,omitempty"`
	CountyFIPSCode  string  `json:"countyFipsCode,omitempty"`
	State           string  `json:"state,omitempty"`
	Name            Name    `json:"name,omitzero"`
	MailingAddress  Address `json:"mailingAddress,omitzero"`
}

// Address is a postal address in a request. When set, Street is required.
type Address struct {
	Street string `json:"street,omitempty"`
	City   string `json:"city,omitempty"`
	State  string `json:"state,omitempty"`
	Zip    string `json:"zip,omitempty"`
}

// Name is a person's name in a request or in the provider's echo of one.
type Name struct {
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
}

// Options control processing and formatting for every item of a request. A
// nil pointer omits the option, so the provider's default applies; the SDK
// never injects a default of its own.
//
// BatchData documents includeTCPABlacklistedPhones as defaulting to true, but
// also says that phones with tcpa=true are excluded when it is not specified.
// Set IncludeTCPABlacklistedPhones explicitly when the difference matters.
type Options struct {
	// IncludeTCPABlacklistedPhones includes phones with TCPA restrictions,
	// each flagged by Phone.TCPA, when true, and filters them out when false.
	IncludeTCPABlacklistedPhones *bool `json:"includeTCPABlacklistedPhones,omitempty"`

	// PrioritizeMobilePhones lists mobile numbers first in each person's
	// phones when true (the documented default), and leaves the order alone
	// when false.
	PrioritizeMobilePhones *bool `json:"prioritizeMobilePhones,omitempty"`

	// DateFormat selects the format of Person.DOB.
	DateFormat DateFormat `json:"dateFormat,omitempty"`
}

// DateFormat selects the format of dates in a response.
type DateFormat string

// Documented date formats.
const (
	// DateFormatISODateTime is an ISO 8601 date-time, such as
	// "1985-07-21T00:00:00.000Z". It is the provider's default.
	DateFormatISODateTime DateFormat = "iso-date-time"

	// DateFormatISODate is an ISO 8601 date, such as "1985-07-21".
	DateFormatISODate DateFormat = "iso-date"
)
