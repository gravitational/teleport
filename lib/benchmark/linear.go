/*
Copyright 2020 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package benchmark

import (
	"context"
	"errors"
	"log"
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
		if sleep {
			select {
			case <-time.After(PauseTimeBetweenBenchmarks):
			case <-ctx.Done():
				return results, trace.ConnectionProblem(ctx.Err(), "context canceled or timed out")
			}
		}
		benchmarkC, err := lg.GetBenchmark()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result, err = benchmarkC.Benchmark(ctx, tc)
		if err != nil {
			return results, trace.Wrap(err)
		}
		results = append(results, result)
		sleep = true
		log.Printf("The current generation made %v requests in %v.\n", result.RequestsOriginated, result.Duration)
	}
	return results, nil
}
