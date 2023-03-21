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

package main

import (
	"math"
	"testing"
)

// TestRateLimiter tests that the adaptive ratelimiter can adjust to the amount of free RCUs
// available as time goes on in a minimal amount of time.
//
// This is accomplished by defining an RCU capacity of 650 and calculating
// the amount of free RCUs we want to hit by subtracting a curve simulating varying usage in Teleport.
func TestRateLimiter(t *testing.T) {
	const dynamoCapacityRCUs = 650
	freeRCUs := func(step int) float64 {
		stepF := float64(step)
		usedByTeleport := 350 + math.Sin(stepF/10)*100 + math.Sin((stepF+100)/7)*50
		return dynamoCapacityRCUs - usedByTeleport
	}

	limiter := newAdaptiveRateLimiter(25)
	for step := 0; step < 300; step++ {
		freeRCUs := freeRCUs(step)
		t.Logf("step %v: free RCUs %v\n", step, freeRCUs)

		for i := 0; true; i++ {
			currentLimit := limiter.currentCapacity()
			if math.Abs(limiter.currentCapacity()-freeRCUs)/freeRCUs < 0.05 {
				t.Logf("adjusted limiter at step %v to target of %v, real value %v within %v iterations\n", step, freeRCUs, limiter.currentCapacity(), i)
				break
			}

			switch {
			case currentLimit < freeRCUs:
				limiter.adjustUp()
			case currentLimit > freeRCUs:
				limiter.reportThrottleError()
			}

			if i > 10 {
				t.Fatalf("too many iterations, lb: %v, hb: %v", limiter.low, limiter.high)
			}
		}
	}
}
