// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package breaker

import (
	"time"

	"github.com/jonboulle/clockwork"
)

// ratioController allows executing portions of the provided functions,
// increasing the amount of passed request using the following linear function:
//
//   allowedRequestsRatio = 0.5 * (Now() - Start())/Duration
//
type ratioController struct {
	duration time.Duration
	start    time.Time
	clock    clockwork.Clock
	allowed  int
	denied   int
}

func newRatioController(clock clockwork.Clock, rampUp time.Duration) *ratioController {
	return &ratioController{
		duration: rampUp,
		clock:    clock,
		start:    clock.Now().UTC(),
	}
}

func (r *ratioController) allowRequest() bool {
	t := r.targetRatio()
	// This condition answers the question - would we satisfy the target ratio if we allow this request?
	e := r.computeRatio(r.allowed+1, r.denied)
	if e < t {
		r.allowed++
		return true
	}
	r.denied++
	return false
}

func (r *ratioController) computeRatio(allowed, denied int) float64 {
	if denied+allowed == 0 {
		return 0
	}
	return float64(allowed) / float64(denied+allowed)
}

func (r *ratioController) targetRatio() float64 {
	// Here's why it's 0.5:
	// We are watching the following ratio
	// ratio = a / (a + d)
	// We can notice, that once we get to 0.5
	// 0.5 = a / (a + d)
	// we can evaluate that a = d
	// that means equilibrium, where we would allow all the requests
	// after this point to achieve ratio of 1 (that can never be reached unless d is 0)
	// so we stop from there
	multiplier := 0.5 / float64(r.duration)
	return multiplier * float64(r.clock.Now().UTC().Sub(r.start))
}
