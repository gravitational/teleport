package backend

import (
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestReporterTopRequestsLimit(t *testing.T) {
	// Test that a Reporter deletes older requests from metrics to limit memory
	// usage. For this test, we'll keep 10 requests.
	const topRequests = 10
	r, err := NewReporter(ReporterConfig{
		Backend:          &nopBackend{},
		Component:        "test",
		TopRequestsCount: topRequests,
	})
	require.NoError(t, err)

	countTopRequests := func() int {
		ch := make(chan prometheus.Metric)
		go func() {
			requests.Collect(ch)
			close(ch)
		}()

		var count int64
		for range ch {
			atomic.AddInt64(&count, 1)
		}
		return int(count)
	}

	// At first, the metric should have no values.
	require.Equal(t, 0, countTopRequests())

	// Run through 1000 unique keys.
	for i := 0; i < 1000; i++ {
		r.trackRequest(OpGet, []byte(strconv.Itoa(i)), nil)
	}

	// Now the metric should have only 10 of the keys above.
	require.Equal(t, topRequests, countTopRequests())
}
