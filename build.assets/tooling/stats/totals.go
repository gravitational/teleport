/*
Copyright 2022 Gravitational, Inc.

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
	"sort"
	"time"
)

// CheckRunTotal represents total stats on a specific check run
type CheckRunTotal struct {
	Name              string
	AppName           string
	FirstRun          time.Time
	LastRun           time.Time
	AverageElapsed    time.Duration
	TotalRuns         int
	Failures          int
	FailurePercentage float64
	Runs              []CheckRun
}

// Totals represents overall totals
type Totals struct {
	FirstRun       time.Time
	LastRun        time.Time
	CheckRunTotals []CheckRunTotal
}

// newTotals creates totals struct from the array of PRs
func newTotals(prs []PR) Totals {
	var firstRun, lastRun = newTotalsTimeFrame(prs)

	var runs = make([]CheckRun, 0)

	for _, pr := range prs {
		runs = append(runs, pr.Runs...)
	}

	// Sort runs in reverse order (newest to oldest), it will be required for latest failures
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.UnixNano() > runs[j].StartedAt.UnixNano()
	})

	var groups = make(map[string][]CheckRun)

	for _, run := range runs {
		group, ok := groups[run.Key()]
		if !ok {
			group = make([]CheckRun, 0)
		}

		group = append(group, run)

		groups[run.Key()] = group
	}

	maxTotalRuns := 0

	for _, runs := range groups {
		if len(runs) > maxTotalRuns {
			maxTotalRuns = len(runs)
		}
	}

	checkRunTotals := make([]CheckRunTotal, 0)

	for _, group := range groups {
		var firstRun, lastRun time.Time
		var totalElapsed time.Duration

		firstRun = time.Now()

		failures := 0

		for _, run := range group {
			if run.StartedAt.UnixNano() < firstRun.UnixNano() {
				firstRun = run.StartedAt
			}

			if run.StartedAt.UnixNano() > lastRun.UnixNano() {
				lastRun = run.StartedAt
			}

			totalElapsed = totalElapsed + run.Elapsed
			if run.Conclusion != "SUCCESS" {
				failures++
			}
		}

		failurePercentage := (float64(failures) / float64(maxTotalRuns)) * 100

		checkRunTotals = append(checkRunTotals, CheckRunTotal{
			Name:              group[0].Name,
			AppName:           group[0].AppName,
			FirstRun:          firstRun,
			LastRun:           lastRun,
			AverageElapsed:    time.Duration(int64(totalElapsed) / int64(len(group))),
			TotalRuns:         len(group),
			Failures:          failures,
			FailurePercentage: failurePercentage,
			Runs:              group,
		})
	}

	return Totals{
		FirstRun:       firstRun,
		LastRun:        lastRun,
		CheckRunTotals: checkRunTotals,
	}
}

// newTotalsTimeFrame returns time frame from the array of PRs
func newTotalsTimeFrame(prs []PR) (time.Time, time.Time) {
	var firstRun, lastRun time.Time

	firstRun = time.Now()

	// Get the time frame
	for _, pr := range prs {
		if pr.MergedAt.UnixNano() < firstRun.UnixNano() {
			firstRun = pr.MergedAt
		}

		if pr.MergedAt.UnixNano() > lastRun.UnixNano() {
			lastRun = pr.MergedAt
		}
	}

	return firstRun, lastRun
}
