package skiptrace

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Request IDs used across the association tables.
const (
	row1 = "row-1"
	row2 = "row-2"
	row3 = "row-3"
	row9 = "row-9"
)

// echo returns a minimal matched record that echoes id; an empty id echoes
// nothing.
func echo(id string) string {
	if id == "" {
		return `{"input":{"propertyAddress":{"street":"100 Example Ave","zip":"00000"}},"persons":[],"meta":{"matched":true,"error":false}}`
	}
	return `{"input":{"requestId":"` + id + `"},"persons":[],"meta":{"matched":true,"error":false}}`
}

// data wraps record JSON values in a result object.
func data(records ...string) []byte {
	return []byte(`{"data":[` + strings.Join(records, ",") + `]}`)
}

func TestAssociation(t *testing.T) {
	type link struct {
		assoc Association
		index int // -1 for nil
	}
	tests := []struct {
		name    string
		ids     []string
		records []string
		want    []link
	}{
		{
			name: "ids echoed for all", ids: []string{row1, row2, row3},
			records: []string{echo(row3), echo(row1), echo(row2)},
			want:    []link{{AssociationRequestID, 2}, {AssociationRequestID, 0}, {AssociationRequestID, 1}},
		},
		{
			name: "echo missing in multi-item request", ids: []string{row1, row2},
			records: []string{echo(""), echo("")},
			want:    []link{{AssociationNone, -1}, {AssociationNone, -1}},
		},
		{
			name: "multi-item request without ids is never positional", ids: []string{"", ""},
			records: []string{echo(""), echo("")},
			want:    []link{{AssociationNone, -1}, {AssociationNone, -1}},
		},
		{
			name: "unknown echo", ids: []string{row1, row2},
			records: []string{echo(row9), echo(row2)},
			want:    []link{{AssociationNone, -1}, {AssociationRequestID, 1}},
		},
		{
			name: "ambiguous request ids", ids: []string{"dup", "dup", row3},
			records: []string{echo("dup"), echo("dup"), echo(row3)},
			want:    []link{{AssociationAmbiguous, -1}, {AssociationAmbiguous, -1}, {AssociationRequestID, 2}},
		},
		{
			name: "several records echo one unique id", ids: []string{row1, row2},
			records: []string{echo(row1), echo(row1), echo(row2)},
			want:    []link{{AssociationRequestID, 0}, {AssociationRequestID, 0}, {AssociationRequestID, 1}},
		},
		{
			name: "fewer records than items", ids: []string{row1, row2, row3},
			records: []string{echo(row2)},
			want:    []link{{AssociationRequestID, 1}},
		},
		{
			name: "more records than items without echo", ids: []string{row1, row2},
			records: []string{echo(""), echo(""), echo("")},
			want:    []link{{AssociationNone, -1}, {AssociationNone, -1}, {AssociationNone, -1}},
		},
		{
			name: "single item without id", ids: []string{""},
			records: []string{echo("")},
			want:    []link{{AssociationSingleItem, 0}},
		},
		{
			name: "single item without id and provider-generated echo", ids: []string{""},
			records: []string{echo("01SYNTHETICGENERATEDID0000")},
			want:    []link{{AssociationSingleItem, 0}},
		},
		{
			name: "single item with id and no echo", ids: []string{row1},
			records: []string{echo("")},
			want:    []link{{AssociationSingleItem, 0}},
		},
		{
			name: "single item with matching echo", ids: []string{row1},
			records: []string{echo(row1)},
			want:    []link{{AssociationRequestID, 0}},
		},
		{
			name: "single item with mismatched echo", ids: []string{row1},
			records: []string{echo(row2)},
			want:    []link{{AssociationNone, -1}},
		},
		{
			name: "single item with two records", ids: []string{""},
			records: []string{echo(""), echo("")},
			want:    []link{{AssociationNone, -1}, {AssociationNone, -1}},
		},
		{
			name: "single item with two records, one echoing its id", ids: []string{row1},
			records: []string{echo(row1), echo("")},
			want:    []link{{AssociationRequestID, 0}, {AssociationNone, -1}},
		},
		{
			name: "single item with one decodable record", ids: []string{""},
			records: []string{`null`, echo("")},
			want:    []link{{AssociationNone, -1}, {AssociationSingleItem, 0}},
		},
		{
			name: "undecodable record echoing an id", ids: []string{row1, row2},
			records: []string{`{"input":{"requestId":"row-1"},"persons":{}}`, echo(row2)},
			want:    []link{{AssociationNone, -1}, {AssociationRequestID, 1}},
		},
		{
			name: "empty echo never matches an item without id", ids: []string{"", row2},
			records: []string{echo(""), echo(row2)},
			want:    []link{{AssociationNone, -1}, {AssociationRequestID, 1}},
		},
		{
			name: "ids differing only in surrounding whitespace", ids: []string{row1, "row-1 ", row2},
			records: []string{echo(row1), echo("row-1 "), echo(row2)},
			want:    []link{{AssociationAmbiguous, -1}, {AssociationAmbiguous, -1}, {AssociationRequestID, 2}},
		},
		{
			name: "ids differing only in case", ids: []string{row1, "ROW-1"},
			records: []string{echo(row1), echo("ROW-1")},
			want:    []link{{AssociationAmbiguous, -1}, {AssociationAmbiguous, -1}},
		},
		{
			name: "echo compared exactly", ids: []string{"Row-1", "row-1 "},
			records: []string{echo(row1), echo("ROW-1")},
			want:    []link{{AssociationNone, -1}, {AssociationNone, -1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := decodeResult(data(tt.records...), tt.ids)
			require.NoError(t, err)
			require.Len(t, res.Records, len(tt.want))
			for i, w := range tt.want {
				r := res.Records[i]
				assert.Equal(t, w.assoc, r.Association, "record %d", i)
				if w.index < 0 {
					assert.Nil(t, r.ItemIndex, "record %d", i)
				} else if assert.NotNil(t, r.ItemIndex, "record %d", i) {
					assert.Equal(t, w.index, *r.ItemIndex, "record %d", i)
				}
			}
		})
	}
}

func TestAssociationItemIndexesAreDistinct(t *testing.T) {
	res, err := decodeResult(data(echo("a"), echo("a")), []string{"a", "b"})
	require.NoError(t, err)
	require.NotNil(t, res.Records[0].ItemIndex)
	require.NotNil(t, res.Records[1].ItemIndex)
	assert.NotSame(t, res.Records[0].ItemIndex, res.Records[1].ItemIndex)
}

func TestOutcome(t *testing.T) {
	tests := []struct {
		name string
		meta string
		want Outcome
	}{
		{"matched", `{"matched":true,"error":false}`, OutcomeMatched},
		{"no match", `{"matched":false,"error":false}`, OutcomeNoMatch},
		{"error wins over matched", `{"matched":true,"error":true}`, OutcomeFailed},
		{"error without matched", `{"error":true}`, OutcomeFailed},
		{"error with no match", `{"matched":false,"error":true}`, OutcomeFailed},
		{"matched absent", `{"error":false}`, OutcomeUnreported},
		{"matched null", `{"matched":null}`, OutcomeUnreported},
		{"meta empty", `{}`, OutcomeUnreported},
		{"meta null", `null`, OutcomeUnreported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := decodeRecord([]byte(`{"meta":` + tt.meta + `}`))
			assert.Equal(t, tt.want, r.Outcome)
		})
	}
	t.Run("meta absent", func(t *testing.T) {
		assert.Equal(t, OutcomeUnreported, decodeRecord([]byte(`{}`)).Outcome)
	})
}

