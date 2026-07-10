/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package cache

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestProperty_HealthReporter checks that, for any interleaving of health
// reports and removals across instances that may share a target, the reported
// gauge always matches the health derived from the instances that are still
// running:
//
//   - no series is reported when no instances are running for a target;
//   - the series is 1 when at least one running instance is healthy;
//   - the series is 0 only when every running instance is unhealthy.
//
// This is the invariant that prevents a shutting-down instance from leaving a
// stale value (in particular a stuck 0) on a target that another instance is
// still serving.
func TestProperty_HealthReporter(t *testing.T) {
	// Small pools so that instances frequently share a target and reuse ids,
	// which is exactly where clobbering would occur.
	targets := []string{"auth", "okta", "discovery"}
	const maxID = 4

	rapid.Check(t, func(t *rapid.T) {
		g := newTestHealthGauge()
		r := newHealthReporter(g)

		// model[target][id] = healthy, mirroring the reporter's own state.
		model := make(map[string]map[uint64]bool)

		checkInvariant := func() {
			for _, target := range targets {
				want, wantOK := deriveHealth(model[target])
				got, gotOK := readGauge(t, g, target)
				require.Equalf(t, wantOK, gotOK, "presence mismatch for target %q", target)
				if wantOK {
					require.Equalf(t, want, got, "value mismatch for target %q", target)
				}
			}
		}

		steps := rapid.IntRange(1, 50).Draw(t, "steps")
		for i := 0; i < steps; i++ {
			target := rapid.SampledFrom(targets).Draw(t, fmt.Sprintf("target-%d", i))
			id := uint64(rapid.IntRange(1, maxID).Draw(t, fmt.Sprintf("id-%d", i)))

			switch rapid.SampledFrom([]string{"healthy", "unhealthy", "remove"}).Draw(t, fmt.Sprintf("op-%d", i)) {
			case "healthy":
				r.setHealth(target, id, true)
				if model[target] == nil {
					model[target] = make(map[uint64]bool)
				}
				model[target][id] = true
			case "unhealthy":
				r.setHealth(target, id, false)
				if model[target] == nil {
					model[target] = make(map[uint64]bool)
				}
				model[target][id] = false
			case "remove":
				r.remove(target, id)
				delete(model[target], id)
				if len(model[target]) == 0 {
					delete(model, target)
				}
			}

			checkInvariant()
		}

		// Removing every remaining instance must leave no series behind.
		for target, instances := range model {
			for id := range instances {
				r.remove(target, id)
			}
		}
		for _, target := range targets {
			_, ok := readGauge(t, g, target)
			require.Falsef(t, ok, "series for target %q should be gone once all instances are removed", target)
		}
	})
}

// deriveHealth is the reference implementation of the reporter's aggregation:
// it returns the expected gauge value and whether a series should exist at all.
func deriveHealth(instances map[uint64]bool) (value float64, present bool) {
	if len(instances) == 0 {
		return 0, false
	}
	for _, healthy := range instances {
		if healthy {
			return 1, true
		}
	}
	return 0, true
}
