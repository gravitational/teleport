package benchmark

import (
	"errors"
	"time"
)

// Linear generator
type Linear struct {
	// LowerBound is the lower end of rps to execute
	LowerBound int
	// UpperBound is the upper end of rps to execute
	UpperBound int
	// Step is the amount of rps to increment by
	Step int
	// MinimumMeasurements is the minimum measurement a benchmark should execute
	MinimumMeasurements int
	// MinimumWindow is the minimum duration to run benchmark for
	MinimumWindow time.Duration
	// Threads is amount of concurrent execution threads to run
	Threads    int
	currentRPS int
	config     Config
}

// GetBenchmark returns the benchmark config for the current generation.
func (lg *Linear) GetBenchmark() (Config, error) {
	cnf := Config{
		MinimumWindow:       lg.MinimumWindow,
		MinimumMeasurements: lg.MinimumMeasurements,
		Rate:                lg.currentRPS,
		Threads:             lg.Threads,
		Command:             lg.config.Command,
	}

	if lg.currentRPS < lg.LowerBound {
		lg.currentRPS = lg.LowerBound
		cnf.Rate = lg.currentRPS
		return cnf, nil
	}

	lg.currentRPS += lg.Step
	cnf.Rate = lg.currentRPS
	if lg.currentRPS > lg.UpperBound {
		return Config{}, errors.New("no more generations")
	}
	return cnf, nil
}
