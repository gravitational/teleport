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
	currentRPS    int
	config        *Config
}

// GetBenchmark returns the benchmark config for the current generation.
func (lg *Linear) GetBenchmark() *Config {
	cnf := &Config{
		MinimumWindow:       lg.MinimumWindow,
		MinimumMeasurements: lg.MinimumMeasurements,
		Interval:                time.Duration(1 / float64(lg.currentRPS) * float64(time.Second)),
	}

	if lg.currentRPS < lg.LowerBound {
		lg.currentRPS = lg.LowerBound
		cnf.Interval = time.Duration(1 / float64(lg.currentRPS) * float64(time.Second))
		return cnf
	}

	lg.currentRPS += lg.Step
	cnf.Interval = time.Duration(1 / float64(lg.currentRPS) * float64(time.Second))
	if lg.currentRPS > lg.UpperBound {
		return nil
	}
	return cnf
}

func validateConfig(lg *Linear) error {
	if lg.MinimumMeasurements <= 0 || lg.UpperBound <= 0 || lg.LowerBound <= 0 || lg.Step <= 0 {
		return errors.New("minimumMeasurements, upperbound, step, and lowerBound must be greater than 0")
	}
	if lg.LowerBound > lg.UpperBound {
		return errors.New("upperbound must be greater than lowerbound")
	}
	return nil
}
