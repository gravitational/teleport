package fallback

import "testing"

// BenchmarkInitTime benchmarks the time it takes to parse all certificates
// in this bundle, it corresponds to the init time of this package.
func BenchmarkInitTime(b *testing.B) {
	for range b.N {
		newFallbackCertPool()
	}
}
