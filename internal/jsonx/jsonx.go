// Package jsonx holds the byte-level JSON checks that the root batchdata
// package and the service packages share: the shape of a raw value, and a
// count of the containers and values in a document.
//
// It is neutral: it never imports the root batchdata package, and it works
// only on input that is already valid JSON, as json.Valid or json.Unmarshal
// has checked it.
package jsonx

import "bytes"

// Present reports whether raw holds a value other than null.
func Present(raw []byte) bool {
	v := bytes.TrimSpace(raw)
	return len(v) > 0 && !bytes.Equal(v, []byte("null"))
}

// IsObject reports whether the valid JSON value raw is an object.
func IsObject(raw []byte) bool { return firstByte(raw) == '{' }

// IsArray reports whether the valid JSON value raw is an array.
func IsArray(raw []byte) bool { return firstByte(raw) == '[' }

// Count counts the objects and arrays (containers) and all values (keys
// excluded) in data, which must be valid JSON. It allocates only its nesting
// stack, which json.Valid has already bounded.
func Count(data []byte) (containers, values int) {
	var objects []bool // per open container: whether it is an object
	expectKey := false
	for i := 0; i < len(data); i++ {
		switch c := data[i]; c {
		case '{', '[':
			containers++
			values++
			objects = append(objects, c == '{')
			expectKey = c == '{'
		case '}', ']':
			objects = objects[:len(objects)-1]
			expectKey = false
		case ',':
			expectKey = len(objects) > 0 && objects[len(objects)-1]
		case ':':
			expectKey = false
		case '"':
			i = stringEnd(data, i)
			if !expectKey {
				values++
			}
			expectKey = false
		case ' ', '\t', '\n', '\r':
		default:
			// A number or a literal: count it once and skip to its end.
			values++
			for i+1 < len(data) && !delimiter(data[i+1]) {
				i++
			}
		}
	}
	return containers, values
}

// stringEnd returns the index of the quote closing the string that starts at
// data[start].
func stringEnd(data []byte, start int) int {
	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return len(data)
}

// delimiter reports whether c ends a number or a literal.
func delimiter(c byte) bool {
	switch c {
	case ',', ']', '}', ' ', '\t', '\n', '\r':
		return true
	}
	return false
}

// firstByte returns the first byte of raw that is not JSON whitespace, or 0.
func firstByte(raw []byte) byte {
	for _, c := range raw {
		switch c {
		case ' ', '\t', '\n', '\r':
		default:
			return c
		}
	}
	return 0
}
