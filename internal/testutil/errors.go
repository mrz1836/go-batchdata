package testutil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// RequireErrorAs returns the first error in err's chain of type E, failing tb
// unless err is non-nil and has one.
func RequireErrorAs[E error](tb testing.TB, err error) E {
	tb.Helper()
	require.Error(tb, err)
	e, ok := errors.AsType[E](err)
	require.True(tb, ok, "want a %T, got %T", e, err)
	return e
}
