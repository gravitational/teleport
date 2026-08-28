/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
		Rate:                lg.currentRPS,
	}

	if lg.currentRPS < lg.LowerBound {
		lg.currentRPS = lg.LowerBound
		cnf.Rate = lg.currentRPS
		return cnf
	}

	lg.currentRPS += lg.Step
	cnf.Rate = lg.currentRPS
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
