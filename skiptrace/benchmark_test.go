package skiptrace_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/go-batchdata"
	"github.com/mrz1836/go-batchdata/internal/testutil"
	"github.com/mrz1836/go-batchdata/skiptrace"
)

// hundred returns a 100-item request and a response answering every item
// with a matched record that echoes its request ID.
func hundred() (skiptrace.Request, string) {
	req := skiptrace.Request{Items: make([]skiptrace.Item, skiptrace.MaxItems)}
	recs := make([]string, skiptrace.MaxItems)
	for i := range req.Items {
		id := fmt.Sprintf("row-%03d", i)
		req.Items[i] = skiptrace.Item{
			RequestID:       id,
			PropertyAddress: skiptrace.Address{Street: fmt.Sprintf("%d Example Ave", 100+i), City: "Testville", State: "ZZ", Zip: "00000"},
			Name:            skiptrace.Name{First: "Sample", Last: "Person"},
		}
		recs[i] = matched(id)
	}
	return req, envelope(recs...)
}

func BenchmarkTrace100(b *testing.B) {
	req, body := hundred()
	tracer, _ := memoryTracer(b, reply(http.StatusOK, body))
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	for b.Loop() {
		if _, err := tracer.Trace(b.Context(), req); err != nil {
			b.Fatal(err)
		}
	}
}

func TestValidateDoesNotAllocate(t *testing.T) {
	testutil.SkipUnderRace(t)
	req, _ := hundred()
	allocs, _ := testutil.AllocStats(20, func() {
		if err := req.Validate(); err != nil {
			panic(err)
		}
	})
	assert.Zero(t, allocs, "Validate of a valid request must not allocate")
}

func TestTraceMemoryIsBoundedByTheResponse(t *testing.T) {
	testutil.SkipUnderRace(t)
	req, body := hundred()
	tracer, _ := memoryTracer(t, reply(http.StatusOK, body))
	_, bytesPerCall := testutil.AllocStats(10, func() {
		if _, err := tracer.Trace(t.Context(), req); err != nil {
			panic(err)
		}
	})
	t.Logf("%d-byte response: %.0f bytes allocated per call (%.1fx)", len(body), bytesPerCall, bytesPerCall/float64(len(body)))
	require.Positive(t, bytesPerCall)
	assert.Less(t, bytesPerCall, 12*float64(len(body)), "a call must not hold many copies of the response")
}

// fill returns prefix, then copies of item joined by commas until the body
// nears the default response limit, then suffix.
func fill(prefix, item, suffix string) string {
	n := (batchdata.DefaultMaxResponseBytes - len(prefix) - len(suffix) - 1024) / (len(item) + 1)
	return prefix + strings.TrimSuffix(strings.Repeat(item+",", n), ",") + suffix
}

func TestTraceMemoryIsBoundedForHostileShapes(t *testing.T) {
	testutil.SkipUnderRace(t)
	tests := []struct {
		name string
		body string
	}{
		{"millions of empty records", fill(`{"result":{"data":[`, `{}`, `]}}`)},
		{"millions of empty persons", fill(`{"result":{"data":[{"persons":[`, `{}`, `]}]}}`)},
		{"millions of empty addresses", fill(`{"result":{"data":[{"persons":[{"addresses":[`, `{}`, `]}]}]}}`)},
		{"millions of empty strings", fill(`{"result":{"data":[{"input":{"propertyAddress":{"cityAliases":[`, `""`, `]}}}]}}`)},
		{"millions of warnings in a rejection", fill(`{"status":{"code":429,"warnings":[`, `{}`, `]}}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Less(t, len(tt.body), batchdata.DefaultMaxResponseBytes)
			tracer, _ := memoryTracer(t, reply(http.StatusOK, tt.body))
			var err error
			_, allocated := testutil.AllocStats(1, func() {
				_, err = tracer.Trace(t.Context(), oneRequest())
			})
			e := testutil.RequireErrorAs[*batchdata.Error](t, err)
			assert.Equal(t, batchdata.KindMalformedResponse, e.Kind)
			t.Logf("%d-byte body: %.1f MiB allocated", len(tt.body), allocated/(1<<20))
			assert.Less(t, allocated, float64(4*len(tt.body)), "rejecting a hostile shape must cost about the body itself")
		})
	}
}
