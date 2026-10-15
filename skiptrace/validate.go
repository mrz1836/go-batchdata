package skiptrace

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mrz1836/go-batchdata"
)

// Validation reasons. They are fixed text: a reason never quotes a value.
const (
	reasonNoItems               = "must contain at least 1 item"
	reasonTooManyItems          = "must contain at most 100 items"
	reasonInvalidUTF8           = "must be valid UTF-8"
	reasonBlank                 = "must not be only whitespace"
	reasonStreetRequired        = "is required when PropertyAddress is set"
	reasonMailingStreetRequired = "is required when MailingAddress is set"
	reasonCountyFIPSCode        = "must be exactly 5 ASCII digits"
	reasonNoLookupKey           = "must include one of: PropertyAddress.Street and PropertyAddress.Zip; " +
		"PropertyAddress.Street, PropertyAddress.City and PropertyAddress.State; " +
		"APN and CountyFIPSCode; APN, County and State"
	reasonDateFormat = "must be empty, DateFormatISODateTime or DateFormatISODate"
)

// Validate reports the first problem with r as a *batchdata.ValidationError
// matching batchdata.ErrInvalidRequest, or nil. It performs no I/O and never
// modifies r.
//
// It checks, in order and stopping at the first problem:
//
//   - that r has 1 to MaxItems items; an out-of-range count is reported
//     alone, without inspecting the items;
//   - each item in index order: every string field, in declaration order, is
//     valid UTF-8 and either empty or not only whitespace (as defined by
//     unicode.IsSpace); a non-zero PropertyAddress or MailingAddress has a
//     Street; a non-empty CountyFIPSCode is exactly 5 ASCII digits; and the
//     item satisfies at least one combination documented on Item;
//   - that Options.DateFormat is empty or a documented DateFormat.
//
// Duplicate RequestIDs are allowed. The error's Field is a Go field path such
// as "Items[3].PropertyAddress.Street"; neither it nor the Reason contains a
// value from r.
func (r Request) Validate() error {
	if len(r.Items) == 0 {
		return invalidRequest("Items", reasonNoItems)
	}
	if len(r.Items) > MaxItems {
		return invalidRequest("Items", reasonTooManyItems)
	}
	for i := range r.Items {
		if field, reason := checkItem(&r.Items[i]); reason != "" {
			return invalidRequest(itemPath(i, field), reason)
		}
	}
	switch r.Options.DateFormat {
	case "", DateFormatISODateTime, DateFormatISODate:
		return nil
	default:
		return invalidRequest("Options.DateFormat", reasonDateFormat)
	}
}

// invalidRequest returns a *batchdata.ValidationError matching
// batchdata.ErrInvalidRequest. The reason must not contain a caller's value.
func invalidRequest(field, reason string) error {
	return &batchdata.ValidationError{Err: batchdata.ErrInvalidRequest, Field: field, Reason: reason}
}

// itemPath returns the path of field within Items[i], or of the item itself
// when field is empty.
func itemPath(i int, field string) string {
	path := "Items[" + strconv.Itoa(i) + "]"
	if field != "" {
		path += "." + field
	}
	return path
}

// checkItem returns the first problem with it, as a field path relative to the
// item (empty for the item as a whole) and a reason, or an empty reason.
func checkItem(it *Item) (field, reason string) {
	if field, reason = checkItemStrings(it); reason != "" {
		return field, reason
	}
	if it.PropertyAddress != (Address{}) && it.PropertyAddress.Street == "" {
		return "PropertyAddress.Street", reasonStreetRequired
	}
	if it.MailingAddress != (Address{}) && it.MailingAddress.Street == "" {
		return "MailingAddress.Street", reasonMailingStreetRequired
	}
	if it.CountyFIPSCode != "" && !isCountyFIPSCode(it.CountyFIPSCode) {
		return "CountyFIPSCode", reasonCountyFIPSCode
	}
	if !hasLookupKey(it) {
		return "", reasonNoLookupKey
	}
	return "", ""
}

// checkItemStrings checks every string field of it in declaration order.
func checkItemStrings(it *Item) (field, reason string) {
	fields := [...]struct{ path, value string }{
		{"RequestID", it.RequestID},
		{"PropertyAddress.Street", it.PropertyAddress.Street},
		{"PropertyAddress.City", it.PropertyAddress.City},
		{"PropertyAddress.State", it.PropertyAddress.State},
		{"PropertyAddress.Zip", it.PropertyAddress.Zip},
		{"APN", it.APN},
		{"County", it.County},
		{"CountyFIPSCode", it.CountyFIPSCode},
		{"State", it.State},
		{"Name.First", it.Name.First},
		{"Name.Last", it.Name.Last},
		{"MailingAddress.Street", it.MailingAddress.Street},
		{"MailingAddress.City", it.MailingAddress.City},
		{"MailingAddress.State", it.MailingAddress.State},
		{"MailingAddress.Zip", it.MailingAddress.Zip},
	}
	for i := range fields {
		if reason = checkString(fields[i].value); reason != "" {
			return fields[i].path, reason
		}
	}
	return "", ""
}

// checkString returns the reason s is unusable, or "". Invalid UTF-8 is
// rejected so that encoding/json never replaces it with U+FFFD, and a value of
// only whitespace is rejected rather than trimmed.
func checkString(s string) string {
	switch {
	case !utf8.ValidString(s):
		return reasonInvalidUTF8
	case s != "" && strings.TrimFunc(s, unicode.IsSpace) == "":
		return reasonBlank
	default:
		return ""
	}
}

// isCountyFIPSCode reports whether s is exactly 5 ASCII digits.
func isCountyFIPSCode(s string) bool {
	if len(s) != 5 {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// hasLookupKey reports whether it satisfies a documented combination:
// PropertyAddress.Street with PropertyAddress.Zip or with PropertyAddress.City
// and PropertyAddress.State, or APN with CountyFIPSCode or with County and
// State. Name and MailingAddress never count.
func hasLookupKey(it *Item) bool {
	a := &it.PropertyAddress
	if a.Street != "" && (a.Zip != "" || (a.City != "" && a.State != "")) {
		return true
	}
	return it.APN != "" && (it.CountyFIPSCode != "" || (it.County != "" && it.State != ""))
}
