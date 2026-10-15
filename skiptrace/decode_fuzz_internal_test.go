package skiptrace

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/jsonx"
	"github.com/mrz1836/go-batchdata/internal/testutil"
)

// FuzzDecodeResult checks that any result object decodes without panicking
// into records whose outcome and association are valid and whose links point
// at real items, and that a decoded Result survives a JSON round trip.
func FuzzDecodeResult(f *testing.F) {
	for _, s := range []string{
		`{"data":[{"input":{"requestId":"a"},"persons":[],"meta":{"matched":false,"error":false}}]}`,
		`{"data":[null,1,"x",[],{"persons":{}}],"options":{"dateFormat":"iso-date"},"meta":{"results":{"matchCount":1}}}`,
		`{"data":[{"input":{"requestId":"b","propertyAddress":{"latitude":1.5,"cityAliases":["x"]}},` +
			`"property":{"owners":[{"name":{"akas":[{"rank":1}]}}]},` +
			`"persons":[{"propertyOwner":null,"dob":null,"phones":[{"rank":1,"tcpa":true}]}],"meta":{"matched":true}}]}`,
		`{"data":{}}`,
		`{}`,
	} {
		f.Add([]byte(s), "a", "b")
	}
	f.Fuzz(func(t *testing.T, raw []byte, id0, id1 string) {
		if !json.Valid(raw) || !jsonx.IsObject(raw) {
			return // the root package only hands over valid objects
		}
		ids := []string{id0, id1}
		res, err := decodeResult(raw, ids)
		if err != nil {
			assert.Nil(t, res)
			return
		}
		require.NotNil(t, res)
		for _, r := range res.Records {
			assert.Contains(t, []Outcome{OutcomeFailed, OutcomeMatched, OutcomeNoMatch, OutcomeUnreported, OutcomeUndecodable}, r.Outcome)
			switch r.Association {
			case AssociationRequestID:
				require.NotNil(t, r.ItemIndex)
				assert.Equal(t, ids[*r.ItemIndex], r.Input.RequestID)
				assert.NotEqual(t, foldID(ids[0]), foldID(ids[1]), "a linked ID must be unique, even after folding")
			case AssociationSingleItem:
				require.Fail(t, "a two-item request is never linked by elimination")
			case AssociationAmbiguous, AssociationNone:
				assert.Nil(t, r.ItemIndex)
			default:
				require.Failf(t, "unknown association", "%q", r.Association)
			}
			if r.Outcome == OutcomeUndecodable {
				assert.Equal(t, Record{Association: AssociationNone, Outcome: OutcomeUndecodable}, r)
			}
		}
		testutil.RequireJSONRoundTrip(t, *res)
	})
}
