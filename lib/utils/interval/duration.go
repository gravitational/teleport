/*
Copyright 2023 Gravitational, Inc.

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

// Count loads the current counter value.
func (c *VariableDuration) Count() int64 {
	return c.counter.Load()
}
