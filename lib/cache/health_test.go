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
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"pgregory.net/rapid"
)

type machine struct {
	up        map[*Cache]bool
	healthyUp map[*Cache]bool

	all []*Cache
}

func (m *machine) init(t *rapid.T) {

	//c1 := newPackForAuth(t)
	//t.Cleanup(c1.Close)

	// will need to reset this since this is process global?
	cacheHealth.Reset()

	// Number of caches can be be
	// TODO(russjones): Is this [2, 4] or [2, 4)?
	for i := range rapid.IntRange(2, 4).Draw(t, "n") {

		c := &Cache{}
		c.setInitError(nil)
		m.all = append(m.all, &Cache{})
	}

}

// SetCacheHealthy will pick a random cache from the slice of caches
// and set it's state to healthy.
func (m *machine) SetCacheHealthy(t *rapid.T) {
	c := m.all[rapid.IntRange(0, len(m.all)-1).Draw(t, "i")]

	c.setInitError(nil)

	m.up[c] = true
	m.healthyUp[c] = true
}

// SetCacheUnhealthy will pick a random cache from the slice of caches
// and set it's state to unhealthy.
func (m *machine) SetCacheUnhealthy(t *rapid.T) {
	c := m.all[rapid.IntRange(0, len(m.all)-1).Draw(t, "i")]

	c.setInitError(errors.New("unhealthy"))

	m.up[c] = true
	delete(m.healthyUp, c)
}

func (m *machine) SetCacheDown(t *rapid.T) {
	c := m.all[rapid.IntRange(0, len(m.all)-1).Draw(t, "i")]

	c.Close()

	delete(m.up, c)
	delete(m.healthyUp, c)
}

func (m *machine) expected() float64 {
	if len(m.up) == 0 || len(m.healthyUp) > 0 {
		return 1.0
	}
	return 0
}

func (m *machine) Check(t *rapid.T) {
	got := testutil.ToFloat64(cacheHealth.WithLabelValues("auth"))
	want := m.expected()

	if got != want {
		t.Fatal("got gauge %v, want: %v", got, want)
	}
}

func TestMetricConverges(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := &machine{}
		m.init(t)
		t.Repeat(rapid.StateMachineActions(m))
	})
}

//// TODO(russjones): Can t.Parallel() be set?
//func TestMetricEventuallyConverges(t *testing.T) {
//	 wanconst target = "auth"
//
//	c1 := newPackForAuth(t)
//	t.Cleanup(c1.Close)
//	c2 := newPackForAuth(t)
//	t.Cleanup(c2.Close)
//
//	require.Equal(t, 1.0, testutil.ToFloat64(cacheHealth.WithLabelValues(target)),
//		"both caches are healthy, metric should report healthy")
//
//	c1.cache.setInitError(errors.New("cache shutting down"))
//
//	require.Equal(t, 1.0, testutil.ToFloat64(cacheHealth.WithLabelValues(target)),
//		"a healthy cache (c2) is still running, metric should report healthy")
//}
