package testutil

import (
	"runtime"
	"testing"
)

// SkipUnderRace skips the test when the race detector is on, because it
// changes allocation counts and sizes.
func SkipUnderRace(tb testing.TB) {
	tb.Helper()
	if RaceEnabled {
		tb.Skip("allocation counts and sizes differ under the race detector")
	}
}

// AllocStats reports the average number of allocations and of bytes allocated
// by one run of f, measured like testing.AllocsPerRun: one warm-up run, then
// runs runs with GOMAXPROCS set to 1. Unlike AllocsPerRun, it keeps fractions
// and also reports bytes.
func AllocStats(runs int, f func()) (allocs, allocated float64) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	f()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range runs {
		f()
	}
	runtime.ReadMemStats(&after)
	return float64(after.Mallocs-before.Mallocs) / float64(runs), float64(after.TotalAlloc-before.TotalAlloc) / float64(runs)
}
