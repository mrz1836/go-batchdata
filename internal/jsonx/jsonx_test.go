package jsonx_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/go-batchdata/internal/jsonx"
)

func TestShape(t *testing.T) {
	tests := []struct {
		name                   string
		raw                    string
		present, object, array bool
	}{
		{"empty", ``, false, false, false},
		{"whitespace only", " \t\r\n", false, false, false},
		{"null", `null`, false, false, false},
		{"null in whitespace", " null ", false, false, false},
		{"object", `{"a":1}`, true, true, false},
		{"object after whitespace", " \t\r\n{}", true, true, false},
		{"array", `[1]`, true, false, true},
		{"array after whitespace", "\n[]", true, false, true},
		{"string", `"{"`, true, false, false},
		{"number", `0`, true, false, false},
		{"literal", `false`, true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.present, jsonx.Present([]byte(tt.raw)), "Present")
			assert.Equal(t, tt.object, jsonx.IsObject([]byte(tt.raw)), "IsObject")
			assert.Equal(t, tt.array, jsonx.IsArray([]byte(tt.raw)), "IsArray")
		})
	}
}
