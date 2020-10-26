package benchmark

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
)

// Linear generator
type Linear struct {
	LowerBound          int
	UpperBound          int
	Step                int
	MinimumMeasurements int
	MinimumWindow       time.Duration
	currentRPS          int
	config              Config
}

// Generate advances the Generator to the next generation.
func (lg *Linear) Generate() bool {
	if lg.currentRPS < lg.LowerBound {
		lg.currentRPS = lg.LowerBound
		return true
	}
	lg.currentRPS += lg.Step
	return lg.currentRPS <= lg.UpperBound
}

// GetBenchmark returns the benchmark config for the current generation.
func (lg *Linear) GetBenchmark() (Config, error) {
	if lg.currentRPS > lg.UpperBound {
		return Config{}, errors.New("No more generations")
	}

	return Config{
		MinimumWindow:       lg.MinimumWindow,
		MinimumMeasurements: lg.MinimumMeasurements,
		Rate:                lg.currentRPS,
		Command:             lg.config.Command,
	}, nil
}

// Benchmark runs the benchmark of receiver type
func (lg *Linear) Benchmark(ctx context.Context, cmd string, tc *client.TeleportClient) ([]Result, error) {
	var result Result
	var results []Result
	c := strings.Split(cmd, " ")
	lg.config.Command = c
	sleep := false

	for lg.Generate() {
		// artificial pause between generations to allow remote state to pause
		if sleep {
			time.Sleep(PauseTimeBetweenBenchmarks)
		}
		benchmarkC, err := lg.GetBenchmark()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result, err = benchmarkC.Benchmark(context.TODO(), tc)
		if err != nil {
			return results, trace.Wrap(err)
		}
		results = append(results, result)
		fmt.Printf("current generation requests: %v, duration: %v\n", result.RequestsOriginated, result.Duration)
		sleep = true
	}
	return results, nil
}
