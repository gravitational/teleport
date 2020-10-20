package benchmark

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/client"
)

// Linear generator
type Linear struct {
	LowerBound          int
	UpperBound          int
	Step                int
	MinimumMeasurements int
	MinimumWindow       time.Duration
	Threads             int
	currentRPS          int
	config              Config
}

// Generate advances the Generator to the next generation.
// It returns false when the generator no longer has configurations to run.
func (lg *Linear) Generate() bool {
	if lg.currentRPS < lg.LowerBound {
		lg.currentRPS = lg.LowerBound
		return true
	}
	lg.currentRPS += lg.Step
	return lg.currentRPS <= lg.UpperBound
}

// GetBenchmark returns the benchmark config for the current generation.
// If called after Generate() returns false, this will result in an error.
func (lg *Linear) GetBenchmark() (context.Context, Config, error) {
	if lg.currentRPS > lg.UpperBound {
		return nil, Config{}, errors.New("No more generations")
	}

	return context.TODO(), Config{
		MinimumWindow:       lg.MinimumWindow,
		MinimumMeasurements: lg.MinimumMeasurements,
		Rate:                lg.currentRPS,
		Threads:             lg.Threads,
		Command:             lg.config.Command,
	}, nil
}

// Benchmark runs the benchmark of receiver type
// return an array of Results that contain information about the generations
func (lg *Linear) Benchmark(command []string, tc *client.TeleportClient) ([]*Result, error) {
	var result *Result
	var results []*Result
	sleep := false
	for lg.Generate() {
		if sleep {
			// artificial pause between generations to allow remote state to pause
			time.Sleep(PauseTimeBetweenBenchmarks)
		}
		c, benchmarkC, err := lg.GetBenchmark()
		if err != nil {
			break
		}
		result, err = benchmarkC.Benchmark(c, tc)
		if err != nil {
			return results, err
		}
		results = append(results, result)
		fmt.Printf("current generation requests: %v, duration: %v\n", result.RequestsOriginated, result.Duration)
		sleep = true
	}
	return results, nil
}
