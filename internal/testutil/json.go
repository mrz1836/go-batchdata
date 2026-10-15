package testutil

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// RequireJSONRoundTrip marshals v, unmarshals the result into a new T and
// fails tb unless it equals v. msgAndArgs annotate a failure.
func RequireJSONRoundTrip[T any](tb testing.TB, v T, msgAndArgs ...any) {
	tb.Helper()
	data, err := json.Marshal(v)
	require.NoError(tb, err, msgAndArgs...)
	var back T
	require.NoError(tb, json.Unmarshal(data, &back), msgAndArgs...)
	require.Equal(tb, v, back, msgAndArgs...)
}
