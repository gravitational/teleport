/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package track

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type simpleTestProxies struct {
	sync.Mutex
	proxies []testProxy
}

func (s *simpleTestProxies) AddRandProxies(n int, min time.Duration, max time.Duration) {
	s.Lock()
	defer s.Unlock()
	for i := 0; i < n; i++ {
		proxy := newTestProxy(prDuration(min, max))
		s.proxies = append(s.proxies, proxy)
	}
}

func (s *simpleTestProxies) RemoveRandProxies(n int) {
	s.Lock()
	defer s.Unlock()
	if len(s.proxies) <= n {
		s.proxies = nil
		return
	}
	rms := make([]bool, len(s.proxies))
	rmc := 0
	for rmc < n {
		i := rand.N(len(s.proxies))
		if !rms[i] {
			rms[i] = true
			rmc++
		}
	}
	filtered := make([]testProxy, 0, len(s.proxies)-n)
	for i, p := range s.proxies {
		if !rms[i] {
			filtered = append(filtered, p)
		}
	}
	s.proxies = filtered
}

func (s *simpleTestProxies) GetRandProxy() (p testProxy, ok bool) {
	s.Lock()
	defer s.Unlock()
	if len(s.proxies) < 1 {
		ok = false
		return
	}
	i := rand.N(len(s.proxies))
	return s.proxies[i], true
}

func (s *simpleTestProxies) Discover(tracker *Tracker, lease *Lease) (ok bool) {
	proxy, ok := s.GetRandProxy()
	if !ok {
		panic("discovery called with no available proxies")
	}
	return s.ProxyLoop(tracker, lease, proxy)
}

func (s *simpleTestProxies) ProxyLoop(tracker *Tracker, lease *Lease, proxy testProxy) (ok bool) {
	defer lease.Release()
	timeout := time.After(proxy.life)

	if !lease.Claim(proxy.principals...) {
		return false
	}

	ticker := time.NewTicker(jitter(time.Millisecond * 100))
	defer ticker.Stop()

Loop:
	for {
		select {
		case <-ticker.C:
			if p, ok := s.GetRandProxy(); ok {
				tracker.TrackExpected(Proxy{Name: p.principals[0]})
			}
		case <-timeout:
			break Loop
		}
	}

	return ok
}

type testProxy struct {
	principals []string
	life       time.Duration
}

func newTestProxy(life time.Duration) testProxy {
	principals := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		p := fmt.Sprintf("proxy-%d", rand.Int())
		principals = append(principals, p)
	}
	return testProxy{principals, life}
}

func prDuration(min time.Duration, max time.Duration) time.Duration {
	return min + rand.N(max-min)
}

func jitter(t time.Duration) time.Duration {
	return t + rand.N(t/5)
}

func TestBasic(t *testing.T) {
	const (
		timeout    = time.Second * 16
		proxyCount = 16
	)

	timeoutC := time.After(timeout)
	ticker := time.NewTicker(time.Millisecond * 10)
	t.Cleanup(ticker.Stop)
	tracker, err := New(Config{ClusterName: "test-cluster"})
	require.NoError(t, err)
	min, max := time.Duration(0), timeout
	var proxies simpleTestProxies
	proxies.AddRandProxies(proxyCount, min, max)

	for {
		if lease := tracker.TryAcquire(); lease != nil {
			t.Logf("acquired lease %v", lease.ID())
			go proxies.Discover(tracker, lease)
			continue
		}

		if tracker.activeCount() == proxyCount {
			t.Logf("activeCount: %v", tracker.activeCount())
			break
		}

		select {
		case <-ticker.C:
			t.Logf("activeCount: %v", tracker.activeCount())
		case <-timeoutC:
			t.Fatal("timeout")
		}
	}
}

