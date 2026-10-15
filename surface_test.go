package batchdata_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mrz1836/go-batchdata"
)

func TestClientExportsOnlyStringAndGoString(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeFor[*batchdata.Client](), reflect.TypeFor[batchdata.Client]()} {
		names := make([]string, 0, typ.NumMethod())
		for i := range typ.NumMethod() {
			names = append(names, typ.Method(i).Name)
		}
		slices.Sort(names)
		assert.Equal(t, []string{"GoString", "String"}, names, "methods of %s", typ)
	}
}

func TestClientHasNoExportedFields(t *testing.T) {
	typ := reflect.TypeFor[batchdata.Client]()
	for i := range typ.NumField() {
		f := typ.Field(i)
		assert.False(t, f.IsExported(), "field %s must be unexported", f.Name)
	}
}

func TestZeroClientPrints(t *testing.T) {
	var c batchdata.Client
	assert.Equal(t, "batchdata.Client{}", c.String())
	assert.Equal(t, "batchdata.Client{}", c.GoString())
}
