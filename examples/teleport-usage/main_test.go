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
	"time"

	"github.com/stretchr/testify/require"
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

func TestRateLimiterMinimumValue(t *testing.T) {
	limiter := newAdaptiveRateLimiter(5)

	for i := 0; i < 20; i++ {
		limiter.reportThrottleError()

		if limiter.currentCapacity() < 1 {
			t.Fatal("read capacity reached zero")
		}
	}
}

// TestDateSet tests that we query the appropriate set of days for an MAU calculation correctly.
func TestDateSet(t *testing.T) {
	startDates := []struct {
		startDate time.Time
		dates     []string
	}{
		{
			time.Date(2020, 5, 17, 0, 0, 0, 0, time.UTC),
			[]string{
				"2020-05-17",
				"2020-05-18",
				"2020-05-19",
				"2020-05-20",
				"2020-05-21",
				"2020-05-22",
				"2020-05-23",
				"2020-05-24",
				"2020-05-25",
				"2020-05-26",
				"2020-05-27",
				"2020-05-28",
				"2020-05-29",
				"2020-05-30",
				"2020-05-31",
				"2020-06-01",
				"2020-06-02",
				"2020-06-03",
				"2020-06-04",
				"2020-06-05",
				"2020-06-06",
				"2020-06-07",
				"2020-06-08",
				"2020-06-09",
				"2020-06-10",
				"2020-06-11",
				"2020-06-12",
				"2020-06-13",
				"2020-06-14",
				"2020-06-15",
			},
		},
		{
			time.Date(2005, 12, 24, 0, 0, 0, 0, time.UTC),
			[]string{
				"2005-12-24",
				"2005-12-25",
				"2005-12-26",
				"2005-12-27",
				"2005-12-28",
				"2005-12-29",
				"2005-12-30",
				"2005-12-31",
				"2006-01-01",
				"2006-01-02",
				"2006-01-03",
				"2006-01-04",
				"2006-01-05",
				"2006-01-06",
				"2006-01-07",
				"2006-01-08",
				"2006-01-09",
				"2006-01-10",
				"2006-01-11",
				"2006-01-12",
				"2006-01-13",
				"2006-01-14",
				"2006-01-15",
				"2006-01-16",
				"2006-01-17",
				"2006-01-18",
				"2006-01-19",
				"2006-01-20",
				"2006-01-21",
				"2006-01-22",
			},
		},
	}

	for _, c := range startDates {
		endDate := c.startDate.Add(scanDuration)
		require.Equal(t, c.dates, daysBetween(c.startDate, endDate))
	}
}
