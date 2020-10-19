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
	currentRPS          int
	config              Config
}

// Generate advances the Generator to the next generation.
// It returns false when the generator no longer has configurations to run.
func (l *Linear) Generate() bool {
	if l.currentRPS < l.LowerBound {
		l.currentRPS = l.LowerBound
		return true
	}
	l.currentRPS += l.Step
	if l.currentRPS > l.UpperBound {
		return false
	}
	return true
}

// GetBenchmark returns the benchmark config for the current generation.
// If called after Generate() returns false, this will result in an error.
func (l *Linear) GetBenchmark() (context.Context, Config, error) {
	if l.currentRPS > l.UpperBound {
		return nil, Config{}, errors.New("No more generations")
	}

	return context.TODO(), Config{
		MinimumWindow:       l.MinimumWindow,
		MinimumMeasurements: l.MinimumMeasurements,
		Rate:                l.currentRPS,
		Threads:             10,
		Duration:            0,
		Command:             l.config.Command,
	}, nil
}

// Benchmark runs the benchmark of reciever type
// return an array of Results that contain information about the generations
func (l *Linear) Benchmark(command []string, tc *client.TeleportClient) ([]*Result, error) {
	var result *Result
	var results []*Result

	for l.Generate() {
		c, benchmarkC, err := l.GetBenchmark()
		if err != nil {
			break
		}
		result, err = benchmarkC.ProgressiveBenchmark(c, tc)
		if err != nil {
			return results, err
		}
		results = append(results, result)
		fmt.Printf("current generation requests: %v, duration: %v\n", result.RequestsOriginated, result.Duration)
		time.Sleep(5 * time.Second)
	}
	return results, nil
}
