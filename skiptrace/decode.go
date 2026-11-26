package skiptrace

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/jsonx"
)

// errMalformed reports a result object that is not the documented shape. The
// root package replaces it with its own error, so its text never surfaces.
var errMalformed = errors.New("skiptrace: malformed result")

// Decode decodes body, the bytes of an HTTP 200 response to a skip-trace call
// that the caller kept, exactly as Trace decodes the response it reads, and
// returns the same Result or the same *batchdata.Error. It makes no call and
// needs no client or token.
//
// req is the request the call sent. Decode reads only how many items it has
// and each item's RequestID, in order: they link records to items (see
// Record.Association). It never validates req, so a request that a later
// version refuses still decodes, and a request with no items links no record.
// Keep at least those IDs, in the order sent, with the body.
//
// Errors are those of batchdata.DecodeResponse, which also says what differs
// from a call. Decode never modifies body, and the Result shares no memory
// with it.
func Decode(body []byte, req Request) (*Result, error) {
	d := newDecoding(req.Items)
	if err := batchdata.DecodeResponse(body, d.decode); err != nil {
		return nil, err
	}
	return d.res, nil
}

// decoding is one decode of a skip-trace response: the request IDs that link
// its records to items, snapshotted from the request, and the Result once it
// is decoded. Trace and Decode both decode through one, so a kept body
// decodes exactly as the call decoded it.
type decoding struct {
	ids []string
	res *Result
}

// newDecoding snapshots each item's RequestID, in order.
func newDecoding(items []Item) *decoding {
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].RequestID
	}
	return &decoding{ids: ids}
}

// decode decodes the envelope's result object and keeps the envelope's
// warnings.
func (d *decoding) decode(result []byte, warnings []batchdata.Warning) error {
	r, err := decodeResult(result, d.ids)
	if err != nil {
		return err
	}
	r.Warnings = warnings
	d.res = r
	return nil
}

// wireResult is the response's result object.
type wireResult struct {
	Data    json.RawMessage `json:"data"`
	Options json.RawMessage `json:"options"`
	Meta    json.RawMessage `json:"meta"`
}

// wireRecord is one element of result.data.
type wireRecord struct {
	Input    Input      `json:"input"`
	Property *Property  `json:"property"`
	Persons  []Person   `json:"persons"`
	Meta     RecordMeta `json:"meta"`
}

// decodeResult decodes the result object strictly: data must be an array, and
// options and meta, when present and not null, must have the documented types.
// Each element of data is then decoded on its own, so one bad record never
// fails the others, and linked to the request items by ids, the request IDs
// snapshotted from the request.
func decodeResult(raw []byte, ids []string) (*Result, error) {
	var w wireResult
	if err := json.Unmarshal(raw, &w); err != nil || !jsonx.IsArray(w.Data) {
		return nil, errMalformed
	}
	var data []json.RawMessage
	if err := json.Unmarshal(w.Data, &data); err != nil {
		return nil, errMalformed
	}
	res := &Result{Records: make([]Record, len(data))}
	if jsonx.Present(w.Options) && json.Unmarshal(w.Options, &res.Options) != nil {
		return nil, errMalformed
	}
	if jsonx.Present(w.Meta) && json.Unmarshal(w.Meta, &res.Meta) != nil {
		return nil, errMalformed
	}
	for i, d := range data {
		res.Records[i] = decodeRecord(d)
	}
	associate(res.Records, ids)
	return res, nil
}

// decodeRecord decodes one element of result.data. A null, a value that is
// not an object, or a value of the wrong type anywhere inside yields a record
// with OutcomeUndecodable and every other field zero: nothing of it is kept.
func decodeRecord(raw json.RawMessage) Record {
	var w wireRecord
	if !jsonx.IsObject(raw) || json.Unmarshal(raw, &w) != nil {
		return Record{Association: AssociationNone, Outcome: OutcomeUndecodable}
	}
	return Record{
		Association: AssociationNone,
		Outcome:     outcome(w.Meta),
		Input:       w.Input,
		Property:    w.Property,
		Persons:     w.Persons,
		Meta:        w.Meta,
	}
}

// outcome applies the precedence error, then matched, then absent.
func outcome(m RecordMeta) Outcome {
	switch {
	case m.Error != nil && *m.Error:
		return OutcomeFailed
	case m.Matched == nil:
		return OutcomeUnreported
	case *m.Matched:
		return OutcomeMatched
	default:
		return OutcomeNoMatch
	}
}

// associate links records to request items by evidence only: an echoed
// request ID that is unique among the items, or, for a one-item request
// answered by exactly one decodable record, elimination. Position, names and
// addresses are never used.
//
// An echo is compared exactly, but an ID is unique only when no other item's
// ID equals it after trimming surrounding whitespace and folding case: should
// BatchData ever normalize its echo, "row-1" and "row-1 " would otherwise
// link a record to the wrong item.
func associate(records []Record, ids []string) {
	owners := make(map[string][]int, len(ids))
	similar := make(map[string]int, len(ids))
	for i, id := range ids {
		if id != "" {
			owners[id] = append(owners[id], i)
			similar[foldID(id)]++
		}
	}
	decodable := 0
	for i := range records {
		if records[i].Outcome != OutcomeUndecodable {
			decodable++
		}
	}
	single := len(ids) == 1 && decodable == 1
	for i := range records {
		r := &records[i]
		if r.Outcome == OutcomeUndecodable {
			continue
		}
		echo := r.Input.RequestID
		switch idx := owners[echo]; {
		case echo != "" && len(idx) == 1 && similar[foldID(echo)] == 1:
			r.Association, r.ItemIndex = AssociationRequestID, new(idx[0])
		case echo != "" && len(idx) > 0:
			r.Association = AssociationAmbiguous
		case single && (ids[0] == "" || echo == ""):
			r.Association, r.ItemIndex = AssociationSingleItem, new(0)
		}
	}
}

// foldID is the form in which two request IDs count as the same for
// uniqueness: surrounding whitespace trimmed and case folded.
func foldID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}
