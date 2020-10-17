package benchmark

import (
	"context"
)

//Generator provides a standardized way to get successive benchmarks from a consistent interface
type Generator interface {
	Generator() bool
	GetBenchmark() (context.Context, Config, error)
	SetBenchmarkResults(Result) error
}
