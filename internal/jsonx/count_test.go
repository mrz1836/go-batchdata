package jsonx_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/jsonx"
)

// isObjectKey reports whether tok is a key of the innermost open object,
// flipping wantKey between keys and values as tokens pass.
func isObjectKey(wantKey *bool, object bool, tok json.Token) bool {
	if !object {
		return false
	}
	if !*wantKey {
		*wantKey = true
		return false
	}
	if _, isKey := tok.(string); isKey {
		*wantKey = false
		return true
	}
	return false
}

// countOracle counts containers and values with encoding/json's tokenizer,
// as an oracle for Count.
func countOracle(t testing.TB, data []byte) (containers, values int) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	type frame struct{ object, wantKey bool }
	var stack []frame
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return containers, values
		}
		require.NoError(t, err)
		if n := len(stack); n > 0 && isObjectKey(&stack[n-1].wantKey, stack[n-1].object, tok) {
			continue
		}
		switch tok {
		case json.Delim('{'), json.Delim('['):
			containers++
			values++
			stack = append(stack, frame{object: tok == json.Delim('{'), wantKey: tok == json.Delim('{')})
		case json.Delim('}'), json.Delim(']'):
			stack = stack[:len(stack)-1]
		default:
			values++
		}
	}
}

func TestCountJSON(t *testing.T) {
	tests := []struct {
		name              string
		data              string
		containers, value int
	}{
		{"empty object", `{}`, 1, 1},
		{"empty array", `[]`, 1, 1},
		{"scalars", `[1,-2.5e3,true,false,null,"s"]`, 1, 7},
		{"keys are not values", `{"a":1,"b":"x","c":{"d":[]}}`, 3, 5},
		{"escaped quotes and brackets in strings", `{"a\"{[":"}],\\\"","b":["{","]"]}`, 2, 5},
		{"whitespace", " { \"a\" : [ 1 , { } ] } ", 3, 4},
		{"nested arrays of objects", `{"data":[{},{},{"persons":[{},{}]}]}`, 8, 8},
		{"top-level string", `"x"`, 0, 1},
		{"top-level number", `12`, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, v := jsonx.Count([]byte(tt.data))
			assert.Equal(t, tt.containers, c)
			assert.Equal(t, tt.value, v)
			oc, ov := countOracle(t, []byte(tt.data))
			assert.Equal(t, oc, c, "oracle containers")
			assert.Equal(t, ov, v, "oracle values")
		})
	}
}

// TestCountJSONUnterminatedString checks that a string missing its closing
// quote, which valid JSON never has, ends the count at the end of the input.
func TestCountJSONUnterminatedString(t *testing.T) {
	tests := []struct {
		name              string
		data              string
		containers, value int
	}{
		{"string", `"abc`, 0, 1},
		{"string ending in a backslash", `"ab\`, 0, 1},
		{"string inside an object", `{"a":"b`, 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, v := jsonx.Count([]byte(tt.data))
			assert.Equal(t, tt.containers, c)
			assert.Equal(t, tt.value, v)
		})
	}
}

// FuzzCountJSON checks Count against the tokenizer oracle on every valid
// JSON input.
func FuzzCountJSON(f *testing.F) {
	for _, s := range []string{`{}`, `{"a":[1,"b",{"c":null}]}`, `[[],{},""]`, `{"\"":"\\"}`, `-1.5e10`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if !json.Valid(data) {
			return
		}
		c, v := jsonx.Count(data)
		oc, ov := countOracle(t, data)
		assert.Equal(t, oc, c)
		assert.Equal(t, ov, v)
	})
}
