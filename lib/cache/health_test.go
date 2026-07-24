// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"pgregory.net/rapid"
)

type machine struct {
	up        map[*Cache]bool
	healthyUp map[*Cache]bool
	closed    map[*Cache]bool

	all []*Cache
}

func (m *machine) init(t *rapid.T) {
	m.up = map[*Cache]bool{}
	m.healthyUp = map[*Cache]bool{}
	m.closed = map[*Cache]bool{}

	healthReporter, err := NewHealthReporter(metrics.NoopRegistry())
	if err != nil {
		t.Fatalf("Failed to create health reporter: %v.", err)
	}

	// Create 2, 3, or 4 caches.
	for range rapid.IntRange(2, 4).Draw(t, "n") {
		// Create a Cachedirectly instead of using newPackForAuth because
		// starting a Cache launches goroutines that call setInitError. To make
		// this test deterministic, only this test should call setInitError.
		ctx, cancel := context.WithCancel(context.Background())
		c := &Cache{
			Config: Config{
				target:         "auth",
				HealthReporter: healthReporter,
			},
			ctx:                   ctx,
			cancel:                cancel,
			initC:                 make(chan struct{}),
			firstTimeInitC:        make(chan struct{}),
			eventsFanout:          services.NewFanoutV2(services.FanoutV2Config{}),
			lowVolumeEventsFanout: utils.NewRoundRobin([]*services.FanoutV2{services.NewFanoutV2(services.FanoutV2Config{})}),
		}

		c.setInitError(nil)
		m.up[c] = true
		m.healthyUp[c] = true

		m.all = append(m.all, c)
	}
}

// SetCacheHealthy will pick a random cache from the slice of caches
// and set it's state to healthy.
func (m *machine) SetCacheHealthy(t *rapid.T) {
	c := m.all[rapid.IntRange(0, len(m.all)-1).Draw(t, "i")]

	c.setInitError(nil)

	// If the cache is closed, we can toggle setInitError as much as we want,
	// but it won't ever be a up or health cache.
	if m.closed[c] {
		return
	}

	m.up[c] = true
	m.healthyUp[c] = true
}

// SetCacheUnhealthy will pick a random cache from the slice of caches
// and set it's state to unhealthy.
func (m *machine) SetCacheUnhealthy(t *rapid.T) {
	c := m.all[rapid.IntRange(0, len(m.all)-1).Draw(t, "i")]

	c.setInitError(errors.New("unhealthy"))

	// If the cache is closed, we can toggle setInitError as much as we want,
	// but it won't ever be a up or health cache.
	if m.closed[c] {
		return
	}

	m.up[c] = true
	delete(m.healthyUp, c)
}

// SetCacheDown will pick a random cache from the slice of caches and close it
// to shut it down.
func (m *machine) SetCacheDown(t *rapid.T) {
	c := m.all[rapid.IntRange(0, len(m.all)-1).Draw(t, "i")]

	c.Close()

	m.closed[c] = true
	delete(m.up, c)
	delete(m.healthyUp, c)
}

// expected is the health property that enforces correctness. The metric is
// healthy if any cache is up or no cache is up. Otherwise it's unhealthy.
func (m *machine) expected() float64 {
	if len(m.up) == 0 || len(m.healthyUp) > 0 {
		return 1.0
	}
	return 0
}

// Check enforces the state machine invariant. It's run after every action
// (SetCacheHealthy, SetCacheUnhealthy, SetCacheDown) and asserts the expected
// value matches the received value of the gauge.
func (m *machine) Check(t *rapid.T) {
	for _, c := range m.all {
		got := c.HealthReporter.anyHealthy(c.target)
		want := m.expected()

		if got != want {
			t.Fatalf("got gauge %v, want: %v", got, want)
		}
	}
}

// TestMetricConverges ensures that caches correctly report their health to the
// prometheus metric.
func TestMetricConverges(tt *testing.T) {
	rapid.Check(tt, func(rt *rapid.T) {
		m := &machine{}
		m.init(rt)
		rt.Repeat(rapid.StateMachineActions(m))
	})
}