func TestFullRotation(t *testing.T) {
	const (
		proxyCount = 8
		minConnA   = time.Second * 2
		maxConnA   = time.Second * 3
		minConnB   = time.Second * 24
		maxConnB   = time.Second * 25
		timeout    = time.Second * 30
	)

	ticker := time.NewTicker(time.Millisecond * 100)
	t.Cleanup(ticker.Stop)

	var proxies simpleTestProxies
	proxies.AddRandProxies(proxyCount, minConnA, maxConnA)
	tracker, err := New(Config{ClusterName: "test-cluster"})
	require.NoError(t, err)

	timeoutC := time.After(timeout)
	for {
		if lease := tracker.TryAcquire(); lease != nil {
			// get our "discovered" proxy in the foreground
			// to prevent race with the call to RemoveRandProxies
			// that comes after this loop.
			proxy, ok := proxies.GetRandProxy()
			if !ok {
				t.Fatal("failed to get test proxy")
			}
			go proxies.ProxyLoop(tracker, lease, proxy)
			continue
		}

		if tracker.activeCount() == proxyCount {
			t.Logf("activeCount0: %v", tracker.activeCount())
			break
		}

		select {
		case <-ticker.C:
			t.Logf("activeCount0: %v", tracker.activeCount())
		case <-timeoutC:
			t.Fatal("timeout")
		}
	}
	proxies.RemoveRandProxies(proxyCount)

	for {
		if tracker.activeCount() < 1 {
			t.Logf("activeCount1: %v", tracker.activeCount())
			break
		}

		select {
		case <-ticker.C:
			t.Logf("activeCount1: %v", tracker.activeCount())
		case <-timeoutC:
			t.Fatal("timeout")
		}
	}
	proxies.AddRandProxies(proxyCount, minConnB, maxConnB)

	for {
		if lease := tracker.TryAcquire(); lease != nil {
			go proxies.Discover(tracker, lease)
			continue
		}

		if tracker.activeCount() >= proxyCount {
			t.Logf("activeCount2: %v", tracker.activeCount())
			break
		}

		select {
		case <-ticker.C:
			t.Logf("activeCount2: %v", tracker.activeCount())
		case <-timeoutC:
			t.Fatal("timeout")
		}
	}
}

// TestUUIDHandling verifies that host UUIDs are correctly extracted
// from the expected teleport principal format, and that gossip messages
// consisting only of uuid don't create duplicate entries.
func TestUUIDHandling(t *testing.T) {
	tracker, err := New(Config{ClusterName: "test-cluster"})
	require.NoError(t, err)

	lease := tracker.TryAcquire()
	require.NotNil(t, lease)
	require.True(t, lease.Claim("my-proxy.test-cluster"))
	require.Equal(t, "my-proxy", lease.claimName)

	tracker.TrackExpected(Proxy{Name: "my-proxy"})

	require.Nil(t, tracker.TryAcquire())
}

func TestIsClaimed(t *testing.T) {
	tracker, err := New(Config{ClusterName: "test-cluster"})
	require.NoError(t, err)

	tracker.TrackExpected(Proxy{Name: "proxy1"}, Proxy{Name: "proxy2"})
	require.False(t, tracker.IsClaimed("proxy1.test-cluster"))

	lease := tracker.TryAcquire()
	require.NotNil(t, lease)

	ok := lease.Claim("proxy1.test-cluster")
	require.True(t, ok)

	require.True(t, tracker.IsClaimed("proxy1"))
	require.True(t, tracker.IsClaimed("proxy1.test-cluster"))
	require.False(t, tracker.IsClaimed("proxy2"))

	lease.Release()

	require.False(t, tracker.IsClaimed("proxy1"))
	require.False(t, tracker.IsClaimed("proxy2"))
}

func (t *Tracker) activeCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.claimed) + t.inflight
}

func TestProxyGroups(t *testing.T) {
	tracker, err := New(Config{ClusterName: "test-cluster"})
	require.NoError(t, err)

	tracker.SetConnectionCount(2)

	tracker.TrackExpected(
		Proxy{Name: "xa", Group: "x", Generation: 1},
		Proxy{Name: "xb", Group: "x", Generation: 1},
		Proxy{Name: "yc", Group: "y", Generation: 1},
		Proxy{Name: "yd", Group: "y", Generation: 1},
	)

	requireAcquire := func() *Lease {
		lease := tracker.TryAcquire()
		require.NotNil(t, lease)
		return lease
	}

	requireNoAcquire := func() {
		require.Nil(t, tracker.TryAcquire())
	}

	xa := requireAcquire()
	xb := requireAcquire()

	require.True(t, xa.Claim("xa"))
	require.True(t, xb.Claim("xb"))

	requireNoAcquire()

	tracker.TrackExpected(
		Proxy{Name: "xe", Group: "x", Generation: 2},
		Proxy{Name: "xf", Group: "x", Generation: 2},
	)

	yc := requireAcquire()
	yd := requireAcquire()

	require.True(t, yc.Claim("yc"))
	require.True(t, yd.Claim("yd"))

	requireNoAcquire()

	tracker.SetConnectionCount(0)

	xe := requireAcquire()
	xf := requireAcquire()

	requireNoAcquire()

	require.True(t, xe.Claim("xe"))
	require.True(t, xf.Claim("xf"))

	// releasing a proxy from a previous generation doesn't let a new connection
	// spawn
	xa.Release()
	requireNoAcquire()

	// whereas releasing a proxy from a current generation does
	yc.Release()
	requireAcquire()
}
