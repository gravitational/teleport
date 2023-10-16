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
	MinDuration time.Duration
	MaxDuration time.Duration
	Step        int64
	Counter     atomic.Int64
}

// Duration calculates the current duration value.
func (v *VariableDuration) Duration() time.Duration {
	mul := float64(v.Counter.Load()) / float64(v.Step)
	if mul < 1 {
		mul = 1
	}

	d := time.Duration(float64(v.MinDuration) * math.Sqrt(mul))

	if d < v.MinDuration {
		d = v.MinDuration
	}

	if d > v.MaxDuration {
		d = v.MaxDuration
	}

	return d
}

// Inc increments the counter.
func (v *VariableDuration) Inc() {
	v.Counter.Add(1)
}

// Dec decrements the counter.
func (v *VariableDuration) Dec() {
	v.Counter.Add(-1)
}
