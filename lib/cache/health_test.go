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

	"github.com/prometheus/client_golang/prometheus/testutil"
	"pgregory.net/rapid"
)

// TestMetricConverges verifies the shared health gauge against an independent
// model of cache state. Rapid varies the number of caches and the sequence of
// healthy, unhealthy, and closed transitions, checking the gauge after every
// transition.
func TestMetricConverges(t *testing.T) {
	rapid.Check(tt, func(rt *rapid.T) {
		m := &machine{}
		m.init(rt)
		rt.Repeat(rapid.StateMachineActions(m))
	})
}

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

	// Vary the cache count to exercise health aggregation across multiple
	// caches.
	for range rapid.IntRange(2, 6).Draw(t, "n") {
		// Construct Cache directly instead of calling newPackForAuth. Starting a Cache
		// launches goroutines that call setInitError, so direct construction keeps
		// health transitions under the state machine's control and makes the test
		// deterministic.
		ctx, cancel := context.WithCancel(context.Background())
		c := &Cache{
			Config: Config{
				target:         "foo",
				HealthReporter: healthReporter,
			},
			ctx:            ctx,
			cancel:         cancel,
			initC:          make(chan struct{}),
			firstTimeInitC: make(chan struct{}),
			eventsFanout:   services.NewFanoutV2(services.FanoutV2Config{}),
			lowVolumeEventsFanout: utils.NewRoundRobin([]*services.FanoutV2{
				services.NewFanoutV2(services.FanoutV2Config{})}),
		}

		c.setInitError(nil)
		m.up[c] = true
		m.healthyUp[c] = true

		m.all = append(m.all, c)
	}
}

// SetCacheHealthy selects a cache at random and reports it as healthy.
func (m *machine) SetCacheHealthy(t *rapid.T) {
	c := m.all[rapid.IntRange(0, len(m.all)-1).Draw(t, "i")]

	c.setInitError(nil)

	// If the cache is closed, update the cache itself by calling setInitError,
	// but don't bookkeep it as up.
	if m.closed[c] {
		return
	}

	m.up[c] = true
	m.healthyUp[c] = true
}

// SetCacheUnhealthy selects a cache at random and reports it as unhealthy.
func (m *machine) SetCacheUnhealthy(t *rapid.T) {
	c := m.all[rapid.IntRange(0, len(m.all)-1).Draw(t, "i")]

	c.setInitError(errors.New("unhealthy"))

	// If the cache is closed, update the cache itself by calling setInitError,
	// but don't bookkeep it as up.
	if m.closed[c] {
		return
	}

	m.up[c] = true
	delete(m.healthyUp, c)
}

// SetCacheDown selects a cache at random and closes it.
func (m *machine) SetCacheDown(t *rapid.T) {
	c := m.all[rapid.IntRange(0, len(m.all)-1).Draw(t, "i")]

	c.Close()

	m.closed[c] = true
	delete(m.up, c)
	delete(m.healthyUp, c)
}

// Check compares the reported gauge with the independent model after every
// state-machine action (SetCacheHealthy, SetCacheUnhealthy, SetCacheDown).
func (m *machine) Check(t *rapid.T) {
	for _, c := range m.all {
		got := testutil.ToFloat64(c.HealthReporter.gauge.WithLabelValues(c.target))
		want := m.expected()

		if got != want {
			t.Fatalf("got gauge %v, want: %v", got, want)
		}
	}
}

// expected derives the gauge value from the modeled cache state. A target is
// healthy when no caches are live or at least one live cache is healthy;
// otherwise, it is unhealthy.
func (m *machine) expected() float64 {
	if len(m.healthyUp) > 0 {
		return 1.0
	}
	return 0
}
