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

package interval

import (
	"math"
	"sync/atomic"
	"time"
)

// VariableDurationConfig holds input parameters for a variable duration.
type VariableDurationConfig struct {
	// MinDuration is the minimum duration value to be emitted when the counter
	// is at or near zero, and the base point of the scaling curve.
	MinDuration time.Duration
	// MaxDuration is the upper limit of the output duration. Once hit, further increases
	// to the counter have no effect.
	MaxDuration time.Duration
	// Step is the step size used to calculate the scaling curve. A larger step size results
	// in a slower/shallower increase in the output duration.
	Step int64
}

// VariableDuration is a duration generator that varies its output based on a
// counter. This type is used to implement intervals that should become less frequent
// as load increases, namely heartbeats. Typically, heartbeats should happen very frequently
// to ensure that changes become visible quickly. This becomes problematic at large scales,
// where the resource consumption of heartbeats becomes onerous. This type attempts to provide
// a reasonable compromise between frequency and scalabiltiy by slowly scaling up the duration
// as the associated counter is increased. Scaling is determined by the square root of the quotient
// of Counter / Step. Min and max duration should be selected based on the needs of the resource
// in question. How quickly/slowly the duration scales is then controlled by picking a step size,
// with larger step sizes resulting in slower scaling.
type VariableDuration struct {
	cfg     VariableDurationConfig
	counter atomic.Int64
}

// NewVariableDuration sets up a new variable duration instance.
func NewVariableDuration(cfg VariableDurationConfig) *VariableDuration {
	return &VariableDuration{
		cfg: cfg,
	}
}

// Duration calculates the current duration value.
func (v *VariableDuration) Duration() time.Duration {
	mul := max(float64(v.Count())/float64(v.cfg.Step), 1.0)

	d := time.Duration(float64(v.cfg.MinDuration) * math.Sqrt(mul))

	d = max(d, v.cfg.MinDuration)
	d = min(d, v.cfg.MaxDuration)

	return d
}

// Inc increments the counter.
func (v *VariableDuration) Inc() {
	v.counter.Add(1)
}

// Dec decrements the counter.
func (v *VariableDuration) Dec() {
	v.counter.Add(-1)
}

// Add adds to the counter. Can be used with a negative value to reduce the
// counter.
func (v *VariableDuration) Add(n int) {
	v.counter.Add(int64(n))
}

// Count loads the current counter value.
func (c *VariableDuration) Count() int64 {
	return c.counter.Load()
}