func TestDecodeRecordUndecodable(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"null", `null`},
		{"number", `1`},
		{"string", `"SENTINEL"`},
		{"array", `[]`},
		{"true", `true`},
		{"persons object", `{"persons":{},"meta":{"matched":true}}`},
		{"matched string", `{"meta":{"matched":"true"}}`},
		{"input string", `{"input":"SENTINEL"}`},
		{"phone rank fraction", `{"persons":[{"phones":[{"rank":1.5,"number":"2025550100"}]}],"meta":{"matched":true}}`},
		{"rank overflow", `{"persons":[{"emails":[{"rank":1e30}]}],"meta":{"matched":true}}`},
		{"dob number", `{"persons":[{"dob":19850721}],"meta":{"matched":true}}`},
		{"latitude string", `{"input":{"propertyAddress":{"latitude":"1.5"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := decodeRecord([]byte(tt.raw))
			assert.Equal(t, Record{Association: AssociationNone, Outcome: OutcomeUndecodable}, r)
		})
	}
}

func TestDecodeResultOneBadRecordAmongGood(t *testing.T) {
	res, err := decodeResult(data(echo("a"), `{"persons":"SENTINEL"}`, echo("b")), []string{"a", "x", "b"})
	require.NoError(t, err)
	require.Len(t, res.Records, 3)
	assert.Equal(t, OutcomeMatched, res.Records[0].Outcome)
	assert.Equal(t, OutcomeUndecodable, res.Records[1].Outcome)
	assert.Equal(t, OutcomeMatched, res.Records[2].Outcome)
	assert.Equal(t, new(2), res.Records[2].ItemIndex)
}

func TestDecodeResultShapes(t *testing.T) {
	t.Run("empty data", func(t *testing.T) {
		res, err := decodeResult([]byte(`{"data":[]}`), []string{"a"})
		require.NoError(t, err)
		assert.NotNil(t, res.Records)
		assert.Empty(t, res.Records)
	})
	t.Run("null options and meta", func(t *testing.T) {
		res, err := decodeResult([]byte(`{"data":[],"options":null,"meta":null}`), nil)
		require.NoError(t, err)
		assert.Zero(t, res.Options)
		assert.Zero(t, res.Meta)
	})
	t.Run("options and meta", func(t *testing.T) {
		res, err := decodeResult([]byte(`{"data":[],"options":{"includeTCPABlacklistedPhones":false,"dateFormat":"iso-date",`+
			`"webhookUrl":"https://example.test/hook"},"meta":{"apiVersion":"9.9.9","requestId":"01SYNTHETICCALLID000000000",`+
			`"results":{"requestCount":2,"matchCount":1,"noMatchCount":1}}}`), nil)
		require.NoError(t, err)
		require.NotNil(t, res.Options.IncludeTCPABlacklistedPhones)
		assert.False(t, *res.Options.IncludeTCPABlacklistedPhones)
		assert.Nil(t, res.Options.PrioritizeMobilePhones)
		assert.Equal(t, DateFormatISODate, res.Options.DateFormat)
		assert.Equal(t, "9.9.9", res.Meta.APIVersion)
		assert.Equal(t, "01SYNTHETICCALLID000000000", res.Meta.CallRequestID)
		assert.Equal(t, new(2), res.Meta.Results.RequestCount)
		assert.Equal(t, new(1), res.Meta.Results.MatchCount)
		assert.Equal(t, new(1), res.Meta.Results.NoMatchCount)
		assert.Nil(t, res.Meta.Results.ErrorCount, "absent is not zero")
	})
}

func TestDecodeResultMalformed(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"data missing", `{}`},
		{"data null", `{"data":null}`},
		{"data object", `{"data":{}}`},
		{"data string", `{"data":"SENTINEL"}`},
		{"data number", `{"data":1}`},
		{"options string", `{"data":[],"options":"x"}`},
		{"options array", `{"data":[],"options":[]}`},
		{"options flag string", `{"data":[],"options":{"includeTCPABlacklistedPhones":"yes"}}`},
		{"options date format number", `{"data":[],"options":{"dateFormat":5}}`},
		{"meta string", `{"data":[],"meta":"x"}`},
		{"meta results array", `{"data":[],"meta":{"results":[]}}`},
		{"meta request id number", `{"data":[],"meta":{"requestId":5}}`},
		{"meta api version number", `{"data":[],"meta":{"apiVersion":3.1}}`},
		{"count overflow", `{"data":[],"meta":{"results":{"matchCount":1e30}}}`},
		{"count fraction", `{"data":[],"meta":{"results":{"requestCount":1.5}}}`},
		{"count string", `{"data":[],"meta":{"results":{"errorCount":"0"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := decodeResult([]byte(tt.raw), nil)
			require.ErrorIs(t, err, errMalformed)
			assert.Nil(t, res)
		})
	}
}
