package call_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata/internal/call"
)

// holder embeds a Handle the way the root Client does.
type holder struct {
	call.Handle[string]
}

func TestHandleHasNoExportedMembers(t *testing.T) {
	typ := reflect.TypeFor[call.Handle[string]]()
	assert.Equal(t, 0, typ.NumMethod(), "Handle must export no methods")
	assert.Equal(t, 0, reflect.PointerTo(typ).NumMethod(), "*Handle must export no methods")
	for i := range typ.NumField() {
		assert.False(t, typ.Field(i).IsExported(), "field %s must be unexported", typ.Field(i).Name)
	}
}

func TestFuncOfReturnsTheHeldSender(t *testing.T) {
	var got call.Call[string]
	send := func(_ context.Context, c call.Call[string]) error {
		got = c
		return nil
	}
	h := holder{Handle: call.NewHandle[string](send)}
	s := call.FuncOf[string](h)
	require.NotNil(t, s)
	require.NoError(t, s(t.Context(), call.Call[string]{Path: "/p", Body: []byte("b")}))
	assert.Equal(t, "/p", got.Path)
	assert.Equal(t, []byte("b"), got.Body)

	ps := call.FuncOf[string](&h)
	require.NotNil(t, ps, "a pointer to a holder satisfies Holder too")
}

func TestFuncOfNilAndZero(t *testing.T) {
	t.Run("nil interface", func(t *testing.T) {
		assert.Nil(t, call.FuncOf[string](nil))
	})
	t.Run("zero handle", func(t *testing.T) {
		assert.Nil(t, call.FuncOf[string](holder{}))
	})
}
